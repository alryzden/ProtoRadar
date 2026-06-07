package governance

import (
	"errors"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/identity"
)

func TestActorFromPrincipalUsesAPITokenPrincipal(t *testing.T) {
	actor, err := ActorFromPrincipal(identity.Principal{
		Subject:     "ci-token",
		DisplayName: "CI token",
		Type:        identity.PrincipalTypeAPIToken,
		Metadata:    map[string]string{"api_token_id": "token-1"},
	})
	if err != nil {
		t.Fatalf("actor from principal: %v", err)
	}
	if actor.Subject != "ci-token" || actor.DisplayName != "CI token" || actor.PrincipalType != "api_token" || actor.Source != GovernanceActorSourcePrincipal || actor.Override {
		t.Fatalf("actor = %#v", actor)
	}
}

func TestActorFromPrincipalRejectsEmptySubject(t *testing.T) {
	_, err := ActorFromPrincipal(identity.Principal{Type: identity.PrincipalTypeAPIToken})
	if !errors.Is(err, ErrInvalidApprovalActor) {
		t.Fatalf("error = %v, want ErrInvalidApprovalActor", err)
	}
}

func TestResolveGovernanceActorUsesPrincipalWhenNoRequestedActor(t *testing.T) {
	actor, err := ResolveGovernanceActor(testPrincipal(), "", ActorOverridePolicy{})
	if err != nil {
		t.Fatalf("resolve actor: %v", err)
	}
	if actor.Subject != "ci-token" || actor.Override {
		t.Fatalf("actor = %#v", actor)
	}
}

func TestResolveGovernanceActorRejectsSpoofedActorWhenOverrideDisabled(t *testing.T) {
	_, err := ResolveGovernanceActor(testPrincipal(), "alice", ActorOverridePolicy{})
	if !errors.Is(err, ErrGovernanceActorOverrideForbidden) {
		t.Fatalf("error = %v, want ErrGovernanceActorOverrideForbidden", err)
	}
}

func TestResolveGovernanceActorAllowsSameActorWhenOverrideDisabled(t *testing.T) {
	actor, err := ResolveGovernanceActor(testPrincipal(), "ci-token", ActorOverridePolicy{})
	if err != nil {
		t.Fatalf("resolve actor: %v", err)
	}
	if actor.Subject != "ci-token" || actor.Override {
		t.Fatalf("actor = %#v", actor)
	}
}

func TestResolveGovernanceActorAllowsOverrideWhenPolicyEnabled(t *testing.T) {
	actor, err := ResolveGovernanceActor(testPrincipal(), "alice", ActorOverridePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("resolve actor: %v", err)
	}
	if actor.Subject != "alice" || actor.DisplayName != "alice" || actor.Source != GovernanceActorSourceOverride || !actor.Override || actor.OriginalSubject != "ci-token" {
		t.Fatalf("actor = %#v", actor)
	}
}

func TestActorFromPrincipalDoesNotUseRawTokenMetadata(t *testing.T) {
	actor, err := ActorFromPrincipal(identity.Principal{
		Subject: "ci-token",
		Type:    identity.PrincipalTypeAPIToken,
		Metadata: map[string]string{
			"raw_token":    "prr_secret",
			"api_token_id": "token-1",
		},
	})
	if err != nil {
		t.Fatalf("actor from principal: %v", err)
	}
	for _, value := range []string{actor.Subject, actor.DisplayName, actor.OriginalSubject} {
		if value == "prr_secret" {
			t.Fatalf("raw token leaked into actor: %#v", actor)
		}
	}
}

func testPrincipal() identity.Principal {
	return identity.Principal{
		Subject:     "ci-token",
		DisplayName: "CI token",
		Type:        identity.PrincipalTypeAPIToken,
	}
}
