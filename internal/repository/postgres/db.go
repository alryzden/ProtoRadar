package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

const (
	uniqueViolationCode = "23505"
	checkViolationCode  = "23514"
)

type queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

type txContextKey struct{}

type DB struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

func (db *DB) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if txFromContext(ctx) != nil {
		return fn(ctx)
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}

	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}

	return tx.Commit(ctx)
}

func (db *DB) executor(ctx context.Context) queryer {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return db.pool
}

func txFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolationCode:
			return domain.ErrDuplicate
		case checkViolationCode:
			if mapped := mapCheckViolation(pgErr.ConstraintName); mapped != nil {
				return mapped
			}
		}
	}

	return err
}

func mapCheckViolation(constraintName string) error {
	switch constraintName {
	case "module_owners_subject_type_check":
		return domain.ErrInvalidGovernanceSubjectType
	case "module_owners_role_check":
		return domain.ErrInvalidModuleOwnerRole
	case "approval_requests_status_check":
		return domain.ErrInvalidApprovalRequestStatus
	case "approval_requirements_type_check":
		return domain.ErrInvalidApprovalRequirementType
	case "approval_requirements_status_check":
		return domain.ErrInvalidApprovalRequirementStatus
	case "approval_decisions_decision_check":
		return domain.ErrInvalidApprovalDecision
	case "governance_audit_events_type_check":
		return domain.ErrInvalidGovernanceAuditEventType
	default:
		return nil
	}
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

var _ domain.RegistryTransactionManager = (*DB)(nil)
