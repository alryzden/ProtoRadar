package bufcli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

const (
	LintModeDisabled = "disabled"
	LintModeWarn     = "warn"
	LintModeEnforce  = "enforce"
)

type Config struct {
	BinaryPath     string
	BuildTimeout   time.Duration
	LintTimeout    time.Duration
	LintMode       string
	RequireConfig  bool
	MaxReportBytes int
}

type Workflow struct {
	cfg    Config
	runner commandRunner
}

func NewWorkflow(cfg Config) (*Workflow, error) {
	return newWorkflow(cfg, execCommandRunner{}), nil
}

func newWorkflow(cfg Config, runner commandRunner) *Workflow {
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = "buf"
	}
	if cfg.BuildTimeout <= 0 {
		cfg.BuildTimeout = 30 * time.Second
	}
	if cfg.LintTimeout <= 0 {
		cfg.LintTimeout = 30 * time.Second
	}
	if cfg.LintMode == "" {
		cfg.LintMode = LintModeWarn
	}
	if cfg.MaxReportBytes <= 0 {
		cfg.MaxReportBytes = 16 * 1024
	}
	return &Workflow{cfg: cfg, runner: runner}
}

func (workflow *Workflow) Inspect(ctx context.Context, workdir string, options registry.BufWorkflowOptions) (registry.BufWorkflowResult, error) {
	configInfo, err := inspectConfig(workdir)
	if err != nil {
		return registry.BufWorkflowResult{}, err
	}
	if (workflow.cfg.RequireConfig || options.RequireBufYAML) && !configInfo.BufYAMLPresent {
		return registry.BufWorkflowResult{}, fmt.Errorf("buf.yaml is required")
	}

	buildResult, err := workflow.runBuild(ctx, workdir)
	if err != nil {
		return registry.BufWorkflowResult{ConfigInfo: configInfo}, err
	}
	image := buildResult.stdout
	sum := sha256.Sum256(image)
	metadata, err := ExtractDescriptorMetadata(image)
	if err != nil {
		return registry.BufWorkflowResult{ConfigInfo: configInfo, BufImage: image, BufImageDigest: "sha256:" + hex.EncodeToString(sum[:])}, err
	}

	lintResult, err := workflow.runLint(ctx, workdir, options.RunLint)
	if err != nil {
		return registry.BufWorkflowResult{
			ConfigInfo:         configInfo,
			BufImage:           image,
			BufImageDigest:     "sha256:" + hex.EncodeToString(sum[:]),
			LintResult:         lintResult,
			DescriptorMetadata: metadata,
		}, err
	}
	configInfo.LintEnabled = lintResult.Status != domain.BufLintStatusNotRun

	return registry.BufWorkflowResult{
		ConfigInfo:         configInfo,
		BufImage:           image,
		BufImageDigest:     "sha256:" + hex.EncodeToString(sum[:]),
		LintResult:         lintResult,
		DescriptorMetadata: metadata,
	}, nil
}

func (workflow *Workflow) runBuild(ctx context.Context, workdir string) (commandResult, error) {
	// Buf can emit a FileDescriptorSet directly; this keeps the stored image parseable
	// without depending on Buf-specific image protobufs in the application model.
	result, err := workflow.runCommand(ctx, commandSpec{
		path:        workflow.cfg.BinaryPath,
		args:        []string{"build", "--as-file-descriptor-set", "-o", "-"},
		dir:         workdir,
		timeout:     workflow.cfg.BuildTimeout,
		stderrLimit: workflow.cfg.MaxReportBytes,
	})
	if err != nil {
		return commandResult{}, err
	}
	if result.exitCode != 0 {
		return commandResult{}, fmt.Errorf("buf build failed: %s", truncateReport(result.report(), workflow.cfg.MaxReportBytes))
	}
	if len(result.stdout) == 0 {
		return commandResult{}, fmt.Errorf("buf build produced empty image")
	}
	return result, nil
}

func (workflow *Workflow) runLint(ctx context.Context, workdir string, requested bool) (domain.BufLintResult, error) {
	if !requested || workflow.cfg.LintMode == LintModeDisabled {
		return domain.BufLintResult{Status: domain.BufLintStatusNotRun}, nil
	}

	result, err := workflow.runCommand(ctx, commandSpec{
		path:        workflow.cfg.BinaryPath,
		args:        []string{"lint"},
		dir:         workdir,
		timeout:     workflow.cfg.LintTimeout,
		stdoutLimit: workflow.cfg.MaxReportBytes,
		stderrLimit: workflow.cfg.MaxReportBytes,
	})
	if err != nil {
		return domain.BufLintResult{}, err
	}
	report := truncateReport(result.report(), workflow.cfg.MaxReportBytes)
	if result.exitCode == 0 {
		return domain.BufLintResult{Status: domain.BufLintStatusPassed, Report: report}, nil
	}
	if workflow.cfg.LintMode == LintModeEnforce {
		lintResult := domain.BufLintResult{Status: domain.BufLintStatusFailed, Report: report}
		return lintResult, fmt.Errorf("buf lint failed: %s", report)
	}
	return domain.BufLintResult{Status: domain.BufLintStatusWarning, Report: report}, nil
}

func (workflow *Workflow) runCommand(ctx context.Context, spec commandSpec) (commandResult, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, spec.timeout)
	defer cancel()
	result, err := workflow.runner.Run(cmdCtx, spec)
	if err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) || errors.Is(cmdCtx.Err(), context.Canceled) {
			return commandResult{}, cmdCtx.Err()
		}
		return commandResult{}, err
	}
	if err := cmdCtx.Err(); err != nil {
		return commandResult{}, err
	}
	return result, nil
}

func inspectConfig(workdir string) (domain.BufConfigInfo, error) {
	bufYAMLPath := filepath.Join(workdir, "buf.yaml")
	bufLockPath := filepath.Join(workdir, "buf.lock")
	bufYAMLPresent, bufYAMLDigest, err := filePresenceAndDigest(bufYAMLPath)
	if err != nil {
		return domain.BufConfigInfo{}, err
	}
	bufLockPresent, bufLockDigest, err := filePresenceAndDigest(bufLockPath)
	if err != nil {
		return domain.BufConfigInfo{}, err
	}
	return domain.BufConfigInfo{
		BufYAMLPresent: bufYAMLPresent,
		BufLockPresent: bufLockPresent,
		BufYAMLDigest:  bufYAMLDigest,
		BufLockDigest:  bufLockDigest,
	}, nil
}

func filePresenceAndDigest(path string) (bool, string, error) {
	body, err := os.ReadFile(path)
	if err == nil {
		sum := sha256.Sum256(body)
		return true, "sha256:" + hex.EncodeToString(sum[:]), nil
	}
	if os.IsNotExist(err) {
		return false, "", nil
	}
	return false, "", err
}

func truncateReport(report string, limit int) string {
	if limit <= 0 || len(report) <= limit {
		return report
	}
	return report[:limit]
}

type commandSpec struct {
	path        string
	args        []string
	dir         string
	timeout     time.Duration
	stdoutLimit int
	stderrLimit int
}

type commandResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func (result commandResult) report() string {
	var parts []string
	if text := strings.TrimSpace(string(result.stdout)); text != "" {
		parts = append(parts, text)
	}
	if text := strings.TrimSpace(string(result.stderr)); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

type commandRunner interface {
	Run(ctx context.Context, spec commandSpec) (commandResult, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, spec commandSpec) (commandResult, error) {
	cmd := exec.CommandContext(ctx, spec.path, spec.args...)
	cmd.Dir = spec.dir
	stdout := newCommandBuffer(spec.stdoutLimit)
	stderr := newCommandBuffer(spec.stderrLimit)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := commandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if cmd.ProcessState != nil {
		result.exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.exitCode = exitErr.ExitCode()
			return result, nil
		}
		return commandResult{}, err
	}
	return result, nil
}

type commandBuffer struct {
	limit int
	body  bytes.Buffer
}

func newCommandBuffer(limit int) commandBuffer {
	return commandBuffer{limit: limit}
}

func (buffer *commandBuffer) Write(p []byte) (int, error) {
	if buffer.limit <= 0 {
		_, _ = buffer.body.Write(p)
		return len(p), nil
	}
	remaining := buffer.limit - buffer.body.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = buffer.body.Write(p[:remaining])
		} else {
			_, _ = buffer.body.Write(p)
		}
	}
	return len(p), nil
}

func (buffer *commandBuffer) Bytes() []byte {
	return buffer.body.Bytes()
}

var _ registry.BufWorkflow = (*Workflow)(nil)
