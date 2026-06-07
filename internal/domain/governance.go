package domain

import (
	"encoding/json"
	"strings"
	"time"
)

const MaxApprovalDecisionCommentLength = 2000

type GovernanceSubjectType string

const (
	GovernanceSubjectTypeUser GovernanceSubjectType = "user"
	GovernanceSubjectTypeTeam GovernanceSubjectType = "team"

	OwnerSubjectTypeUser = GovernanceSubjectTypeUser
	OwnerSubjectTypeTeam = GovernanceSubjectTypeTeam
)

func GovernanceSubjectTypes() []GovernanceSubjectType {
	return []GovernanceSubjectType{
		GovernanceSubjectTypeUser,
		GovernanceSubjectTypeTeam,
	}
}

func NewGovernanceSubjectType(value string) (GovernanceSubjectType, error) {
	subjectType := GovernanceSubjectType(strings.TrimSpace(value))
	if !subjectType.IsValid() {
		return "", ErrInvalidGovernanceSubjectType
	}
	return subjectType, nil
}

func (subjectType GovernanceSubjectType) String() string {
	return string(subjectType)
}

func (subjectType GovernanceSubjectType) IsValid() bool {
	for _, allowed := range GovernanceSubjectTypes() {
		if subjectType == allowed {
			return true
		}
	}
	return false
}

type ModuleOwnerRole string

const (
	ModuleOwnerRoleOwner      ModuleOwnerRole = "owner"
	ModuleOwnerRoleMaintainer ModuleOwnerRole = "maintainer"
)

func ModuleOwnerRoles() []ModuleOwnerRole {
	return []ModuleOwnerRole{
		ModuleOwnerRoleOwner,
		ModuleOwnerRoleMaintainer,
	}
}

func NewModuleOwnerRole(value string) (ModuleOwnerRole, error) {
	role := ModuleOwnerRole(strings.TrimSpace(value))
	if !role.IsValid() {
		return "", ErrInvalidModuleOwnerRole
	}
	return role, nil
}

func (role ModuleOwnerRole) String() string {
	return string(role)
}

func (role ModuleOwnerRole) IsValid() bool {
	for _, allowed := range ModuleOwnerRoles() {
		if role == allowed {
			return true
		}
	}
	return false
}

type ModuleOwner struct {
	ID          ModuleOwnerID
	ModuleID    ModuleID
	ModuleName  ModuleName
	SubjectType GovernanceSubjectType
	Subject     string
	Role        ModuleOwnerRole
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (owner ModuleOwner) Validate() error {
	if !owner.SubjectType.IsValid() {
		return ErrInvalidGovernanceSubjectType
	}
	if strings.TrimSpace(owner.Subject) == "" {
		return ErrInvalidGovernanceSubject
	}
	if !owner.Role.IsValid() {
		return ErrInvalidModuleOwnerRole
	}
	return nil
}

type ApprovalRequestStatus string

const (
	ApprovalRequestStatusPending     ApprovalRequestStatus = "pending"
	ApprovalRequestStatusApproved    ApprovalRequestStatus = "approved"
	ApprovalRequestStatusRejected    ApprovalRequestStatus = "rejected"
	ApprovalRequestStatusCancelled   ApprovalRequestStatus = "cancelled"
	ApprovalRequestStatusNotRequired ApprovalRequestStatus = "not_required"
)

func ApprovalRequestStatuses() []ApprovalRequestStatus {
	return []ApprovalRequestStatus{
		ApprovalRequestStatusPending,
		ApprovalRequestStatusApproved,
		ApprovalRequestStatusRejected,
		ApprovalRequestStatusCancelled,
		ApprovalRequestStatusNotRequired,
	}
}

func NewApprovalRequestStatus(value string) (ApprovalRequestStatus, error) {
	status := ApprovalRequestStatus(strings.TrimSpace(value))
	if !status.IsValid() {
		return "", ErrInvalidApprovalRequestStatus
	}
	return status, nil
}

func (status ApprovalRequestStatus) String() string {
	return string(status)
}

func (status ApprovalRequestStatus) IsValid() bool {
	for _, allowed := range ApprovalRequestStatuses() {
		if status == allowed {
			return true
		}
	}
	return false
}

type ApprovalRequirementType string

const (
	ApprovalRequirementTypeModuleOwnerApproval      ApprovalRequirementType = "module_owner_approval"
	ApprovalRequirementTypeAffectedConsumerApproval ApprovalRequirementType = "affected_consumer_approval"
)

func ApprovalRequirementTypes() []ApprovalRequirementType {
	return []ApprovalRequirementType{
		ApprovalRequirementTypeModuleOwnerApproval,
		ApprovalRequirementTypeAffectedConsumerApproval,
	}
}

func NewApprovalRequirementType(value string) (ApprovalRequirementType, error) {
	requirementType := ApprovalRequirementType(strings.TrimSpace(value))
	if !requirementType.IsValid() {
		return "", ErrInvalidApprovalRequirementType
	}
	return requirementType, nil
}

func (requirementType ApprovalRequirementType) String() string {
	return string(requirementType)
}

func (requirementType ApprovalRequirementType) IsValid() bool {
	for _, allowed := range ApprovalRequirementTypes() {
		if requirementType == allowed {
			return true
		}
	}
	return false
}

type ApprovalRequirementStatus string

const (
	ApprovalRequirementStatusPending     ApprovalRequirementStatus = "pending"
	ApprovalRequirementStatusApproved    ApprovalRequirementStatus = "approved"
	ApprovalRequirementStatusRejected    ApprovalRequirementStatus = "rejected"
	ApprovalRequirementStatusNotRequired ApprovalRequirementStatus = "not_required"
)

func ApprovalRequirementStatuses() []ApprovalRequirementStatus {
	return []ApprovalRequirementStatus{
		ApprovalRequirementStatusPending,
		ApprovalRequirementStatusApproved,
		ApprovalRequirementStatusRejected,
		ApprovalRequirementStatusNotRequired,
	}
}

func NewApprovalRequirementStatus(value string) (ApprovalRequirementStatus, error) {
	status := ApprovalRequirementStatus(strings.TrimSpace(value))
	if !status.IsValid() {
		return "", ErrInvalidApprovalRequirementStatus
	}
	return status, nil
}

func (status ApprovalRequirementStatus) String() string {
	return string(status)
}

func (status ApprovalRequirementStatus) IsValid() bool {
	for _, allowed := range ApprovalRequirementStatuses() {
		if status == allowed {
			return true
		}
	}
	return false
}

type ApprovalDecisionValue string

const (
	ApprovalDecisionValueApproved ApprovalDecisionValue = "approved"
	ApprovalDecisionValueRejected ApprovalDecisionValue = "rejected"

	ApprovalDecisionApproved = ApprovalDecisionValueApproved
	ApprovalDecisionRejected = ApprovalDecisionValueRejected
)

func ApprovalDecisionValues() []ApprovalDecisionValue {
	return []ApprovalDecisionValue{
		ApprovalDecisionValueApproved,
		ApprovalDecisionValueRejected,
	}
}

func NewApprovalDecisionValue(value string) (ApprovalDecisionValue, error) {
	decision := ApprovalDecisionValue(strings.TrimSpace(value))
	if !decision.IsValid() {
		return "", ErrInvalidApprovalDecision
	}
	return decision, nil
}

func (decision ApprovalDecisionValue) String() string {
	return string(decision)
}

func (decision ApprovalDecisionValue) IsValid() bool {
	for _, allowed := range ApprovalDecisionValues() {
		if decision == allowed {
			return true
		}
	}
	return false
}

type ApprovalRequest struct {
	ID                ApprovalRequestID
	ModuleID          ModuleID
	ModuleName        ModuleName
	BreakingReportID  *BreakingReportID
	TargetRef         string
	Status            ApprovalRequestStatus
	RequiredApprovals int
	ReceivedApprovals int
	Requirements      []ApprovalRequirement
	Decisions         []ApprovalDecision
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ApprovalRequirement struct {
	ID                ApprovalRequirementID
	ApprovalRequestID ApprovalRequestID
	RequirementType   ApprovalRequirementType
	TargetModuleID    *ModuleID
	TargetModuleName  ModuleName
	RequiredRole      ModuleOwnerRole
	Status            ApprovalRequirementStatus
	Reason            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ApprovalDecision struct {
	ID                ApprovalDecisionID
	ApprovalRequestID ApprovalRequestID
	RequirementID     ApprovalRequirementID
	Decision          ApprovalDecisionValue
	DecidedBy         string
	Comment           string
	CreatedAt         time.Time
}

func (decision ApprovalDecision) Validate() error {
	if !decision.Decision.IsValid() {
		return ErrInvalidApprovalDecision
	}
	if strings.TrimSpace(decision.DecidedBy) == "" {
		return ErrInvalidApprovalActor
	}
	if len(decision.Comment) > MaxApprovalDecisionCommentLength {
		return ErrApprovalDecisionCommentTooLong
	}
	return nil
}

type GovernanceAuditEventType string

const (
	GovernanceAuditEventTypeModuleOwnerAdded             GovernanceAuditEventType = "module_owner_added"
	GovernanceAuditEventTypeModuleOwnerRemoved           GovernanceAuditEventType = "module_owner_removed"
	GovernanceAuditEventTypeApprovalRequestCreated       GovernanceAuditEventType = "approval_request_created"
	GovernanceAuditEventTypeApprovalDecisionRecorded     GovernanceAuditEventType = "approval_decision_recorded"
	GovernanceAuditEventTypeApprovalRequestStatusChanged GovernanceAuditEventType = "approval_request_status_changed"
	GovernanceAuditEventTypePolicyEvaluated              GovernanceAuditEventType = "policy_evaluated"
)

func GovernanceAuditEventTypes() []GovernanceAuditEventType {
	return []GovernanceAuditEventType{
		GovernanceAuditEventTypeModuleOwnerAdded,
		GovernanceAuditEventTypeModuleOwnerRemoved,
		GovernanceAuditEventTypeApprovalRequestCreated,
		GovernanceAuditEventTypeApprovalDecisionRecorded,
		GovernanceAuditEventTypeApprovalRequestStatusChanged,
		GovernanceAuditEventTypePolicyEvaluated,
	}
}

func NewGovernanceAuditEventType(value string) (GovernanceAuditEventType, error) {
	eventType := GovernanceAuditEventType(strings.TrimSpace(value))
	if !eventType.IsValid() {
		return "", ErrInvalidGovernanceAuditEventType
	}
	return eventType, nil
}

func (eventType GovernanceAuditEventType) String() string {
	return string(eventType)
}

func (eventType GovernanceAuditEventType) IsValid() bool {
	for _, allowed := range GovernanceAuditEventTypes() {
		if eventType == allowed {
			return true
		}
	}
	return false
}

type GovernanceAuditEvent struct {
	ID                GovernanceAuditEventID
	EventType         GovernanceAuditEventType
	Actor             string
	ModuleID          *ModuleID
	ModuleName        ModuleName
	ApprovalRequestID *ApprovalRequestID
	BreakingReportID  *BreakingReportID
	PayloadJSON       json.RawMessage
	CreatedAt         time.Time
}
