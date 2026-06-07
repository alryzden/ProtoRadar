package governance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/audit"
	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

type Service struct {
	modules      domain.ModuleRepository
	owners       domain.ModuleOwnerRepository
	audit        audit.AuditSink
	transactions domain.RegistryTransactionManager
	outbox       outbox.Writer
	clock        Clock
	ids          IDGenerator
}

func NewService(
	modules domain.ModuleRepository,
	owners domain.ModuleOwnerRepository,
	auditSink audit.AuditSink,
	transactions domain.RegistryTransactionManager,
	outboxWriter outbox.Writer,
	clock Clock,
	ids IDGenerator,
) *Service {
	return &Service{
		modules:      modules,
		owners:       owners,
		audit:        auditSink,
		transactions: transactions,
		outbox:       outboxWriter,
		clock:        clock,
		ids:          ids,
	}
}

type AddModuleOwnerInput struct {
	ModuleName  string
	SubjectType string
	Subject     string
	Role        string
	Actor       string
}

type RemoveModuleOwnerInput struct {
	ModuleName string
	OwnerID    string
	Actor      string
}

type ListModuleOwnersInput struct {
	ModuleName string
}

func (svc *Service) AddModuleOwner(ctx context.Context, input AddModuleOwnerInput) (domain.ModuleOwner, error) {
	moduleName, err := domain.NewModuleName(input.ModuleName)
	if err != nil {
		return domain.ModuleOwner{}, ErrInvalidModuleName
	}
	subjectType, err := domain.NewGovernanceSubjectType(input.SubjectType)
	if err != nil {
		return domain.ModuleOwner{}, ErrInvalidSubjectType
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return domain.ModuleOwner{}, ErrInvalidSubject
	}
	role, err := domain.NewModuleOwnerRole(input.Role)
	if err != nil {
		return domain.ModuleOwner{}, ErrInvalidModuleOwnerRole
	}
	actor, err := effectiveActorSubject(input.Actor)
	if err != nil {
		return domain.ModuleOwner{}, err
	}
	module, err := svc.modules.GetByName(ctx, moduleName)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ModuleOwner{}, ErrModuleNotFound
	}
	if err != nil {
		return domain.ModuleOwner{}, err
	}

	now := svc.clock.Now()
	ownerID, err := svc.ids.NewModuleOwnerID()
	if err != nil {
		return domain.ModuleOwner{}, err
	}
	owner := domain.ModuleOwner{
		ID:          ownerID,
		ModuleID:    module.ID,
		ModuleName:  module.Name,
		SubjectType: subjectType,
		Subject:     subject,
		Role:        role,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := owner.Validate(); err != nil {
		return domain.ModuleOwner{}, mapOwnerValidationError(err)
	}

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.owners.Add(txCtx, owner); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleOwnerAlreadyExists
			}
			return mapOwnerValidationError(err)
		}
		if err := svc.appendOwnerAudit(txCtx, domain.GovernanceAuditEventTypeModuleOwnerAdded, owner, actor, now); err != nil {
			return err
		}
		record, err := protoradarevents.NewModuleOwnerAdded(protoradarevents.ModuleOwnerAdded{Owner: owner, Actor: actor, OccurredAt: now})
		if err != nil {
			return err
		}
		return svc.outbox.Create(txCtx, record)
	})
	if err != nil {
		return domain.ModuleOwner{}, err
	}
	return owner, nil
}

func (svc *Service) RemoveModuleOwner(ctx context.Context, input RemoveModuleOwnerInput) error {
	moduleName, err := domain.NewModuleName(input.ModuleName)
	if err != nil {
		return ErrInvalidModuleName
	}
	ownerID := domain.NewModuleOwnerID(input.OwnerID)
	if ownerID == "" {
		return ErrInvalidModuleOwnerID
	}
	actor, err := effectiveActorSubject(input.Actor)
	if err != nil {
		return err
	}
	module, err := svc.modules.GetByName(ctx, moduleName)
	if errors.Is(err, domain.ErrNotFound) {
		return ErrModuleNotFound
	}
	if err != nil {
		return err
	}
	owner, err := svc.owners.GetByID(ctx, ownerID)
	if errors.Is(err, domain.ErrNotFound) {
		return ErrModuleOwnerNotFound
	}
	if err != nil {
		return err
	}
	if owner.ModuleID != module.ID {
		return ErrModuleOwnerNotFound
	}

	now := svc.clock.Now()
	owner.ModuleName = module.Name
	owner.UpdatedAt = now
	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.owners.Remove(txCtx, owner.ID, now); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return ErrModuleOwnerNotFound
			}
			return err
		}
		if err := svc.appendOwnerAudit(txCtx, domain.GovernanceAuditEventTypeModuleOwnerRemoved, owner, actor, now); err != nil {
			return err
		}
		record, err := protoradarevents.NewModuleOwnerRemoved(protoradarevents.ModuleOwnerRemoved{Owner: owner, Actor: actor, OccurredAt: now})
		if err != nil {
			return err
		}
		return svc.outbox.Create(txCtx, record)
	})
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) ListModuleOwners(ctx context.Context, input ListModuleOwnersInput) ([]domain.ModuleOwner, error) {
	moduleName, err := domain.NewModuleName(input.ModuleName)
	if err != nil {
		return nil, ErrInvalidModuleName
	}
	module, err := svc.modules.GetByName(ctx, moduleName)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrModuleNotFound
	}
	if err != nil {
		return nil, err
	}
	return svc.owners.ListByModule(ctx, module.ID)
}

func (svc *Service) appendOwnerAudit(ctx context.Context, eventType domain.GovernanceAuditEventType, owner domain.ModuleOwner, actor string, occurredAt time.Time) error {
	auditID, err := svc.ids.NewGovernanceAuditEventID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(moduleOwnerAuditPayload{
		OwnerID:     owner.ID.String(),
		SubjectType: owner.SubjectType.String(),
		Subject:     owner.Subject,
		Role:        owner.Role.String(),
	})
	if err != nil {
		return err
	}
	moduleID := owner.ModuleID
	return svc.audit.Record(ctx, audit.AuditEvent{
		ID:         auditID.String(),
		Type:       eventType.String(),
		Actor:      strings.TrimSpace(actor),
		Action:     eventType.String(),
		Outcome:    audit.OutcomeSuccess,
		Resource:   audit.AuditResource{Type: "module", ID: moduleID.String(), Name: owner.ModuleName.String()},
		Payload:    payload,
		OccurredAt: occurredAt,
	})
}

type moduleOwnerAuditPayload struct {
	OwnerID     string `json:"owner_id"`
	SubjectType string `json:"subject_type"`
	Subject     string `json:"subject"`
	Role        string `json:"role"`
}

func mapOwnerValidationError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidGovernanceSubjectType):
		return ErrInvalidSubjectType
	case errors.Is(err, domain.ErrInvalidGovernanceSubject):
		return ErrInvalidSubject
	case errors.Is(err, domain.ErrInvalidModuleOwnerRole):
		return ErrInvalidModuleOwnerRole
	default:
		return err
	}
}
