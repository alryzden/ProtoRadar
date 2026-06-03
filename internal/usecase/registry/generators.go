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
