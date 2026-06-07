package governance

import (
	"strings"

	"github.com/alryzden/ProtoRadar/internal/identity"
)

const (
	GovernanceActorSourcePrincipal = "principal"
	GovernanceActorSourceOverride  = "override"
	GovernanceActorSourceSystem    = "system"
)

type GovernanceActor struct {
	Subject         string
	DisplayName     string
	Source          string
	PrincipalType   string
	Override        bool
	OriginalSubject string
}

type ActorOverridePolicy struct {
	Enabled bool
}

func ActorFromPrincipal(principal identity.Principal) (GovernanceActor, error) {
	subject := strings.TrimSpace(principal.Subject)
	if subject == "" {
		return GovernanceActor{}, ErrInvalidApprovalActor
	}
	displayName := strings.TrimSpace(principal.DisplayName)
	if displayName == "" {
		displayName = subject
	}
	return GovernanceActor{
		Subject:       subject,
		DisplayName:   displayName,
		Source:        GovernanceActorSourcePrincipal,
		PrincipalType: strings.TrimSpace(principal.Type.String()),
	}, nil
}

func ResolveGovernanceActor(principal identity.Principal, requestedActor string, policy ActorOverridePolicy) (GovernanceActor, error) {
	actor, err := ActorFromPrincipal(principal)
	if err != nil {
		return GovernanceActor{}, err
	}

	requested := strings.TrimSpace(requestedActor)
	if requested == "" || requested == actor.Subject {
		return actor, nil
	}
	if !policy.Enabled {
		return GovernanceActor{}, ErrGovernanceActorOverrideForbidden
	}

	return GovernanceActor{
		Subject:         requested,
		DisplayName:     requested,
		Source:          GovernanceActorSourceOverride,
		PrincipalType:   actor.PrincipalType,
		Override:        true,
		OriginalSubject: actor.Subject,
	}, nil
}

func effectiveActorSubject(actor string) (string, error) {
	subject := strings.TrimSpace(actor)
	if subject == "" {
		return "", ErrInvalidApprovalActor
	}
	return subject, nil
}
