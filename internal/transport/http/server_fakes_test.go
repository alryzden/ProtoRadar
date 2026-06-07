package httptransport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/identity"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
)

type fakeRegistry struct {
	now                time.Time
	modules            map[string]domain.Module
	versions           map[string]domain.ModuleVersion
	artifacts          map[string][]domain.Artifact
	objects            map[string][]byte
	objectReaders      map[string]io.ReadCloser
	bufConfigs         map[string]domain.BufConfigInfo
	lintResults        map[string]domain.BufLintResult
	metadata           map[string]domain.DescriptorMetadata
	reports            map[string]registry.CheckBreakingResponse
	gitLabProjects     map[string]domain.ModuleGitLabProject
	dependencies       []domain.ModuleDependency
	unresolved         []domain.UnresolvedProtoDependency
	runtimeServices    map[string]domain.RuntimeService
	runtimeDeployments []domain.RuntimeDeployment
	runtimeUsages      []domain.RuntimeModuleUsage
	dependencyErr      error
	runtimeErr         error
	publishErr         error
	deprecateInput     registry.DeprecateModuleVersionInput
	linkGitLabErr      error
	publishLint        domain.BufLintResult
	checkErr           error
	breakingStatus     domain.BreakingReportStatus
	breakingChanges    []domain.BreakingChange
}

type fakeAuthProvider struct {
	mu                          sync.Mutex
	principal                   identity.Principal
	seen                        identity.AuthRequest
	err                         error
	acceptedAuthorizationHeader string
}

func (fake *fakeAuthProvider) Authenticate(ctx context.Context, req identity.AuthRequest) (identity.Principal, error) {
	fake.mu.Lock()
	fake.seen = req
	fake.mu.Unlock()

	if fake.err != nil {
		return identity.Principal{}, fake.err
	}
	if fake.acceptedAuthorizationHeader != "" && req.AuthorizationHeader != fake.acceptedAuthorizationHeader {
		return identity.Principal{}, registry.ErrInvalidOrExpiredToken
	}
	return fake.principal, nil
}

type fakeAuthorizer struct {
	mu            sync.Mutex
	seenPrincipal identity.Principal
	seenAction    authorization.Action
	seenResource  authorization.Resource
	calls         []fakeAuthorizeCall
	denyAll       bool
	denyActions   map[authorization.Action]bool
	denyTargets   []fakeAuthorizeCall
	err           error
}

type fakeAuthorizeCall struct {
	principal identity.Principal
	action    authorization.Action
	resource  authorization.Resource
}

func (fake *fakeAuthorizer) Authorize(ctx context.Context, principal identity.Principal, action authorization.Action, resource authorization.Resource) error {
	resource = cloneAuthorizationResource(resource)
	call := fakeAuthorizeCall{
		principal: principal,
		action:    action,
		resource:  resource,
	}

	fake.mu.Lock()
	fake.seenPrincipal = principal
	fake.seenAction = action
	fake.seenResource = resource
	fake.calls = append(fake.calls, call)
	denyAll := fake.denyAll
	denyAction := fake.denyActions[action]
	denyTarget := false
	for _, target := range fake.denyTargets {
		if target.action == action && reflect.DeepEqual(target.resource, resource) {
			denyTarget = true
			break
		}
	}
	err := fake.err
	fake.mu.Unlock()

	if denyAll || denyAction || denyTarget {
		return authorization.ErrForbidden
	}
	return err
}

func (fake *fakeAuthorizer) recordedCalls() []fakeAuthorizeCall {
	fake.mu.Lock()
	defer fake.mu.Unlock()

	calls := make([]fakeAuthorizeCall, 0, len(fake.calls))
	for _, call := range fake.calls {
		call.resource = cloneAuthorizationResource(call.resource)
		calls = append(calls, call)
	}
	return calls
}

func assertAuthorizerNotCalled(t *testing.T, authorizer *fakeAuthorizer) {
	t.Helper()
	calls := authorizer.recordedCalls()
	if len(calls) != 0 {
		t.Fatalf("authorizer calls = %d, want 0: %#v", len(calls), calls)
	}
}

func assertAuthorizerCall(t *testing.T, authorizer *fakeAuthorizer, action authorization.Action, resource authorization.Resource) {
	t.Helper()
	call := assertAuthorizerCalledOnce(t, authorizer)
	if call.principal.Subject != "test" || call.principal.Type != identity.PrincipalTypeAPIToken {
		t.Fatalf("principal = %#v, want test api_token principal", call.principal)
	}
	if call.action != action {
		t.Fatalf("action = %s, want %s", call.action, action)
	}
	assertResource(t, call.resource, resource)
}

func assertAuthorizerCalledOnce(t *testing.T, authorizer *fakeAuthorizer) fakeAuthorizeCall {
	t.Helper()
	calls := authorizer.recordedCalls()
	if len(calls) != 1 {
		t.Fatalf("authorizer calls = %d, want 1: %#v", len(calls), calls)
	}
	return calls[0]
}

func assertGovernanceNotCalled(t *testing.T, governanceFake *fakeGovernance) {
	t.Helper()
	if calls := governanceFake.totalCalls(); calls != 0 {
		t.Fatalf("governance calls = %d, want 0", calls)
	}
}

func cloneAuthorizationResource(resource authorization.Resource) authorization.Resource {
	if resource.Attributes == nil {
		return resource
	}
	attributes := make(map[string]string, len(resource.Attributes))
	for key, value := range resource.Attributes {
		attributes[key] = value
	}
	resource.Attributes = attributes
	return resource
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		now:             time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		modules:         map[string]domain.Module{},
		versions:        map[string]domain.ModuleVersion{},
		artifacts:       map[string][]domain.Artifact{},
		objects:         map[string][]byte{},
		objectReaders:   map[string]io.ReadCloser{},
		bufConfigs:      map[string]domain.BufConfigInfo{},
		lintResults:     map[string]domain.BufLintResult{},
		metadata:        map[string]domain.DescriptorMetadata{},
		reports:         map[string]registry.CheckBreakingResponse{},
		gitLabProjects:  map[string]domain.ModuleGitLabProject{},
		runtimeServices: map[string]domain.RuntimeService{},
		publishLint:     domain.BufLintResult{Status: domain.BufLintStatusPassed},
		breakingStatus:  domain.BreakingReportStatusPassed,
	}
}

func (fake *fakeRegistry) AuthenticateToken(ctx context.Context, rawToken string) (registry.AuthSubject, error) {
	if rawToken != "valid" {
		return registry.AuthSubject{}, registry.ErrInvalidOrExpiredToken
	}
	return registry.AuthSubject{Name: "test"}, nil
}

func (fake *fakeRegistry) CreateModule(ctx context.Context, req registry.CreateModuleRequest) (domain.Module, error) {
	name, err := domain.NewModuleName(req.Name)
	if err != nil {
		return domain.Module{}, registry.ErrInvalidModuleName
	}
	if _, exists := fake.modules[name.String()]; exists {
		return domain.Module{}, registry.ErrModuleAlreadyExists
	}
	module := domain.Module{
		ID:            domain.NewModuleID("module-" + name.String()),
		Name:          name,
		Description:   req.Description,
		RepositoryURL: req.RepositoryURL,
		CreatedAt:     fake.now,
		UpdatedAt:     fake.now,
	}
	fake.modules[name.String()] = module
	return module, nil
}

func (fake *fakeRegistry) ListModules(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	modules := make([]domain.Module, 0, len(fake.modules))
	for _, module := range fake.modules {
		modules = append(modules, module)
	}
	return modules, nil
}

func (fake *fakeRegistry) GetModule(ctx context.Context, name string) (domain.Module, error) {
	module, exists := fake.modules[name]
	if !exists {
		return domain.Module{}, registry.ErrModuleNotFound
	}
	return module, nil
}

func (fake *fakeRegistry) LinkModuleGitLabProject(ctx context.Context, input registry.LinkModuleGitLabProjectInput) (registry.LinkModuleGitLabProjectOutput, error) {
	if fake.linkGitLabErr != nil {
		return registry.LinkModuleGitLabProjectOutput{}, fake.linkGitLabErr
	}
	module, exists := fake.modules[input.ModuleName]
	if !exists {
		return registry.LinkModuleGitLabProjectOutput{}, registry.ErrModuleNotFound
	}
	mapping := domain.ModuleGitLabProject{
		ID:                domain.NewModuleGitLabProjectID("gitlab-project-" + input.ModuleName),
		ModuleID:          module.ID,
		ModuleName:        module.Name,
		GitLabBaseURL:     input.GitLabBaseURL,
		GitLabProjectID:   input.GitLabProjectID,
		GitLabProjectPath: input.GitLabProjectPath,
		CreatedAt:         fake.now,
		UpdatedAt:         fake.now,
	}
	normalized, err := mapping.Normalized()
	if err != nil {
		return registry.LinkModuleGitLabProjectOutput{}, mapFakeGitLabValidationError(err)
	}
	if existing, exists := fake.gitLabProjectByBaseURLAndID(normalized.GitLabBaseURL, normalized.GitLabProjectID); exists && existing.ModuleID != module.ID {
		return registry.LinkModuleGitLabProjectOutput{}, registry.ErrGitLabProjectAlreadyLinked
	}
	if existing, exists := fake.gitLabProjects[module.ID.String()]; exists {
		normalized.ID = existing.ID
		normalized.CreatedAt = existing.CreatedAt
	}
	fake.gitLabProjects[module.ID.String()] = normalized
	return registry.LinkModuleGitLabProjectOutput{Mapping: normalized}, nil
}

func (fake *fakeRegistry) GetModuleGitLabProject(ctx context.Context, moduleName string) (registry.GetModuleGitLabProjectOutput, error) {
	module, exists := fake.modules[moduleName]
	if !exists {
		return registry.GetModuleGitLabProjectOutput{}, registry.ErrModuleNotFound
	}
	mapping, exists := fake.gitLabProjects[module.ID.String()]
	if !exists {
		return registry.GetModuleGitLabProjectOutput{}, registry.ErrModuleGitLabProjectNotFound
	}
	return registry.GetModuleGitLabProjectOutput{Mapping: mapping}, nil
}

func (fake *fakeRegistry) gitLabProjectByBaseURLAndID(baseURL string, projectID int64) (domain.ModuleGitLabProject, bool) {
	for _, mapping := range fake.gitLabProjects {
		if mapping.GitLabBaseURL == baseURL && mapping.GitLabProjectID == projectID {
			return mapping, true
		}
	}
	return domain.ModuleGitLabProject{}, false
}

func mapFakeGitLabValidationError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidModuleName):
		return registry.ErrInvalidModuleName
	case errors.Is(err, domain.ErrInvalidGitLabBaseURL):
		return registry.ErrInvalidGitLabBaseURL
	case errors.Is(err, domain.ErrInvalidGitLabProjectID):
		return registry.ErrInvalidGitLabProjectID
	case errors.Is(err, domain.ErrInvalidGitLabProjectPath):
		return registry.ErrInvalidGitLabProjectPath
	default:
		return err
	}
}

func (fake *fakeRegistry) PublishModuleVersion(ctx context.Context, req registry.PublishModuleVersionRequest) (registry.PublishModuleVersionResponse, error) {
	if fake.publishErr != nil {
		return registry.PublishModuleVersionResponse{}, fake.publishErr
	}
	module, exists := fake.modules[req.ModuleName]
	if !exists {
		return registry.PublishModuleVersionResponse{}, registry.ErrModuleNotFound
	}
	versionValue, err := domain.NewVersion(req.Version)
	if err != nil {
		return registry.PublishModuleVersionResponse{}, registry.ErrInvalidVersion
	}
	key := req.ModuleName + ":" + req.Version
	if _, exists := fake.versions[key]; exists {
		return registry.PublishModuleVersionResponse{}, registry.ErrModuleVersionAlreadyExists
	}
	body, err := io.ReadAll(req.Artifact)
	if err != nil {
		return registry.PublishModuleVersionResponse{}, err
	}
	sum := sha256.Sum256(body)
	checksum := hex.EncodeToString(sum[:])
	publishedAt := fake.now
	moduleVersion := domain.ModuleVersion{
		ID:          domain.NewModuleVersionID("version-" + req.Version),
		ModuleID:    module.ID,
		Version:     versionValue,
		Digest:      "sha256:" + checksum,
		Status:      domain.ModuleVersionStatusPublished,
		CreatedAt:   fake.now,
		PublishedAt: &publishedAt,
	}
	sourceArtifact := domain.Artifact{
		ID:              domain.NewArtifactID("artifact-" + req.Version),
		ModuleVersionID: moduleVersion.ID,
		Kind:            domain.ArtifactKindSourceArchive,
		StorageKey:      "artifact-key-" + req.Version,
		ChecksumSHA256:  checksum,
		SizeBytes:       int64(len(body)),
		CreatedAt:       fake.now,
	}
	bufImageArtifact := domain.Artifact{
		ID:              domain.NewArtifactID("buf-image-" + req.Version),
		ModuleVersionID: moduleVersion.ID,
		Kind:            domain.ArtifactKindBufImage,
		StorageKey:      "buf-image-key-" + req.Version,
		ChecksumSHA256:  "buf-image-checksum",
		SizeBytes:       int64(len("buf-image")),
		CreatedAt:       fake.now,
	}
	bufConfig := domain.BufConfigInfo{
		BufYAMLPresent:        true,
		BufLockPresent:        true,
		BufYAMLDigest:         "buf-yaml-digest",
		BufLockDigest:         "buf-lock-digest",
		ModulePaths:           []string{"proto"},
		Deps:                  []string{"buf.build/googleapis/googleapis"},
		LintEnabled:           fake.publishLint.Status != domain.BufLintStatusNotRun,
		BreakingConfigPresent: true,
	}
	metadata := testDescriptorMetadata()
	fake.versions[key] = moduleVersion
	fake.artifacts[key] = []domain.Artifact{sourceArtifact, bufImageArtifact}
	fake.objects[key] = body
	fake.bufConfigs[key] = bufConfig
	fake.lintResults[key] = fake.publishLint
	fake.metadata[key] = metadata
	return registry.PublishModuleVersionResponse{
		Version:          moduleVersion,
		SourceArtifact:   sourceArtifact,
		BufImageArtifact: bufImageArtifact,
		BufConfig:        bufConfig,
		LintResult:       fake.publishLint,
		MetadataSummary:  metadata.Summary(),
	}, nil
}

func (fake *fakeRegistry) DeprecateModuleVersion(ctx context.Context, input registry.DeprecateModuleVersionInput) (registry.DeprecateModuleVersionOutput, error) {
	fake.deprecateInput = input
	moduleVersion, err := fake.GetModuleVersion(ctx, input.ModuleName, input.Version)
	if err != nil {
		return registry.DeprecateModuleVersionOutput{}, err
	}
	deprecatedAt := fake.now
	moduleVersion.DeprecatedAt = &deprecatedAt
	moduleVersion.DeprecatedBy = input.Actor
	moduleVersion.DeprecationReason = strings.TrimSpace(input.Reason)
	fake.versions[input.ModuleName+":"+input.Version] = moduleVersion
	return registry.DeprecateModuleVersionOutput{Version: moduleVersion}, nil
}

func (fake *fakeRegistry) ListModuleVersions(ctx context.Context, moduleName string, limit int, offset int) ([]domain.ModuleVersion, error) {
	if _, exists := fake.modules[moduleName]; !exists {
		return nil, registry.ErrModuleNotFound
	}
	versions := make([]domain.ModuleVersion, 0)
	for key, version := range fake.versions {
		if strings.HasPrefix(key, moduleName+":") {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (fake *fakeRegistry) GetModuleVersion(ctx context.Context, moduleName string, version string) (domain.ModuleVersion, error) {
	if _, exists := fake.modules[moduleName]; !exists {
		return domain.ModuleVersion{}, registry.ErrModuleNotFound
	}
	moduleVersion, exists := fake.versions[moduleName+":"+version]
	if !exists {
		return domain.ModuleVersion{}, registry.ErrModuleNotFound
	}
	return moduleVersion, nil
}

func (fake *fakeRegistry) GetModuleVersionDetails(ctx context.Context, moduleName string, version string) (registry.ModuleVersionDetailsResponse, error) {
	moduleVersion, err := fake.GetModuleVersion(ctx, moduleName, version)
	if err != nil {
		return registry.ModuleVersionDetailsResponse{}, err
	}
	key := moduleName + ":" + version
	metadata := fake.metadata[key]
	return registry.ModuleVersionDetailsResponse{
		Version:         moduleVersion,
		Artifacts:       fake.artifacts[key],
		BufConfig:       fake.bufConfigs[key],
		LintResult:      fake.lintResults[key],
		MetadataSummary: metadata.Summary(),
	}, nil
}

func (fake *fakeRegistry) GetModuleVersionMetadata(ctx context.Context, moduleName string, version string) (domain.DescriptorMetadata, error) {
	if _, err := fake.GetModuleVersion(ctx, moduleName, version); err != nil {
		return domain.DescriptorMetadata{}, err
	}
	metadata, exists := fake.metadata[moduleName+":"+version]
	if !exists {
		return domain.DescriptorMetadata{}, registry.ErrModuleNotFound
	}
	return metadata, nil
}

func (fake *fakeRegistry) CheckBreaking(ctx context.Context, req registry.CheckBreakingRequest) (registry.CheckBreakingResponse, error) {
	if fake.checkErr != nil {
		return registry.CheckBreakingResponse{}, fake.checkErr
	}
	module, exists := fake.modules[req.ModuleName]
	if !exists {
		return registry.CheckBreakingResponse{}, registry.ErrModuleNotFound
	}
	against := req.Against
	if against == "latest" {
		var latest domain.ModuleVersion
		for key, version := range fake.versions {
			if strings.HasPrefix(key, req.ModuleName+":") && (latest.ID == "" || version.CreatedAt.After(latest.CreatedAt)) {
				latest = version
			}
		}
		if latest.ID == "" {
			return registry.CheckBreakingResponse{}, registry.ErrBaselineVersionNotFound
		}
		against = latest.Version.String()
	}
	baseline, exists := fake.versions[req.ModuleName+":"+against]
	if !exists {
		return registry.CheckBreakingResponse{}, registry.ErrBaselineVersionNotFound
	}
	targetRef := req.TargetRef
	if targetRef == "" {
		targetRef = "local"
	}
	changes := make([]domain.BreakingChange, 0, len(fake.breakingChanges))
	reportID := domain.NewBreakingReportID(fmt.Sprintf("report-%d", len(fake.reports)+1))
	for index, change := range fake.breakingChanges {
		change.ID = domain.NewBreakingChangeID(fmt.Sprintf("change-%d", index+1))
		change.ReportID = reportID
		change.CreatedAt = fake.now
		changes = append(changes, change)
	}
	report := domain.BreakingReport{
		ID:            reportID,
		ModuleID:      module.ID,
		ModuleName:    module.Name,
		BaseVersionID: baseline.ID,
		BaseVersion:   baseline.Version,
		TargetRef:     targetRef,
		Status:        fake.breakingStatus,
		ChangeCount:   len(changes),
		HumanSummary:  "ProtoRadar Breaking Change Report\nStatus: " + fake.breakingStatus.String(),
		CreatedAt:     fake.now,
	}
	response := registry.CheckBreakingResponse{Report: report, Changes: changes}
	fake.reports[report.ID.String()] = response
	return response, nil
}

func (fake *fakeRegistry) GetBreakingReport(ctx context.Context, reportID string) (registry.CheckBreakingResponse, error) {
	response, exists := fake.reports[reportID]
	if !exists {
		return registry.CheckBreakingResponse{}, registry.ErrBreakingReportNotFound
	}
	return response, nil
}

func (fake *fakeRegistry) ListBreakingReports(ctx context.Context, moduleName string, limit int, offset int) ([]domain.BreakingReport, error) {
	module, exists := fake.modules[moduleName]
	if !exists {
		return nil, registry.ErrModuleNotFound
	}
	reports := make([]domain.BreakingReport, 0)
	for _, response := range fake.reports {
		if response.Report.ModuleID == module.ID {
			reports = append(reports, response.Report)
		}
	}
	slices.SortFunc(reports, func(left domain.BreakingReport, right domain.BreakingReport) int {
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		if left.CreatedAt.Before(right.CreatedAt) {
			return 1
		}
		return strings.Compare(left.ID.String(), right.ID.String())
	})
	if limit > 0 && len(reports) > limit {
		reports = reports[:limit]
	}
	return reports, nil
}

func (fake *fakeRegistry) GetModuleDependencyGraph(ctx context.Context, moduleName string) (registry.ModuleDependencyGraphResponse, error) {
	if fake.dependencyErr != nil {
		return registry.ModuleDependencyGraphResponse{}, fake.dependencyErr
	}
	module, exists := fake.modules[moduleName]
	if !exists {
		return registry.ModuleDependencyGraphResponse{}, registry.ErrModuleNotFound
	}
	upstream := make([]domain.ModuleDependency, 0)
	downstream := make([]domain.ModuleDependency, 0)
	for _, dependency := range fake.dependencies {
		if dependency.ConsumerModuleID == module.ID {
			upstream = append(upstream, dependency)
		}
		if dependency.ProviderModuleID == module.ID {
			downstream = append(downstream, dependency)
		}
	}
	unresolved := make([]domain.UnresolvedProtoDependency, 0)
	for _, dependency := range fake.unresolved {
		if dependency.ModuleID == module.ID {
			unresolved = append(unresolved, dependency)
		}
	}
	return registry.ModuleDependencyGraphResponse{Module: module, Upstream: upstream, Downstream: downstream, Unresolved: unresolved}, nil
}

func (fake *fakeRegistry) ListAffectedModules(ctx context.Context, moduleName string) (registry.AffectedModulesResponse, error) {
	if fake.dependencyErr != nil {
		return registry.AffectedModulesResponse{}, fake.dependencyErr
	}
	module, exists := fake.modules[moduleName]
	if !exists {
		return registry.AffectedModulesResponse{}, registry.ErrModuleNotFound
	}
	return registry.AffectedModulesResponse{Module: module, AffectedModules: fake.affectedModules(module.ID)}, nil
}

func (fake *fakeRegistry) GetBreakingReportAffectedModules(ctx context.Context, reportID string) (registry.BreakingReportAffectedModulesResponse, error) {
	if fake.dependencyErr != nil {
		return registry.BreakingReportAffectedModulesResponse{}, fake.dependencyErr
	}
	response, exists := fake.reports[reportID]
	if !exists {
		return registry.BreakingReportAffectedModulesResponse{}, registry.ErrBreakingReportNotFound
	}
	return registry.BreakingReportAffectedModulesResponse{
		Report:          response.Report,
		AffectedModules: fake.affectedModules(response.Report.ModuleID),
	}, nil
}

func (fake *fakeRegistry) affectedModules(providerModuleID domain.ModuleID) []domain.AffectedModule {
	type item struct {
		name    domain.ModuleName
		version domain.Version
		sources map[domain.DependencySource]struct{}
		reasons map[domain.DependencyResolutionReason]struct{}
	}
	groups := map[string]*item{}
	for _, dependency := range fake.dependencies {
		if dependency.ProviderModuleID != providerModuleID {
			continue
		}
		key := dependency.ConsumerModuleID.String()
		group, exists := groups[key]
		if !exists {
			group = &item{
				name:    dependency.ConsumerModuleName,
				version: dependency.ConsumerVersion,
				sources: map[domain.DependencySource]struct{}{},
				reasons: map[domain.DependencyResolutionReason]struct{}{},
			}
			groups[key] = group
		}
		group.sources[dependency.Source] = struct{}{}
		group.reasons[dependency.Reason] = struct{}{}
	}
	items := make([]domain.AffectedModule, 0, len(groups))
	for _, group := range groups {
		sources := make([]domain.DependencySource, 0, len(group.sources))
		for source := range group.sources {
			sources = append(sources, source)
		}
		reasons := make([]domain.DependencyResolutionReason, 0, len(group.reasons))
		for reason := range group.reasons {
			reasons = append(reasons, reason)
		}
		items = append(items, domain.AffectedModule{
			ModuleName:        group.name,
			LatestVersion:     group.version,
			DependencySources: sources,
			Reasons:           reasons,
		})
	}
	return items
}

func (fake *fakeRegistry) DownloadArtifact(ctx context.Context, moduleName string, version string) (storage.ArtifactObject, domain.Artifact, error) {
	key := moduleName + ":" + version
	artifacts, exists := fake.artifacts[key]
	if !exists || len(artifacts) == 0 {
		return storage.ArtifactObject{}, domain.Artifact{}, registry.ErrModuleNotFound
	}
	artifact := artifacts[0]
	body := fake.objects[key]
	reader := fake.objectReaders[key]
	if reader == nil {
		reader = io.NopCloser(bytes.NewReader(body))
	}
	return storage.ArtifactObject{
		Key:         artifact.StorageKey,
		ContentType: "application/gzip",
		SizeBytes:   int64(len(body)),
		Body:        reader,
	}, artifact, nil
}

type failingReadCloser struct {
	chunks [][]byte
	err    error
	closed bool
}

func (reader *failingReadCloser) Read(p []byte) (int, error) {
	if len(reader.chunks) == 0 {
		return 0, reader.err
	}
	chunk := reader.chunks[0]
	reader.chunks = reader.chunks[1:]
	return copy(p, chunk), nil
}

func (reader *failingReadCloser) Close() error {
	reader.closed = true
	return nil
}

func (fake *fakeRegistry) ReportRuntimeInventory(ctx context.Context, input runtimeinventory.ReportRuntimeInventoryInput) (runtimeinventory.ReportRuntimeInventoryOutput, error) {
	if fake.runtimeErr != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, fake.runtimeErr
	}
	serviceName, err := domain.NewRuntimeServiceName(input.ServiceName)
	if err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	environment, err := domain.NewRuntimeEnvironment(input.Environment)
	if err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	if err := domain.ValidateRuntimeGitCommit(input.GitCommit); err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	if err := domain.ValidateRuntimeBuildVersion(input.BuildVersion); err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	if len(input.Modules) == 0 {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, runtimeinventory.ErrRuntimeModulesRequired
	}
	service := fake.runtimeServices[serviceName.String()]
	if service.ID == "" {
		service = domain.RuntimeService{
			ID:        domain.NewRuntimeServiceID("runtime-service-" + serviceName.String()),
			Name:      serviceName,
			CreatedAt: fake.now,
			UpdatedAt: fake.now,
		}
		fake.runtimeServices[serviceName.String()] = service
	}
	deployment := domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID(fmt.Sprintf("runtime-deployment-%d", len(fake.runtimeDeployments)+1)),
		ServiceID:    service.ID,
		ServiceName:  service.Name,
		Environment:  environment,
		GitCommit:    input.GitCommit,
		BuildVersion: input.BuildVersion,
		ReportedAt:   fake.now,
		CreatedAt:    fake.now,
	}
	fake.runtimeDeployments = append(fake.runtimeDeployments, deployment)

	seen := map[string]struct{}{}
	outputUsages := make([]runtimeinventory.RuntimeModuleUsageOutput, 0, len(input.Modules))
	for _, reported := range input.Modules {
		moduleName, err := domain.NewModuleName(reported.Module)
		if err != nil {
			return runtimeinventory.ReportRuntimeInventoryOutput{}, err
		}
		version, err := domain.NewVersion(reported.Version)
		if err != nil {
			return runtimeinventory.ReportRuntimeInventoryOutput{}, err
		}
		key := moduleName.String() + "\x00" + version.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		usage, output := fake.runtimeUsageForReport(deployment.ID, moduleName, version)
		fake.runtimeUsages = append(fake.runtimeUsages, usage)
		outputUsages = append(outputUsages, output)
	}
	if len(outputUsages) == 0 {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, runtimeinventory.ErrRuntimeModulesRequired
	}
	return runtimeinventory.ReportRuntimeInventoryOutput{
		DeploymentID: deployment.ID.String(),
		ServiceName:  serviceName.String(),
		Environment:  environment.String(),
		GitCommit:    input.GitCommit,
		BuildVersion: input.BuildVersion,
		ReportedAt:   fake.now,
		Usages:       outputUsages,
	}, nil
}

func (fake *fakeRegistry) runtimeUsageForReport(deploymentID domain.RuntimeDeploymentID, moduleName domain.ModuleName, version domain.Version) (domain.RuntimeModuleUsage, runtimeinventory.RuntimeModuleUsageOutput) {
	status := domain.RuntimeDriftStatusUnknownVersion
	reason := "module_not_found"
	latestVersion := ""
	var moduleID *domain.ModuleID
	var moduleVersionID *domain.ModuleVersionID
	if module, exists := fake.modules[moduleName.String()]; exists {
		moduleID = &module.ID
		if reported, exists := fake.versions[moduleName.String()+":"+version.String()]; exists {
			moduleVersionID = &reported.ID
			latest := fake.latestVersion(moduleName.String())
			if latest.ID != "" {
				latestVersion = latest.Version.String()
			}
			if reported.IsDeprecated() {
				status = domain.RuntimeDriftStatusDeprecatedVersion
				reason = "deprecated_version"
			} else if latest.ID == reported.ID {
				status = domain.RuntimeDriftStatusUpToDate
				reason = "up_to_date"
			} else {
				status = domain.RuntimeDriftStatusBehindLatest
				reason = "behind_latest: latest version is " + latestVersion
			}
		} else {
			reason = "version_not_found"
		}
	}
	usage := domain.RuntimeModuleUsage{
		ID:              domain.NewRuntimeModuleUsageID(fmt.Sprintf("runtime-usage-%d", len(fake.runtimeUsages)+1)),
		DeploymentID:    deploymentID,
		ModuleID:        moduleID,
		ModuleName:      moduleName,
		ModuleVersionID: moduleVersionID,
		Version:         version,
		DriftStatus:     status,
		DriftReason:     reason,
		CreatedAt:       fake.now,
	}
	if latestVersion != "" {
		latest, _ := domain.NewVersion(latestVersion)
		usage.LatestVersion = &latest
	}
	return usage, runtimeinventory.RuntimeModuleUsageOutput{
		Module:        moduleName.String(),
		Version:       version.String(),
		LatestVersion: latestVersion,
		DriftStatus:   status,
		DriftReason:   reason,
	}
}

func (fake *fakeRegistry) latestVersion(moduleName string) domain.ModuleVersion {
	var latest domain.ModuleVersion
	for key, version := range fake.versions {
		if strings.HasPrefix(key, moduleName+":") && (latest.ID == "" || version.CreatedAt.After(latest.CreatedAt)) {
			latest = version
		}
	}
	return latest
}

func (fake *fakeRegistry) ListRuntimeServices(ctx context.Context, limit int, offset int) ([]domain.RuntimeServiceSummary, error) {
	if fake.runtimeErr != nil {
		return nil, fake.runtimeErr
	}
	summaries := make([]domain.RuntimeServiceSummary, 0, len(fake.runtimeServices))
	for _, service := range fake.runtimeServices {
		summary := domain.RuntimeServiceSummary{Service: service}
		environmentSet := map[string]domain.RuntimeEnvironment{}
		for _, deployment := range fake.runtimeDeployments {
			if deployment.ServiceID != service.ID {
				continue
			}
			summary.DeploymentCount++
			environmentSet[deployment.Environment.String()] = deployment.Environment
			if summary.LatestReportedAt == nil || deployment.ReportedAt.After(*summary.LatestReportedAt) {
				value := deployment.ReportedAt
				summary.LatestReportedAt = &value
			}
			for _, usage := range fake.runtimeUsages {
				if usage.DeploymentID == deployment.ID {
					addRuntimeSummaryDrift(&summary, usage.DriftStatus)
				}
			}
		}
		for _, environment := range environmentSet {
			summary.Environments = append(summary.Environments, environment)
		}
		summary.EnvironmentCount = len(summary.Environments)
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func addRuntimeSummaryDrift(summary *domain.RuntimeServiceSummary, status domain.RuntimeDriftStatus) {
	switch status {
	case domain.RuntimeDriftStatusUpToDate:
		summary.UpToDateCount++
	case domain.RuntimeDriftStatusBehindLatest:
		summary.BehindLatestCount++
	case domain.RuntimeDriftStatusUnknownVersion:
		summary.UnknownVersionCount++
	case domain.RuntimeDriftStatusDeprecatedVersion:
		summary.DeprecatedCount++
	}
}

func (fake *fakeRegistry) GetRuntimeServiceDetails(ctx context.Context, serviceNameValue string) (domain.RuntimeServiceDetails, error) {
	if fake.runtimeErr != nil {
		return domain.RuntimeServiceDetails{}, fake.runtimeErr
	}
	service, exists := fake.runtimeServices[serviceNameValue]
	if !exists {
		return domain.RuntimeServiceDetails{}, domain.ErrNotFound
	}
	deployments := make([]domain.RuntimeDeployment, 0)
	usages := make([]domain.RuntimeModuleUsage, 0)
	for _, deployment := range fake.runtimeDeployments {
		if deployment.ServiceID != service.ID {
			continue
		}
		deployments = append(deployments, deployment)
		for _, usage := range fake.runtimeUsages {
			if usage.DeploymentID == deployment.ID {
				usages = append(usages, usage)
			}
		}
	}
	return domain.RuntimeServiceDetails{Service: service, Deployments: deployments, Usages: usages}, nil
}

func (fake *fakeRegistry) GetEnvironmentInventory(ctx context.Context, environmentValue string, limit int, offset int) (domain.RuntimeEnvironmentInventory, error) {
	if fake.runtimeErr != nil {
		return domain.RuntimeEnvironmentInventory{}, fake.runtimeErr
	}
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		return domain.RuntimeEnvironmentInventory{}, err
	}
	deployments := make([]domain.RuntimeDeployment, 0)
	usages := make([]domain.RuntimeModuleUsage, 0)
	for _, deployment := range fake.runtimeDeployments {
		if deployment.Environment != environment {
			continue
		}
		deployments = append(deployments, deployment)
		for _, usage := range fake.runtimeUsages {
			if usage.DeploymentID == deployment.ID {
				usages = append(usages, usage)
			}
		}
	}
	return domain.RuntimeEnvironmentInventory{Environment: environment, Deployments: deployments, Usages: usages}, nil
}

func (fake *fakeRegistry) GetModuleRuntimeUsages(ctx context.Context, moduleNameValue string, limit int, offset int) ([]domain.ModuleRuntimeUsage, error) {
	if fake.runtimeErr != nil {
		return nil, fake.runtimeErr
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ModuleRuntimeUsage, 0)
	for _, usage := range fake.runtimeUsages {
		if usage.ModuleName != moduleName {
			continue
		}
		for _, deployment := range fake.runtimeDeployments {
			if deployment.ID == usage.DeploymentID {
				items = append(items, domain.ModuleRuntimeUsage{
					ServiceName:  deployment.ServiceName,
					Environment:  deployment.Environment,
					DeploymentID: deployment.ID,
					ModuleName:   usage.ModuleName,
					Version:      usage.Version,
					GitCommit:    deployment.GitCommit,
					BuildVersion: deployment.BuildVersion,
					ReportedAt:   deployment.ReportedAt,
					DriftStatus:  usage.DriftStatus,
					DriftReason:  usage.DriftReason,
				})
			}
		}
	}
	return items, nil
}

func (fake *fakeRegistry) GetBreakingReportRuntimeImpact(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.RuntimeImpact, error) {
	if fake.runtimeErr != nil {
		return nil, fake.runtimeErr
	}
	report, exists := fake.reports[reportID.String()]
	if !exists {
		return nil, registry.ErrBreakingReportNotFound
	}
	items := make([]domain.RuntimeImpact, 0)
	for _, usage := range fake.runtimeUsages {
		if usage.ModuleVersionID == nil || *usage.ModuleVersionID != report.Report.BaseVersionID {
			continue
		}
		for _, deployment := range fake.runtimeDeployments {
			if deployment.ID == usage.DeploymentID {
				items = append(items, domain.RuntimeImpact{
					ServiceName:  deployment.ServiceName,
					Environment:  deployment.Environment,
					UsedModule:   usage.ModuleName,
					UsedVersion:  usage.Version,
					GitCommit:    deployment.GitCommit,
					BuildVersion: deployment.BuildVersion,
					ReportedAt:   deployment.ReportedAt,
					ImpactStatus: domain.RuntimeImpactStatusPotentiallyAffectedByBreakingChange,
					Reason:       "exact runtime module version matches breaking report base version",
					DriftStatus:  usage.DriftStatus,
					DriftReason:  usage.DriftReason,
				})
			}
		}
	}
	return items, nil
}

func (fake *fakeRegistry) seedRuntimeServiceSummary(t *testing.T, serviceNameValue string, environmentValue string) {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment(environmentValue)
	fake.runtimeServices[serviceName.String()] = domain.RuntimeService{
		ID:        domain.NewRuntimeServiceID("runtime-service-" + serviceName.String()),
		Name:      serviceName,
		CreatedAt: fake.now,
		UpdatedAt: fake.now,
	}
	fake.runtimeDeployments = append(fake.runtimeDeployments, domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID("runtime-deployment-summary"),
		ServiceID:    fake.runtimeServices[serviceName.String()].ID,
		ServiceName:  serviceName,
		Environment:  environment,
		GitCommit:    "abc123",
		BuildVersion: "build-1",
		ReportedAt:   fake.now,
		CreatedAt:    fake.now,
	})
}

func (fake *fakeRegistry) seedRuntimeDeployment(t *testing.T, serviceNameValue string, environmentValue string, moduleNameValue string, versionValue string) {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment(environmentValue)
	moduleName, _ := domain.NewModuleName(moduleNameValue)
	version, _ := domain.NewVersion(versionValue)
	service := fake.runtimeServices[serviceName.String()]
	if service.ID == "" {
		service = domain.RuntimeService{
			ID:        domain.NewRuntimeServiceID("runtime-service-" + serviceName.String()),
			Name:      serviceName,
			CreatedAt: fake.now,
			UpdatedAt: fake.now,
		}
		fake.runtimeServices[serviceName.String()] = service
	}
	deployment := domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID(fmt.Sprintf("runtime-deployment-%d", len(fake.runtimeDeployments)+1)),
		ServiceID:    service.ID,
		ServiceName:  serviceName,
		Environment:  environment,
		GitCommit:    "abc123",
		BuildVersion: "build-1",
		ReportedAt:   fake.now,
		CreatedAt:    fake.now,
	}
	fake.runtimeDeployments = append(fake.runtimeDeployments, deployment)
	usage, _ := fake.runtimeUsageForReport(deployment.ID, moduleName, version)
	fake.runtimeUsages = append(fake.runtimeUsages, usage)
}

func testDescriptorMetadata() domain.DescriptorMetadata {
	return domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{
			Path:        "user/v1/user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Imports: []domain.ProtoImport{
				{Path: "google/protobuf/timestamp.proto", Public: true},
			},
			Services: []domain.ProtoService{
				{
					Name:     "UserService",
					FullName: "user.v1.UserService",
					Methods: []domain.ProtoMethod{
						{Name: "GetUser", InputType: ".user.v1.GetUserRequest", OutputType: ".user.v1.User"},
					},
				},
			},
			Messages: []domain.ProtoMessage{
				{
					Name:     "User",
					FullName: "user.v1.User",
					Fields: []domain.ProtoField{
						{Name: "id", Number: 1, Type: "string", JSONName: "id"},
					},
				},
			},
			Enums: []domain.ProtoEnum{
				{
					Name:     "Role",
					FullName: "user.v1.Role",
					Values: []domain.ProtoEnumValue{
						{Name: "ROLE_UNSPECIFIED", Number: 0},
					},
				},
			},
		},
	}}
}

func (fake *fakeRegistry) CreateAPIToken(ctx context.Context, req registry.CreateAPITokenRequest) (registry.CreateAPITokenResponse, error) {
	token := domain.APIToken{
		ID:        domain.NewAPITokenID("token-1"),
		Name:      req.Name,
		TokenHash: "stored-hash",
		CreatedAt: fake.now,
		ExpiresAt: req.ExpiresAt,
	}
	return registry.CreateAPITokenResponse{
		Token:    token,
		RawToken: "raw-token",
	}, nil
}
