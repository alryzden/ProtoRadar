package governance

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type IDGenerator interface {
	NewModuleOwnerID() (domain.ModuleOwnerID, error)
	NewApprovalRequestID() (domain.ApprovalRequestID, error)
	NewApprovalRequirementID() (domain.ApprovalRequirementID, error)
	NewApprovalDecisionID() (domain.ApprovalDecisionID, error)
	NewGovernanceAuditEventID() (domain.GovernanceAuditEventID, error)
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) NewModuleOwnerID() (domain.ModuleOwnerID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewModuleOwnerID(value), nil
}

func (RandomIDGenerator) NewApprovalRequestID() (domain.ApprovalRequestID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewApprovalRequestID(value), nil
}

func (RandomIDGenerator) NewApprovalRequirementID() (domain.ApprovalRequirementID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewApprovalRequirementID(value), nil
}

func (RandomIDGenerator) NewApprovalDecisionID() (domain.ApprovalDecisionID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewApprovalDecisionID(value), nil
}

func (RandomIDGenerator) NewGovernanceAuditEventID() (domain.GovernanceAuditEventID, error) {
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return domain.NewGovernanceAuditEventID(value), nil
}

func randomHex(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
