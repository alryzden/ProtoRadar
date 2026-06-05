package registry

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) NewModuleID() (domain.ModuleID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewModuleID(value), nil
}

func (RandomIDGenerator) NewModuleGitLabProjectID() (domain.ModuleGitLabProjectID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewModuleGitLabProjectID(value), nil
}

func (RandomIDGenerator) NewModuleVersionID() (domain.ModuleVersionID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewModuleVersionID(value), nil
}

func (RandomIDGenerator) NewArtifactID() (domain.ArtifactID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewArtifactID(value), nil
}

func (RandomIDGenerator) NewAPITokenID() (domain.APITokenID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewAPITokenID(value), nil
}

func (RandomIDGenerator) NewBreakingReportID() (domain.BreakingReportID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewBreakingReportID(value), nil
}

func (RandomIDGenerator) NewBreakingChangeID() (domain.BreakingChangeID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewBreakingChangeID(value), nil
}

func (RandomIDGenerator) NewModuleDependencyID() (domain.ModuleDependencyID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewModuleDependencyID(value), nil
}

func (RandomIDGenerator) NewUnresolvedProtoDependencyID() (domain.UnresolvedProtoDependencyID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewUnresolvedProtoDependencyID(value), nil
}

func (RandomIDGenerator) NewRuntimeServiceID() (domain.RuntimeServiceID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewRuntimeServiceID(value), nil
}

func (RandomIDGenerator) NewRuntimeDeploymentID() (domain.RuntimeDeploymentID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewRuntimeDeploymentID(value), nil
}

func (RandomIDGenerator) NewRuntimeModuleUsageID() (domain.RuntimeModuleUsageID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewRuntimeModuleUsageID(value), nil
}

type RandomTokenGenerator struct{}

func (RandomTokenGenerator) NewToken() (string, error) {
	value, err := randomHex(32)
	if err != nil {
		return "", err
	}
	return value, nil
}

func randomHex(sizeBytes int) (string, error) {
	value := make([]byte, sizeBytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
