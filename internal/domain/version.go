package domain

import "strings"

type Version string

func NewVersion(value string) (Version, error) {
	version := strings.TrimSpace(value)
	if version == "" {
		return "", ErrInvalidVersion
	}
	return Version(version), nil
}

func (version Version) String() string {
	return string(version)
}
