package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestGovernanceAllowedValuesMatchDatabaseConstraints(t *testing.T) {
	assertAllowedValues(t, "subject types", governanceSubjectTypeStrings(), []string{"user", "team"})
	assertAllowedValues(t, "owner roles", moduleOwnerRoleStrings(), []string{"owner", "maintainer"})
	assertAllowedValues(t, "approval request statuses", approvalRequestStatusStrings(), []string{"pending", "approved", "rejected", "cancelled", "not_required"})
	assertAllowedValues(t, "approval requirement types", approvalRequirementTypeStrings(), []string{"module_owner_approval", "affected_consumer_approval"})
	assertAllowedValues(t, "approval requirement statuses", approvalRequirementStatusStrings(), []string{"pending", "approved", "rejected", "not_required"})
	assertAllowedValues(t, "approval decisions", approvalDecisionValueStrings(), []string{"approved", "rejected"})
	assertAllowedValues(t, "governance audit event types", governanceAuditEventTypeStrings(), []string{
		"module_owner_added",
		"module_owner_removed",
		"approval_request_created",
		"approval_decision_recorded",
		"approval_request_status_changed",
		"policy_evaluated",
	})

	if OwnerSubjectTypeUser != GovernanceSubjectTypeUser || OwnerSubjectTypeTeam != GovernanceSubjectTypeTeam {
		t.Fatalf("owner subject type aliases are not aligned")
	}
	if ApprovalDecisionApproved != ApprovalDecisionValueApproved || ApprovalDecisionRejected != ApprovalDecisionValueRejected {
		t.Fatalf("approval decision aliases are not aligned")
	}
}

func TestNewGovernanceSubjectType(t *testing.T) {
	for _, value := range []string{"user", "team", " user "} {
		subjectType, err := NewGovernanceSubjectType(value)
		if err != nil {
			t.Fatalf("subject type %q: %v", value, err)
		}
		if !subjectType.IsValid() {
			t.Fatalf("subject type %q should be valid", value)
		}
	}
	for _, value := range []string{"", "service", "owner"} {
		if _, err := NewGovernanceSubjectType(value); !errors.Is(err, ErrInvalidGovernanceSubjectType) {
			t.Fatalf("subject type %q error = %v, want %v", value, err, ErrInvalidGovernanceSubjectType)
		}
	}
}

func TestNewModuleOwnerRole(t *testing.T) {
	for _, value := range []string{"owner", "maintainer", " owner "} {
		role, err := NewModuleOwnerRole(value)
		if err != nil {
			t.Fatalf("role %q: %v", value, err)
		}
		if !role.IsValid() {
			t.Fatalf("role %q should be valid", value)
		}
	}
	for _, value := range []string{"", "admin", "reviewer"} {
		if _, err := NewModuleOwnerRole(value); !errors.Is(err, ErrInvalidModuleOwnerRole) {
			t.Fatalf("role %q error = %v, want %v", value, err, ErrInvalidModuleOwnerRole)
		}
	}
}

func TestModuleOwnerValidate(t *testing.T) {
	owner := ModuleOwner{SubjectType: GovernanceSubjectTypeUser, Subject: "alice", Role: ModuleOwnerRoleOwner}
	if err := owner.Validate(); err != nil {
		t.Fatalf("valid owner: %v", err)
	}

	owner.SubjectType = "service"
	if err := owner.Validate(); !errors.Is(err, ErrInvalidGovernanceSubjectType) {
		t.Fatalf("invalid subject type error = %v", err)
	}
	owner.SubjectType = GovernanceSubjectTypeUser
	owner.Subject = "  "
	if err := owner.Validate(); !errors.Is(err, ErrInvalidGovernanceSubject) {
		t.Fatalf("invalid subject error = %v", err)
	}
	owner.Subject = "alice"
	owner.Role = "admin"
	if err := owner.Validate(); !errors.Is(err, ErrInvalidModuleOwnerRole) {
		t.Fatalf("invalid role error = %v", err)
	}
}

func TestNewApprovalRequestStatus(t *testing.T) {
	for _, value := range []string{"pending", "approved", "rejected", "cancelled", "not_required", " pending "} {
		status, err := NewApprovalRequestStatus(value)
		if err != nil {
			t.Fatalf("request status %q: %v", value, err)
		}
		if !status.IsValid() {
			t.Fatalf("request status %q should be valid", value)
		}
	}
	for _, value := range []string{"", "done", "not-required"} {
		if _, err := NewApprovalRequestStatus(value); !errors.Is(err, ErrInvalidApprovalRequestStatus) {
			t.Fatalf("request status %q error = %v", value, err)
		}
	}
}

func TestNewApprovalRequirementType(t *testing.T) {
	for _, value := range []string{"module_owner_approval", "affected_consumer_approval", " module_owner_approval "} {
		requirementType, err := NewApprovalRequirementType(value)
		if err != nil {
			t.Fatalf("requirement type %q: %v", value, err)
		}
		if !requirementType.IsValid() {
			t.Fatalf("requirement type %q should be valid", value)
		}
	}
	for _, value := range []string{"", "missing_module_owner", "approval"} {
		if _, err := NewApprovalRequirementType(value); !errors.Is(err, ErrInvalidApprovalRequirementType) {
			t.Fatalf("requirement type %q error = %v", value, err)
		}
	}
}

func TestNewApprovalRequirementStatus(t *testing.T) {
	for _, value := range []string{"pending", "approved", "rejected", "not_required", " pending "} {
		status, err := NewApprovalRequirementStatus(value)
		if err != nil {
			t.Fatalf("requirement status %q: %v", value, err)
		}
		if !status.IsValid() {
			t.Fatalf("requirement status %q should be valid", value)
		}
	}
	for _, value := range []string{"", "cancelled", "done"} {
		if _, err := NewApprovalRequirementStatus(value); !errors.Is(err, ErrInvalidApprovalRequirementStatus) {
			t.Fatalf("requirement status %q error = %v", value, err)
		}
	}
}

func TestNewApprovalDecisionValue(t *testing.T) {
	for _, value := range []string{"approved", "rejected", " approved "} {
		decision, err := NewApprovalDecisionValue(value)
		if err != nil {
			t.Fatalf("decision %q: %v", value, err)
		}
		if !decision.IsValid() {
			t.Fatalf("decision %q should be valid", value)
		}
	}
	for _, value := range []string{"", "pending", "cancelled"} {
		if _, err := NewApprovalDecisionValue(value); !errors.Is(err, ErrInvalidApprovalDecision) {
			t.Fatalf("decision %q error = %v", value, err)
		}
	}
}

func TestNewGovernanceAuditEventType(t *testing.T) {
	for _, value := range governanceAuditEventTypeStrings() {
		eventType, err := NewGovernanceAuditEventType(value)
		if err != nil {
			t.Fatalf("audit event type %q: %v", value, err)
		}
		if !eventType.IsValid() {
			t.Fatalf("audit event type %q should be valid", value)
		}
	}
	for _, value := range []string{"", "manual_override", "approval_request_deleted"} {
		if _, err := NewGovernanceAuditEventType(value); !errors.Is(err, ErrInvalidGovernanceAuditEventType) {
			t.Fatalf("audit event type %q error = %v", value, err)
		}
	}
}

func TestApprovalDecisionValidate(t *testing.T) {
	decision := ApprovalDecision{Decision: ApprovalDecisionValueApproved, DecidedBy: "alice"}
	if err := decision.Validate(); err != nil {
		t.Fatalf("valid decision: %v", err)
	}

	decision.Decision = "pending"
	if err := decision.Validate(); !errors.Is(err, ErrInvalidApprovalDecision) {
		t.Fatalf("invalid decision error = %v", err)
	}
	decision.Decision = ApprovalDecisionValueApproved
	decision.DecidedBy = "   "
	if err := decision.Validate(); !errors.Is(err, ErrInvalidApprovalActor) {
		t.Fatalf("invalid actor error = %v", err)
	}
	decision.DecidedBy = "alice"
	decision.Comment = strings.Repeat("x", MaxApprovalDecisionCommentLength+1)
	if err := decision.Validate(); !errors.Is(err, ErrApprovalDecisionCommentTooLong) {
		t.Fatalf("long comment error = %v", err)
	}
}

func assertAllowedValues(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func governanceSubjectTypeStrings() []string {
	values := GovernanceSubjectTypes()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func moduleOwnerRoleStrings() []string {
	values := ModuleOwnerRoles()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func approvalRequestStatusStrings() []string {
	values := ApprovalRequestStatuses()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func approvalRequirementTypeStrings() []string {
	values := ApprovalRequirementTypes()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func approvalRequirementStatusStrings() []string {
	values := ApprovalRequirementStatuses()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func approvalDecisionValueStrings() []string {
	values := ApprovalDecisionValues()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func governanceAuditEventTypeStrings() []string {
	values := GovernanceAuditEventTypes()
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}
