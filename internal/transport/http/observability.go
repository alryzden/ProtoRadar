package httptransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-ID"
const prometheusContentType = "text/plain; version=0.0.4; charset=utf-8"

type requestIDContextKey struct{}

func NewLogger(level string, format string, out io.Writer) (*slog.Logger, error) {
	if out == nil {
		out = io.Discard
	}
	var slogLevel slog.Level
	switch strings.TrimSpace(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "", "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("log.level must be one of debug, info, warn, error")
	}
	opts := &slog.HandlerOptions{Level: slogLevel}
	switch strings.TrimSpace(format) {
	case "", "json":
		return slog.New(slog.NewJSONHandler(out, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(out, opts)), nil
	default:
		return nil, fmt.Errorf("log.format must be one of json, text")
	}
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func (server *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID)))
	})
}

func (server *Server) withRequestLogging(next http.Handler) http.Handler {
	if server.logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		server.logger.InfoContext(r.Context(), "http_request",
			slog.String("method", r.Method),
			slog.String("route", routeTemplate(r)),
			slog.Int("status", recorder.status),
			slog.Float64("duration_ms", float64(time.Since(started).Microseconds())/1000),
			slog.String("request_id", requestIDFromContext(r.Context())),
			slog.String("remote_addr", r.RemoteAddr),
		)
	})
}

func (server *Server) withHTTPMetrics(next http.Handler) http.Handler {
	if server.metrics == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.metrics.IncInFlight()
		defer server.metrics.DecInFlight()
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		server.metrics.RecordHTTPRequest(r.Method, routeTemplate(r), strconv.Itoa(recorder.status), time.Since(started).Seconds())
	})
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func routeTemplate(r *http.Request) string {
	pattern := strings.TrimSpace(r.Pattern)
	if pattern == "" {
		return "unknown"
	}
	if method, route, ok := strings.Cut(pattern, " "); ok && method == r.Method {
		return route
	}
	return pattern
}

type Metrics struct {
	inFlight atomic.Int64

	mu                        sync.Mutex
	httpRequests              map[labelKey]uint64
	httpDuration              map[labelKey]durationMetric
	publishTotal              map[string]uint64
	publishDuration           durationMetric
	breakingChecksTotal       map[string]uint64
	breakingCheckDuration     durationMetric
	runtimeReportsTotal       map[string]uint64
	dependencyEdgesTotal      int64
	unresolvedDependencyTotal int64
}

type labelKey struct {
	Method string
	Route  string
	Status string
}

type durationMetric struct {
	Count uint64
	Sum   float64
}

func NewMetrics() *Metrics {
	return &Metrics{
		httpRequests:        map[labelKey]uint64{},
		httpDuration:        map[labelKey]durationMetric{},
		publishTotal:        map[string]uint64{},
		breakingChecksTotal: map[string]uint64{},
		runtimeReportsTotal: map[string]uint64{},
	}
}

func (metrics *Metrics) IncInFlight() {
	metrics.inFlight.Add(1)
}

func (metrics *Metrics) DecInFlight() {
	metrics.inFlight.Add(-1)
}

func (metrics *Metrics) RecordHTTPRequest(method string, route string, status string, seconds float64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	key := labelKey{Method: method, Route: route, Status: status}
	metrics.httpRequests[key]++
	duration := metrics.httpDuration[key]
	duration.Count++
	duration.Sum += seconds
	metrics.httpDuration[key] = duration
}

func (metrics *Metrics) RecordPublish(status string, seconds float64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.publishTotal[status]++
	metrics.publishDuration.Count++
	metrics.publishDuration.Sum += seconds
}

func (metrics *Metrics) RecordBreakingCheck(status string, seconds float64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.breakingChecksTotal[status]++
	metrics.breakingCheckDuration.Count++
	metrics.breakingCheckDuration.Sum += seconds
}

func (metrics *Metrics) RecordRuntimeReport(status string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.runtimeReportsTotal[status]++
}

func (metrics *Metrics) SetDependencyTotals(edges int, unresolved int) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.dependencyEdgesTotal = int64(edges)
	metrics.unresolvedDependencyTotal = int64(unresolved)
}

func (metrics *Metrics) WritePrometheus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", prometheusContentType)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	fmt.Fprintln(w, "# HELP protoradar_http_requests_total Total HTTP requests.")
	fmt.Fprintln(w, "# TYPE protoradar_http_requests_total counter")
	for _, key := range sortedLabelKeys(metrics.httpRequests) {
		fmt.Fprintf(w, "protoradar_http_requests_total{method=%q,route=%q,status=%q} %d\n", key.Method, key.Route, key.Status, metrics.httpRequests[key])
	}
	fmt.Fprintln(w, "# HELP protoradar_http_request_duration_seconds HTTP request duration in seconds.")
	fmt.Fprintln(w, "# TYPE protoradar_http_request_duration_seconds summary")
	for _, key := range sortedDurationKeys(metrics.httpDuration) {
		duration := metrics.httpDuration[key]
		fmt.Fprintf(w, "protoradar_http_request_duration_seconds_sum{method=%q,route=%q,status=%q} %g\n", key.Method, key.Route, key.Status, duration.Sum)
		fmt.Fprintf(w, "protoradar_http_request_duration_seconds_count{method=%q,route=%q,status=%q} %d\n", key.Method, key.Route, key.Status, duration.Count)
	}
	fmt.Fprintln(w, "# HELP protoradar_http_in_flight_requests Current in-flight HTTP requests.")
	fmt.Fprintln(w, "# TYPE protoradar_http_in_flight_requests gauge")
	fmt.Fprintf(w, "protoradar_http_in_flight_requests %d\n", metrics.inFlight.Load())
	writeStatusCounters(w, "protoradar_publish_total", "Total publish requests.", metrics.publishTotal)
	writeDuration(w, "protoradar_publish_duration_seconds", "Publish duration in seconds.", metrics.publishDuration)
	writeStatusCounters(w, "protoradar_breaking_checks_total", "Total breaking-check requests.", metrics.breakingChecksTotal)
	writeDuration(w, "protoradar_breaking_check_duration_seconds", "Breaking-check duration in seconds.", metrics.breakingCheckDuration)
	writeStatusCounters(w, "protoradar_runtime_reports_total", "Total runtime inventory report requests.", metrics.runtimeReportsTotal)
	fmt.Fprintln(w, "# HELP protoradar_dependency_edges_total Latest observed dependency edge count.")
	fmt.Fprintln(w, "# TYPE protoradar_dependency_edges_total gauge")
	fmt.Fprintf(w, "protoradar_dependency_edges_total %d\n", metrics.dependencyEdgesTotal)
	fmt.Fprintln(w, "# HELP protoradar_unresolved_dependencies_total Latest observed unresolved dependency count.")
	fmt.Fprintln(w, "# TYPE protoradar_unresolved_dependencies_total gauge")
	fmt.Fprintf(w, "protoradar_unresolved_dependencies_total %d\n", metrics.unresolvedDependencyTotal)
}

func sortedLabelKeys(values map[labelKey]uint64) []labelKey {
	keys := make([]labelKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		if keys[i].Route != keys[j].Route {
			return keys[i].Route < keys[j].Route
		}
		return keys[i].Status < keys[j].Status
	})
	return keys
}

func sortedDurationKeys(values map[labelKey]durationMetric) []labelKey {
	keys := make([]labelKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i int, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		if keys[i].Route != keys[j].Route {
			return keys[i].Route < keys[j].Route
		}
		return keys[i].Status < keys[j].Status
	})
	return keys
}

func writeStatusCounters(w io.Writer, name string, help string, values map[string]uint64) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s counter\n", name)
	statuses := make([]string, 0, len(values))
	for status := range values {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		fmt.Fprintf(w, "%s{status=%q} %d\n", name, status, values[status])
	}
}

func writeDuration(w io.Writer, name string, help string, value durationMetric) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s summary\n", name)
	fmt.Fprintf(w, "%s_sum %g\n", name, value.Sum)
	fmt.Fprintf(w, "%s_count %d\n", name, value.Count)
}

func (server *Server) recordPublishMetric(status string, started time.Time) {
	if server.metrics == nil {
		return
	}
	server.metrics.RecordPublish(status, time.Since(started).Seconds())
}

func (server *Server) recordBreakingCheckMetric(status string, started time.Time) {
	if server.metrics == nil {
		return
	}
	server.metrics.RecordBreakingCheck(status, time.Since(started).Seconds())
}

func (server *Server) recordRuntimeReportMetric(status string) {
	if server.metrics == nil {
		return
	}
	server.metrics.RecordRuntimeReport(status)
}
