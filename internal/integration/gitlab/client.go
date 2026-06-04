package gitlab

import (
	"context"
	"time"
)

type CommitStatusState string

const (
	CommitStatusStatePending CommitStatusState = "pending"
	CommitStatusStateRunning CommitStatusState = "running"
	CommitStatusStateSuccess CommitStatusState = "success"
	CommitStatusStateFailed  CommitStatusState = "failed"
)

type MergeRequest struct {
	ProjectID    int64
	IID          int64
	Title        string
	SourceBranch string
	TargetBranch string
	WebURL       string
	SHA          string
}

type MergeRequestNoteAuthor struct {
	Username string
	Name     string
}

type MergeRequestNote struct {
	ID        int64
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Author    MergeRequestNoteAuthor
}

type CommitStatus struct {
	State       CommitStatusState
	Name        string
	TargetURL   string
	Description string
}

type Client interface {
	GetMergeRequest(ctx context.Context, projectID int64, mergeRequestIID int64) (MergeRequest, error)
	ListMergeRequestNotes(ctx context.Context, projectID int64, mergeRequestIID int64) ([]MergeRequestNote, error)
	CreateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, body string) (MergeRequestNote, error)
	UpdateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, noteID int64, body string) (MergeRequestNote, error)
	SetCommitStatus(ctx context.Context, projectID int64, sha string, status CommitStatus) error
}
