package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeModuleGitLabProjectLinked = "protoradar.module_gitlab_project.linked"

type ModuleGitLabProjectLinkedPayload struct {
	ModuleID          string    `json:"module_id"`
	ModuleName        string    `json:"module_name"`
	GitLabBaseURL     string    `json:"gitlab_base_url"`
	GitLabProjectID   int64     `json:"gitlab_project_id"`
	GitLabProjectPath string    `json:"gitlab_project_path"`
	OccurredAt        time.Time `json:"occurred_at"`
}

func NewModuleGitLabProjectLinked(mapping domain.ModuleGitLabProject, occurredAt time.Time) (outbox.Record, error) {
	normalized, err := mapping.Normalized()
	if err != nil {
		return outbox.Record{}, err
	}

	payload := ModuleGitLabProjectLinkedPayload{
		ModuleID:          normalized.ModuleID.String(),
		ModuleName:        normalized.ModuleName.String(),
		GitLabBaseURL:     normalized.GitLabBaseURL,
		GitLabProjectID:   normalized.GitLabProjectID,
		GitLabProjectPath: normalized.GitLabProjectPath,
		OccurredAt:        occurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	moduleID := normalized.ModuleID.String()
	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   moduleID,
		EventType:     EventTypeModuleGitLabProjectLinked,
		DedupKey: fmt.Sprintf(
			"module:%s:gitlab-project:%s:%d:linked",
			moduleID,
			normalized.GitLabBaseURL,
			normalized.GitLabProjectID,
		),
		Payload:    body,
		OccurredAt: occurredAt,
	}, nil
}
