package httptransport

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/authorization"
)

func TestCoreRegistryRoutesAuthorizerCoverage(t *testing.T) {
	runProtectedRouteAuthorizationCoverage(t, coreRegistryAuthorizationRoutes())
}

func TestIntegrationRuntimeAndEditionRoutesAuthorizerCoverage(t *testing.T) {
	runProtectedRouteAuthorizationCoverage(t, integrationRuntimeAndEditionAuthorizationRoutes())
}

func TestGovernanceRoutesAuthorizerCoverage(t *testing.T) {
	for _, tt := range governanceAuthorizationRoutes() {
		t.Run(tt.name+"/deny_all", func(t *testing.T) {
			authorizer := &fakeAuthorizer{denyAll: true}
			governanceFake := newFakeGovernance()
			server := newProtectedRouteTestServerWithGovernance(authorizer, governanceFake)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "Bearer valid", contentType)

			assertForbiddenByAuthorizer(t, res)
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
			assertGovernanceNotCalled(t, governanceFake)
		})

		t.Run(tt.name+"/allow_all", func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			governanceFake := newFakeGovernance()
			server := newProtectedRouteTestServerWithGovernance(authorizer, governanceFake)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "Bearer valid", contentType)

			if res.Code != tt.allowedStatus {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, tt.allowedStatus, res.Body.String())
			}
			if res.Code == http.StatusForbidden {
				t.Fatalf("allowed Authorizer unexpectedly returned forbidden: %s", res.Body.String())
			}
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
			if governanceFake.totalCalls() == 0 {
				t.Fatalf("governance usecase was not called on allowed request")
			}
		})

		t.Run(tt.name+"/missing_bearer", func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			governanceFake := newFakeGovernance()
			server := newProtectedRouteTestServerWithGovernance(authorizer, governanceFake)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "", contentType)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
			assertAPIError(t, res, "unauthorized")
			assertAuthorizerNotCalled(t, authorizer)
			assertGovernanceNotCalled(t, governanceFake)
		})

		t.Run(tt.name+"/invalid_bearer", func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			governanceFake := newFakeGovernance()
			server := newProtectedRouteTestServerWithGovernance(authorizer, governanceFake)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "Bearer invalid", contentType)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
			assertAPIError(t, res, "unauthorized")
			assertAuthorizerNotCalled(t, authorizer)
			assertGovernanceNotCalled(t, governanceFake)
		})
	}
}

func TestGovernanceAuthorizerDenialPrecedesSpoofedActorResolution(t *testing.T) {
	authorizer := &fakeAuthorizer{denyAll: true}
	governanceFake := newFakeGovernance()
	server := newProtectedRouteTestServerWithGovernance(authorizer, governanceFake)

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "user",
		"subject": "alice",
		"role": "owner",
		"actor": "mallory"
	}`), "Bearer valid", "application/json")

	assertForbiddenByAuthorizer(t, res)
	assertAuthorizerCall(t, authorizer, authorization.ActionGovernanceOwnerManage, authorization.Resource{Type: "module", Name: "user-api"})
	assertGovernanceNotCalled(t, governanceFake)
}

func runProtectedRouteAuthorizationCoverage(t *testing.T, routes []protectedRouteAuthorizationCase) {
	t.Helper()
	for _, tt := range routes {
		t.Run(tt.name+"/deny_all", func(t *testing.T) {
			authorizer := &fakeAuthorizer{denyAll: true}
			server := newProtectedRouteTestServer(authorizer)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "Bearer valid", contentType)

			assertForbiddenByAuthorizer(t, res)
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
		})

		t.Run(tt.name+"/allow_all", func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			server := newProtectedRouteTestServer(authorizer)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "Bearer valid", contentType)

			if res.Code != tt.allowedStatus {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, tt.allowedStatus, res.Body.String())
			}
			if res.Code == http.StatusForbidden {
				t.Fatalf("allowed Authorizer unexpectedly returned forbidden: %s", res.Body.String())
			}
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
		})

		t.Run(tt.name+"/missing_bearer", func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			server := newProtectedRouteTestServer(authorizer)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "", contentType)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
			assertAPIError(t, res, "unauthorized")
			assertAuthorizerNotCalled(t, authorizer)
		})

		t.Run(tt.name+"/invalid_bearer", func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			server := newProtectedRouteTestServer(authorizer)

			body, contentType := tt.requestBody()
			res := request(t, server, tt.method, tt.path, body, "Bearer invalid", contentType)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
			assertAPIError(t, res, "unauthorized")
			assertAuthorizerNotCalled(t, authorizer)
		})
	}
}

func TestAuthenticatedAPIRoutesRejectMissingAuthenticationBeforeAuthorizer(t *testing.T) {
	routes := authenticatedRouteExpectations()

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			server := newAuthorizerCoverageTestServer(authorizer)

			res := request(t, server, tt.method, tt.path, tt.bodyReader(), "", tt.contentType)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
			assertAPIError(t, res, "unauthorized")
			assertAuthorizerNotCalled(t, authorizer)
		})
	}
}

func TestAuthenticatedAPIRoutesRejectInvalidAuthenticationBeforeAuthorizer(t *testing.T) {
	routes := authenticatedRouteExpectations()

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			server := newAuthorizerCoverageTestServer(authorizer)

			res := request(t, server, tt.method, tt.path, tt.bodyReader(), "Bearer invalid", tt.contentType)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
			}
			assertAPIError(t, res, "unauthorized")
			assertAuthorizerNotCalled(t, authorizer)
		})
	}
}

func TestAuthenticatedAPIRoutesInvokeAuthorizerWhenAllowed(t *testing.T) {
	routes := authenticatedRouteExpectations()

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			server := newAuthorizerCoverageTestServer(authorizer)

			res := request(t, server, tt.method, tt.path, tt.bodyReader(), "Bearer valid", tt.contentType)
			if res.Code == http.StatusUnauthorized || res.Code == http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
		})
	}
}

func TestAuthenticatedAPIRoutesDenyThroughAuthorizer(t *testing.T) {
	routes := authenticatedRouteExpectations()

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			authorizer := &fakeAuthorizer{denyAll: true}
			server := newAuthorizerCoverageTestServer(authorizer)

			res := request(t, server, tt.method, tt.path, tt.bodyReader(), "Bearer valid", tt.contentType)
			assertForbiddenByAuthorizer(t, res)
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
		})
	}
}

func TestAuthenticatedAPIRoutesDenySelectedActionsThroughAuthorizer(t *testing.T) {
	routes := authenticatedRouteExpectations()

	for _, tt := range routes {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			authorizer := &fakeAuthorizer{
				denyActions: map[authorization.Action]bool{tt.action: true},
			}
			server := newAuthorizerCoverageTestServer(authorizer)

			res := request(t, server, tt.method, tt.path, tt.bodyReader(), "Bearer valid", tt.contentType)
			if res.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusForbidden, res.Body.String())
			}
			assertAPIError(t, res, "forbidden")
			assertAuthorizerCall(t, authorizer, tt.action, tt.resource)
		})
	}
}
