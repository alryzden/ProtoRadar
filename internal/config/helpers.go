package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseBool(path, value string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", path)
	}
	return parsed, nil
}

func parseInt(path, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", path)
	}
	return parsed, nil
}

func parseInt64(path, value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", path)
	}
	return parsed, nil
}

func normalizePath(value string) string {
	path := strings.TrimRight(strings.TrimSpace(value), "/")
	if path == "" {
		return "/"
	}
	return path
}

func parseStringList(path, value string) ([]string, error) {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%s must contain at least one value", path)
	}
	return values, nil
}

func parseDuration(path, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration", path)
	}
	return parsed, nil
}

func parsePositiveDuration(path string, value time.Duration) (time.Duration, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", path)
	}
	return value, nil
}

// ParsePositiveDuration parses a raw config duration and returns a deterministic config-path error.
func ParsePositiveDuration(path, value string) (time.Duration, error) {
	parsed, err := parseDuration(path, value)
	if err != nil {
		return 0, err
	}
	return parsePositiveDuration(path, parsed)
}
