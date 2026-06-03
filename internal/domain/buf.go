package domain

type BufConfigInfo struct {
	BufYAMLPresent        bool
	BufLockPresent        bool
	BufYAMLDigest         string
	BufLockDigest         string
	ModulePaths           []string
	Deps                  []string
	LintEnabled           bool
	BreakingConfigPresent bool
}

type BufLintStatus string

const (
	BufLintStatusNotRun  BufLintStatus = "not_run"
	BufLintStatusPassed  BufLintStatus = "passed"
	BufLintStatusWarning BufLintStatus = "warning"
	BufLintStatusFailed  BufLintStatus = "failed"
)

func (status BufLintStatus) String() string {
	return string(status)
}

type BufLintResult struct {
	Status BufLintStatus
	Report string
}
