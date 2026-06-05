package runtimeinventory

import "errors"

var ErrInvalidModuleVersionReference = errors.New("invalid module version reference")
var ErrRuntimeModulesRequired = errors.New("runtime report must include at least one module")
