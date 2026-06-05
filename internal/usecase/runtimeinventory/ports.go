package runtimeinventory

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewRuntimeServiceID() (domain.RuntimeServiceID, error)
	NewRuntimeDeploymentID() (domain.RuntimeDeploymentID, error)
	NewRuntimeModuleUsageID() (domain.RuntimeModuleUsageID, error)
}
