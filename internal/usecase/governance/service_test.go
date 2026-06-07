package governance

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/audit"
	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

func TestAddModuleOwnerSuccessWritesAuditAndOutbox(t *testing.T) {
	fixture := newFixture(t)

	owner, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{
		ModuleName:  "user-api",
		SubjectType: "user",
		Subject:     " alice ",
		Role:        "owner",
		Actor:       "admin",
	})
	if err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if owner.Subject != "alice" || owner.SubjectType != domain.GovernanceSubjectTypeUser || owner.Role != domain.ModuleOwnerRoleOwner {
		t.Fatalf("owner = %#v", owner)
	}
	if len(fixture.owners.byID) != 1 {
		t.Fatalf("stored owners = %d, want 1", len(fixture.owners.byID))
	}
	if len(fixture.audit.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(fixture.audit.events))
	}
	event := fixture.audit.events[0]
	if event.EventType != domain.GovernanceAuditEventTypeModuleOwnerAdded || event.Actor != "admin" || event.ModuleID == nil || *event.ModuleID != owner.ModuleID {
		t.Fatalf("audit event = %#v", event)
	}
	var payload moduleOwnerAuditPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("audit payload: %v", err)
	}
	if payload.OwnerID != owner.ID.String() || payload.Subject != "alice" || payload.Role != "owner" {
		t.Fatalf("audit payload = %#v", payload)
	}
	assertAuditPayloadSafe(t, event.PayloadJSON)
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d, want 1", len(fixture.outbox.records))
	}
	if fixture.outbox.records[0].EventType != "protoradar.module_owner.added" {
		t.Fatalf("outbox event type = %q", fixture.outbox.records[0].EventType)
	}
	assertOutboxActor(t, fixture.outbox.records[0].Payload, "admin")
	assertAuditPayloadSafe(t, fixture.outbox.records[0].Payload)
}

func TestAddModuleMaintainerSuccess(t *testing.T) {
	fixture := newFixture(t)

	owner, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{
		ModuleName:  "user-api",
		SubjectType: "team",
		Subject:     "platform",
		Role:        "maintainer",
		Actor:       "admin",
	})
	if err != nil {
		t.Fatalf("add maintainer: %v", err)
	}
	if owner.SubjectType != domain.GovernanceSubjectTypeTeam || owner.Role != domain.ModuleOwnerRoleMaintainer {
		t.Fatalf("owner = %#v", owner)
	}
}

func TestAddModuleOwnerRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		input AddModuleOwnerInput
		want  error
	}{
		{name: "subject type", input: AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "service", Subject: "alice", Role: "owner"}, want: ErrInvalidSubjectType},
		{name: "subject", input: AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "  ", Role: "owner"}, want: ErrInvalidSubject},
		{name: "role", input: AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "admin"}, want: ErrInvalidModuleOwnerRole},
		{name: "actor", input: AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "owner"}, want: ErrInvalidApprovalActor},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newFixture(t)
			if _, err := fixture.service.AddModuleOwner(context.Background(), tt.input); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if len(fixture.owners.byID) != 0 || len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
				t.Fatalf("invalid input should not write state")
			}
		})
	}
}

func TestAddModuleOwnerUnknownModuleReturnsNotFound(t *testing.T) {
	fixture := newFixture(t)

	_, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "missing-api", SubjectType: "user", Subject: "alice", Role: "owner", Actor: "admin"})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("error = %v, want ErrModuleNotFound", err)
	}
}

func TestAddModuleOwnerDuplicateReturnsConflict(t *testing.T) {
	fixture := newFixture(t)
	input := AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "owner", Actor: "admin"}
	if _, err := fixture.service.AddModuleOwner(context.Background(), input); err != nil {
		t.Fatalf("add first owner: %v", err)
	}
	if _, err := fixture.service.AddModuleOwner(context.Background(), input); !errors.Is(err, ErrModuleOwnerAlreadyExists) {
		t.Fatalf("duplicate error = %v, want ErrModuleOwnerAlreadyExists", err)
	}
}

func TestAddModuleOwnerRollbackPreventsOwnerAuditAndOutbox(t *testing.T) {
	fixture := newFixture(t)
	fixture.audit.failAppend = errors.New("audit failed")

	_, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "owner", Actor: "admin"})
	if !errors.Is(err, fixture.audit.failAppend) {
		t.Fatalf("error = %v, want audit failure", err)
	}
	if len(fixture.owners.byID) != 0 || len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback did not restore state: owners=%d audit=%d outbox=%d", len(fixture.owners.byID), len(fixture.audit.events), len(fixture.outbox.records))
	}
}

func TestOwnerAuditPayloadDoesNotIncludeSecretsOrTokens(t *testing.T) {
	fixture := newFixture(t)

	_, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{
		ModuleName:  "user-api",
		SubjectType: "user",
		Subject:     "alice",
		Role:        "owner",
		Actor:       "admin",
	})
	if err != nil {
		t.Fatalf("add owner: %v", err)
	}
	assertAuditPayloadSafe(t, fixture.audit.events[0].PayloadJSON)
}

func TestRemoveModuleOwnerSuccessWritesAuditAndOutbox(t *testing.T) {
	fixture := newFixture(t)
	owner, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "team", Subject: "platform", Role: "owner", Actor: "admin"})
	if err != nil {
		t.Fatalf("add owner: %v", err)
	}
	fixture.audit.events = nil
	fixture.outbox.records = nil

	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "user-api", OwnerID: owner.ID.String(), Actor: "admin"}); err != nil {
		t.Fatalf("remove owner: %v", err)
	}
	if len(fixture.owners.byID) != 0 {
		t.Fatalf("owner should be removed")
	}
	if len(fixture.audit.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(fixture.audit.events))
	}
	event := fixture.audit.events[0]
	if event.EventType != domain.GovernanceAuditEventTypeModuleOwnerRemoved || event.Actor != "admin" {
		t.Fatalf("audit event = %#v", event)
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d, want 1", len(fixture.outbox.records))
	}
	if fixture.outbox.records[0].EventType != "protoradar.module_owner.removed" {
		t.Fatalf("outbox event type = %q", fixture.outbox.records[0].EventType)
	}
	assertOutboxActor(t, fixture.outbox.records[0].Payload, "admin")
	assertAuditPayloadSafe(t, fixture.outbox.records[0].Payload)
}

func TestRemoveModuleOwnerWrongModuleReturnsNotFound(t *testing.T) {
	fixture := newFixture(t)
	owner, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "owner", Actor: "admin"})
	if err != nil {
		t.Fatalf("add owner: %v", err)
	}

	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "billing-api", OwnerID: owner.ID.String(), Actor: "admin"}); !errors.Is(err, ErrModuleOwnerNotFound) {
		t.Fatalf("error = %v, want ErrModuleOwnerNotFound", err)
	}
}

func TestRemoveModuleOwnerRollbackPreventsOwnerAuditAndOutbox(t *testing.T) {
	fixture := newFixture(t)
	owner, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "owner", Actor: "admin"})
	if err != nil {
		t.Fatalf("add owner: %v", err)
	}
	fixture.audit.events = nil
	fixture.outbox.records = nil
	fixture.outbox.failCreate = errors.New("outbox failed")

	err = fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "user-api", OwnerID: owner.ID.String(), Actor: "admin"})
	if !errors.Is(err, fixture.outbox.failCreate) {
		t.Fatalf("error = %v, want outbox failure", err)
	}
	if len(fixture.owners.byID) != 1 || len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback did not restore state: owners=%d audit=%d outbox=%d", len(fixture.owners.byID), len(fixture.audit.events), len(fixture.outbox.records))
	}
}

func TestRemoveModuleOwnerUnknownInputs(t *testing.T) {
	fixture := newFixture(t)
	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "bad name", OwnerID: "owner-1"}); !errors.Is(err, ErrInvalidModuleName) {
		t.Fatalf("invalid module error = %v", err)
	}
	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "user-api", OwnerID: "   "}); !errors.Is(err, ErrInvalidModuleOwnerID) {
		t.Fatalf("invalid owner id error = %v", err)
	}
	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "missing-api", OwnerID: "owner-1", Actor: "admin"}); !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("missing module error = %v", err)
	}
	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "user-api", OwnerID: "missing-owner", Actor: "admin"}); !errors.Is(err, ErrModuleOwnerNotFound) {
		t.Fatalf("missing owner error = %v", err)
	}
	if err := fixture.service.RemoveModuleOwner(context.Background(), RemoveModuleOwnerInput{ModuleName: "user-api", OwnerID: "owner-1"}); !errors.Is(err, ErrInvalidApprovalActor) {
		t.Fatalf("invalid actor error = %v", err)
	}
}

func TestListModuleOwners(t *testing.T) {
	fixture := newFixture(t)
	owner, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "user", Subject: "alice", Role: "owner", Actor: "admin"})
	if err != nil {
		t.Fatalf("add owner: %v", err)
	}
	maintainer, err := fixture.service.AddModuleOwner(context.Background(), AddModuleOwnerInput{ModuleName: "user-api", SubjectType: "team", Subject: "platform", Role: "maintainer", Actor: "admin"})
	if err != nil {
		t.Fatalf("add maintainer: %v", err)
	}

	owners, err := fixture.service.ListModuleOwners(context.Background(), ListModuleOwnersInput{ModuleName: "user-api"})
	if err != nil {
		t.Fatalf("list owners: %v", err)
	}
	if len(owners) != 2 {
		t.Fatalf("owners = %d, want 2", len(owners))
	}
	ids := []domain.ModuleOwnerID{owners[0].ID, owners[1].ID}
	if !slices.Contains(ids, owner.ID) || !slices.Contains(ids, maintainer.ID) {
		t.Fatalf("owners = %#v", owners)
	}
}

func TestListModuleOwnersUnknownModuleReturnsNotFound(t *testing.T) {
	fixture := newFixture(t)

	_, err := fixture.service.ListModuleOwners(context.Background(), ListModuleOwnersInput{ModuleName: "missing-api"})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("error = %v, want ErrModuleNotFound", err)
	}
}

type fixture struct {
	service *Service
	modules *fakeModules
	owners  *fakeOwners
	audit   *fakeAudit
	tx      *fakeTransactions
	outbox  *fakeOutbox
	ids     *fakeIDs
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	userAPIName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	billingAPIName, err := domain.NewModuleName("billing-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	modules := &fakeModules{byName: map[string]domain.Module{
		"user-api":    {ID: domain.NewModuleID("module-1"), Name: userAPIName},
		"billing-api": {ID: domain.NewModuleID("module-2"), Name: billingAPIName},
	}}
	owners := &fakeOwners{byID: map[string]domain.ModuleOwner{}}
	auditRepo := &fakeAudit{}
	outboxWriter := &fakeOutbox{}
	transaction := &fakeTransactions{owners: owners, audit: auditRepo, outbox: outboxWriter}
	ids := &fakeIDs{}
	service := NewService(modules, owners, audit.NewCommunityAuditSink(auditRepo), transaction, outboxWriter, fixedClock{now: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)}, ids)
	return &fixture{service: service, modules: modules, owners: owners, audit: auditRepo, tx: transaction, outbox: outboxWriter, ids: ids}
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

type fakeIDs struct {
	ownerID       int
	requestID     int
	requirementID int
	decisionID    int
	auditID       int
}

func (ids *fakeIDs) NewModuleOwnerID() (domain.ModuleOwnerID, error) {
	ids.ownerID++
	return domain.NewModuleOwnerID("owner-" + strconv.Itoa(ids.ownerID)), nil
}

func (ids *fakeIDs) NewApprovalRequestID() (domain.ApprovalRequestID, error) {
	ids.requestID++
	return domain.NewApprovalRequestID("request-" + strconv.Itoa(ids.requestID)), nil
}

func (ids *fakeIDs) NewApprovalRequirementID() (domain.ApprovalRequirementID, error) {
	ids.requirementID++
	return domain.NewApprovalRequirementID("requirement-" + strconv.Itoa(ids.requirementID)), nil
}

func (ids *fakeIDs) NewApprovalDecisionID() (domain.ApprovalDecisionID, error) {
	ids.decisionID++
	return domain.NewApprovalDecisionID("decision-" + strconv.Itoa(ids.decisionID)), nil
}

func (ids *fakeIDs) NewGovernanceAuditEventID() (domain.GovernanceAuditEventID, error) {
	ids.auditID++
	return domain.NewGovernanceAuditEventID("audit-" + strconv.Itoa(ids.auditID)), nil
}

type fakeModules struct {
	byName map[string]domain.Module
}

func (repo *fakeModules) Create(ctx context.Context, module domain.Module) error { return nil }
func (repo *fakeModules) GetByID(ctx context.Context, id domain.ModuleID) (domain.Module, error) {
	for _, module := range repo.byName {
		if module.ID == id {
			return module, nil
		}
	}
	return domain.Module{}, domain.ErrNotFound
}
func (repo *fakeModules) GetByName(ctx context.Context, name domain.ModuleName) (domain.Module, error) {
	module, ok := repo.byName[name.String()]
	if !ok {
		return domain.Module{}, domain.ErrNotFound
	}
	return module, nil
}
func (repo *fakeModules) List(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	return nil, nil
}

type fakeOwners struct {
	byID map[string]domain.ModuleOwner
}

func (repo *fakeOwners) Add(ctx context.Context, owner domain.ModuleOwner) error {
	for _, existing := range repo.byID {
		if existing.ModuleID == owner.ModuleID && existing.SubjectType == owner.SubjectType && existing.Subject == owner.Subject && existing.Role == owner.Role {
			return domain.ErrDuplicate
		}
	}
	repo.byID[owner.ID.String()] = owner
	return nil
}

func (repo *fakeOwners) Remove(ctx context.Context, id domain.ModuleOwnerID, updatedAt time.Time) error {
	if _, ok := repo.byID[id.String()]; !ok {
		return domain.ErrNotFound
	}
	delete(repo.byID, id.String())
	return nil
}

func (repo *fakeOwners) GetByID(ctx context.Context, id domain.ModuleOwnerID) (domain.ModuleOwner, error) {
	owner, ok := repo.byID[id.String()]
	if !ok {
		return domain.ModuleOwner{}, domain.ErrNotFound
	}
	return owner, nil
}

func (repo *fakeOwners) ListByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleOwner, error) {
	owners := make([]domain.ModuleOwner, 0)
	for _, owner := range repo.byID {
		if owner.ModuleID == moduleID {
			owners = append(owners, owner)
		}
	}
	return owners, nil
}

func (repo *fakeOwners) HasRole(ctx context.Context, moduleID domain.ModuleID, subjectType domain.GovernanceSubjectType, subject string, roles []domain.ModuleOwnerRole) (bool, error) {
	allowed := make(map[domain.ModuleOwnerRole]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	for _, owner := range repo.byID {
		if owner.ModuleID != moduleID || owner.SubjectType != subjectType || owner.Subject != subject {
			continue
		}
		if _, ok := allowed[owner.Role]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (repo *fakeOwners) snapshot() map[string]domain.ModuleOwner {
	copy := make(map[string]domain.ModuleOwner, len(repo.byID))
	for key, value := range repo.byID {
		copy[key] = value
	}
	return copy
}

func (repo *fakeOwners) restore(snapshot map[string]domain.ModuleOwner) {
	repo.byID = snapshot
}

type fakeAudit struct {
	events     []domain.GovernanceAuditEvent
	failAppend error
}

func (repo *fakeAudit) Append(ctx context.Context, event domain.GovernanceAuditEvent) error {
	if repo.failAppend != nil {
		return repo.failAppend
	}
	repo.events = append(repo.events, event)
	return nil
}

func (repo *fakeAudit) ListByApprovalRequest(ctx context.Context, approvalRequestID domain.ApprovalRequestID, limit int, offset int) ([]domain.GovernanceAuditEvent, error) {
	return nil, nil
}

func (repo *fakeAudit) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.GovernanceAuditEvent, error) {
	return nil, nil
}

func (repo *fakeAudit) snapshot() []domain.GovernanceAuditEvent {
	return append([]domain.GovernanceAuditEvent(nil), repo.events...)
}

func (repo *fakeAudit) restore(snapshot []domain.GovernanceAuditEvent) {
	repo.events = snapshot
}

type fakeOutbox struct {
	records    []outbox.Record
	failCreate error
}

func (writer *fakeOutbox) Create(ctx context.Context, record outbox.Record) error {
	if writer.failCreate != nil {
		return writer.failCreate
	}
	writer.records = append(writer.records, record)
	return nil
}

func (writer *fakeOutbox) snapshot() []outbox.Record {
	return append([]outbox.Record(nil), writer.records...)
}

func (writer *fakeOutbox) restore(snapshot []outbox.Record) {
	writer.records = snapshot
}

type fakeTransactions struct {
	owners    *fakeOwners
	approvals *fakeApprovals
	audit     *fakeAudit
	outbox    *fakeOutbox
}

func assertAuditPayloadSafe(t *testing.T, payload []byte) {
	t.Helper()
	lowered := strings.ToLower(string(payload))
	for _, forbidden := range []string{"authorization", "bearer", "api_token", "raw_token", "token_hash", "s3_secret", "gitlab_token"} {
		if strings.Contains(lowered, forbidden) {
			t.Fatalf("audit payload contains %q: %s", forbidden, string(payload))
		}
	}
}

func assertOutboxActor(t *testing.T, payload []byte, want string) {
	t.Helper()
	var body struct {
		Actor string `json:"actor"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("outbox payload: %v", err)
	}
	if body.Actor != want {
		t.Fatalf("outbox actor = %q, want %q; payload=%s", body.Actor, want, string(payload))
	}
}

func assertDecisionOutboxActor(t *testing.T, payload []byte, want string) {
	t.Helper()
	var body struct {
		DecidedBy string `json:"decided_by"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("decision outbox payload: %v", err)
	}
	if body.DecidedBy != want {
		t.Fatalf("decision outbox actor = %q, want %q; payload=%s", body.DecidedBy, want, string(payload))
	}
}

func (tx *fakeTransactions) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	var ownerSnapshot map[string]domain.ModuleOwner
	if tx.owners != nil {
		ownerSnapshot = tx.owners.snapshot()
	}
	var approvalSnapshot map[string]domain.ApprovalRequest
	if tx.approvals != nil {
		approvalSnapshot = tx.approvals.snapshot()
	}
	auditSnapshot := tx.audit.snapshot()
	outboxSnapshot := tx.outbox.snapshot()
	if err := fn(ctx); err != nil {
		if tx.owners != nil {
			tx.owners.restore(ownerSnapshot)
		}
		if tx.approvals != nil {
			tx.approvals.restore(approvalSnapshot)
		}
		tx.audit.restore(auditSnapshot)
		tx.outbox.restore(outboxSnapshot)
		return err
	}
	return nil
}
