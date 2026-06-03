package domain

import "strings"

type ModuleName string

func NewModuleName(value string) (ModuleName, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", ErrInvalidModuleName
	}

	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '-' || ch == '_' {
			continue
		}
		return "", ErrInvalidModuleName
	}

	return ModuleName(name), nil
}

func (name ModuleName) String() string {
	return string(name)
}
