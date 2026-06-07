package uiquery

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/edition"
	buildversion "github.com/alryzden/ProtoRadar/internal/version"
)

func TestListModuleOverviewsIncludesLatestVersionAndVersionCount(t *testing.T) {
	svc, _ := newFixture()

	items, err := svc.ListModuleOverviews(context.Background(), ListModuleOverviewsInput{})
	if err != nil {
		t.Fatalf("list modules: %v", err)
	}

	user := findModuleOverview(t, items, "user-api")
	if user.LatestVersion == nil || user.LatestVersion.Version != "v2.0.0" {
		t.Fatalf("latest version = %#v", user.LatestVersion)
	}
	if user.VersionCount != 2 {
		t.Fatalf("version count = %d", user.VersionCount)
	}
	if user.BreakingReportCount != 2 {
		t.Fatalf("report count = %d", user.BreakingReportCount)
	}
	if user.LastBreakingStatus != "breaking" {
		t.Fatalf("last breaking status = %q", user.LastBreakingStatus)
	}
}

func TestGetEditionReturnsCommunityCapabilities(t *testing.T) {
	_, fixture := newFixture()
	svc := NewService(fixture.modules, fixture.gitlab, fixture.versions, fixture.artifacts, fixture.bufConfigs, fixture.metadata, fixture.reports, fixture.dependencies, nil, GovernanceRepositories{}, edition.NewCommunityCapabilityChecker(), buildversion.BuildInfo{Version: "v1.2.0", Commit: "abc123", BuildDate: "2026-06-05T12:00:00Z"})

	details, err := svc.GetEdition(context.Background(), GetEditionInput{})
	if err != nil {
		t.Fatalf("get edition: %v", err)
	}
	if details.Edition != edition.NameCommunity || details.Version != "v1.2.0" || details.Commit != "abc123" {
		t.Fatalf("edition = %#v", details)
	}
	statuses := map[string]bool{}
	for _, status := range details.Capabilities {
		statuses[status.Name] = status.Enabled
	}
	if !statuses[edition.CapabilityRegistry.String()] {
		t.Fatalf("registry disabled: %#v", statuses)
	}
	if statuses[edition.CapabilityOIDCAuth.String()] {
		t.Fatalf("oidc enabled: %#v", statuses)
	}
}

func TestListModuleOverviewsAppliesQueryFilter(t *testing.T) {
	svc, _ := newFixture()

	items, err := svc.ListModuleOverviews(context.Background(), ListModuleOverviewsInput{Query: "billing"})
	if err != nil {
		t.Fatalf("list modules: %v", err)
	}
	if len(items) != 1 || items[0].Module.Name != "billing-api" {
		t.Fatalf("items = %#v", items)
	}
}

func TestGetModuleOverviewReturnsVersionsAndRecentReports(t *testing.T) {
	svc, _ := newFixture()

	overview, err := svc.GetModuleOverview(context.Background(), GetModuleOverviewInput{Module: "user-api"})
	if err != nil {
		t.Fatalf("get module: %v", err)
	}
	if len(overview.Versions) != 2 {
		t.Fatalf("versions = %#v", overview.Versions)
	}
	if overview.Versions[0].Version != "v2.0.0" || len(overview.Versions[0].Artifacts) != 1 {
		t.Fatalf("version details = %#v", overview.Versions[0])
	}
	if overview.Versions[0].LintStatus != "passed" {
		t.Fatalf("lint status = %q", overview.Versions[0].LintStatus)
	}
	if overview.Versions[0].MetadataSummary.Files != 2 {
		t.Fatalf("metadata summary = %#v", overview.Versions[0].MetadataSummary)
	}
	if len(overview.RecentReports) != 2 || overview.RecentReports[0].ID != "report-2" {
		t.Fatalf("reports = %#v", overview.RecentReports)
	}
}

func TestGetModuleOverviewReturnsGitLabMappingWhenAvailable(t *testing.T) {
	svc, _ := newFixture()

	overview, err := svc.GetModuleOverview(context.Background(), GetModuleOverviewInput{Module: "user-api"})
	if err != nil {
		t.Fatalf("get module: %v", err)
	}
	if overview.GitLabProject == nil {
		t.Fatalf("gitlab mapping missing")
	}
	if overview.GitLabProject.ProjectID != 12345 || overview.GitLabProject.ProjectPath != "platform/user-api" {
		t.Fatalf("gitlab mapping = %#v", overview.GitLabProject)
	}
}

func TestGetVersionOverviewReturnsArtifactsAndDescriptorMetadata(t *testing.T) {
	svc, _ := newFixture()

	overview, err := svc.GetVersionOverview(context.Background(), GetVersionOverviewInput{Module: "user-api", Version: "v2.0.0"})
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if overview.Module.Name != "user-api" || overview.Version.Version != "v2.0.0" {
		t.Fatalf("overview = %#v", overview)
	}
	if len(overview.Artifacts) != 1 || overview.Artifacts[0].Kind != "buf_image" || overview.Artifacts[0].ChecksumSHA256 != "sha-v2" {
		t.Fatalf("artifacts = %#v", overview.Artifacts)
	}
	if !overview.BufConfig.ConfigPresent || overview.BufConfig.LintStatus != "passed" {
		t.Fatalf("buf config = %#v", overview.BufConfig)
	}
	if overview.MetadataCounts.Files != 2 || len(overview.Metadata.Files) != 1 {
		t.Fatalf("metadata = %#v / %#v", overview.MetadataCounts, overview.Metadata)
	}
	if len(overview.RelatedReports) != 1 || overview.RelatedReports[0].ID != "report-2" {
		t.Fatalf("related reports = %#v", overview.RelatedReports)
	}
}

func TestGetVersionOverviewReturnsNotFoundForMissingModuleOrVersion(t *testing.T) {
	svc, _ := newFixture()

	if _, err := svc.GetVersionOverview(context.Background(), GetVersionOverviewInput{Module: "missing", Version: "v1.0.0"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing module error = %v", err)
	}
	if _, err := svc.GetVersionOverview(context.Background(), GetVersionOverviewInput{Module: "user-api", Version: "v9.0.0"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing version error = %v", err)
	}
}

func TestListBreakingReportOverviewsFiltersByModuleAndStatus(t *testing.T) {
	svc, _ := newFixture()

	items, err := svc.ListBreakingReportOverviews(context.Background(), ListBreakingReportOverviewsInput{Module: "user-api", Status: "passed"})
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(items) != 1 || items[0].ID != "report-1" {
		t.Fatalf("items = %#v", items)
	}
}

func TestListBreakingReportOverviewsAppliesQueryAndSorts(t *testing.T) {
	svc, _ := newFixture()

	items, err := svc.ListBreakingReportOverviews(context.Background(), ListBreakingReportOverviewsInput{Query: "feature/remove-name"})
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(items) != 1 || items[0].ID != "report-2" {
		t.Fatalf("items = %#v", items)
	}
}

func TestGetBreakingReportDetailsReturnsReportAndChanges(t *testing.T) {
	svc, _ := newFixture()

	details, err := svc.GetBreakingReportDetails(context.Background(), GetBreakingReportDetailsInput{ReportID: "report-2"})
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if details.Report.Module != "user-api" || details.Report.BaseVersion != "v2.0.0" {
		t.Fatalf("report = %#v", details.Report)
	}
	if details.Summary != "breaking summary" {
		t.Fatalf("summary = %q", details.Summary)
	}
	if len(details.Changes) != 1 || details.Changes[0].RuleID != "FIELD_NO_DELETE" {
		t.Fatalf("changes = %#v", details.Changes)
	}
	if len(details.AffectedModules) != 1 || details.AffectedModules[0].Module != "billing-api" {
		t.Fatalf("affected modules = %#v", details.AffectedModules)
	}
}

func TestGetModuleDependencyGraphReturnsDownstreamUpstreamAndUnresolved(t *testing.T) {
	svc, _ := newFixture()

	graph, err := svc.GetModuleDependencyGraph(context.Background(), GetModuleDependencyGraphInput{Module: "user-api"})
	if err != nil {
		t.Fatalf("get graph: %v", err)
	}
	if graph.Module.Name != "user-api" {
		t.Fatalf("module = %#v", graph.Module)
	}
	if len(graph.Downstream) != 1 || graph.Downstream[0].Module != "billing-api" || graph.Downstream[0].Version != "v1.0.0" {
		t.Fatalf("downstream = %#v", graph.Downstream)
	}
	if len(graph.Upstream) != 1 || graph.Upstream[0].Module != "common-api" || graph.Upstream[0].Version != "v1.0.0" {
		t.Fatalf("upstream = %#v", graph.Upstream)
	}
	if len(graph.Unresolved) != 1 || graph.Unresolved[0].ImportPath != "missing/v1/missing.proto" {
		t.Fatalf("unresolved = %#v", graph.Unresolved)
	}
}

func TestEmptyStatesReturnEmptySlices(t *testing.T) {
	fixture := emptyFixture()
	svc := fixture.service()

	items, err := svc.ListModuleOverviews(context.Background(), ListModuleOverviewsInput{})
	if err != nil {
		t.Fatalf("list modules: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v", items)
	}

	fixture.modules.items = append(fixture.modules.items, module("empty-api", "Empty", "", at(1)))
	overview, err := svc.GetModuleOverview(context.Background(), GetModuleOverviewInput{Module: "empty-api"})
	if err != nil {
		t.Fatalf("get module: %v", err)
	}
	if overview.Versions == nil || overview.RecentReports == nil {
		t.Fatalf("empty slices should be non-nil: %#v", overview)
	}
}

func newFixture() (*Service, *fixture) {
	fixture := emptyFixture()
	user := module("user-api", "User service protobuf contracts", "https://gitlab.example.com/platform/user-api", at(1))
	billing := module("billing-api", "Billing service protobuf contracts", "https://gitlab.example.com/platform/billing-api", at(2))
	common := module("common-api", "Common contracts", "https://gitlab.example.com/platform/common-api", at(2))
	fixture.modules.items = []domain.Module{user, billing, common}
	fixture.gitlab.items[user.ID] = domain.ModuleGitLabProject{
		ID:                domain.NewModuleGitLabProjectID("mapping-user"),
		ModuleID:          user.ID,
		ModuleName:        user.Name,
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   12345,
		GitLabProjectPath: "platform/user-api",
		UpdatedAt:         at(4),
	}

	v1 := moduleVersion(user.ID, "user-v1", "v1.0.0", "digest-v1", at(3))
	v2 := moduleVersion(user.ID, "user-v2", "v2.0.0", "digest-v2", at(5))
	billingV1 := moduleVersion(billing.ID, "billing-v1", "v1.0.0", "digest-billing-v1", at(4))
	commonV1 := moduleVersion(common.ID, "common-v1", "v1.0.0", "digest-common-v1", at(4))
	fixture.versions.items = []domain.ModuleVersion{v2, v1, billingV1, commonV1}
	fixture.artifacts.items[v2.ID] = []domain.Artifact{{ID: domain.NewArtifactID("artifact-v2"), ModuleVersionID: v2.ID, Kind: domain.ArtifactKindBufImage, ChecksumSHA256: "sha-v2", SizeBytes: 42, CreatedAt: at(5)}}
	fixture.bufConfigs.items[v2.ID] = domain.BufConfigInfo{BufYAMLPresent: true, BufYAMLDigest: "buf-yaml-v2", LintEnabled: true, BreakingConfigPresent: true}
	fixture.metadata.summary[v2.ID] = domain.DescriptorMetadataSummary{FileCount: 2, PackageCount: 1, ImportCount: 3, ServiceCount: 1, MethodCount: 2, MessageCount: 4, FieldCount: 8, EnumCount: 1, EnumValueCount: 2}
	fixture.metadata.items[v2.ID] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "proto/user/v1/user.proto", PackageName: "user.v1", Syntax: "proto3"}}}

	fixture.reports.items[user.ID] = []domain.BreakingReport{
		report("report-2", user, v2, domain.BreakingReportStatusBreaking, "feature/remove-name", 1, "breaking summary", at(7)),
		report("report-1", user, v1, domain.BreakingReportStatusPassed, "feature/add-field", 0, "passed summary", at(6)),
	}
	fixture.reports.items[billing.ID] = []domain.BreakingReport{
		report("report-3", billing, billingV1, domain.BreakingReportStatusPassed, "feature/billing", 0, "billing summary", at(8)),
	}
	fixture.reports.changes[domain.NewBreakingReportID("report-2")] = []domain.BreakingChange{{ID: domain.NewBreakingChangeID("change-1"), ReportID: domain.NewBreakingReportID("report-2"), FilePath: "proto/user/v1/user.proto", Symbol: "user.v1.User.name", RuleID: "FIELD_NO_DELETE", Message: "field deleted", Severity: "error", CreatedAt: at(7)}}
	fixture.dependencies.dependencies = []domain.ModuleDependency{
		dependency("dep-billing-user", billing, billingV1, user, v2, domain.DependencySourceImport, domain.DependencyResolutionReasonImportPath),
		dependency("dep-user-common", user, v2, common, commonV1, domain.DependencySourceTypeReference, domain.DependencyResolutionReasonSymbol),
	}
	fixture.dependencies.unresolved = []domain.UnresolvedProtoDependency{{
		ID:               domain.NewUnresolvedProtoDependencyID("unresolved-user"),
		ModuleID:         user.ID,
		ModuleName:       user.Name,
		ModuleVersionID:  v2.ID,
		Version:          v2.Version,
		Source:           domain.DependencySourceImport,
		ImportPath:       "missing/v1/missing.proto",
		ReferencedSymbol: "missing.v1.Missing",
		Reason:           domain.UnresolvedDependencyReasonProviderNotFound,
		CreatedAt:        at(5),
	}}
	return fixture.service(), fixture
}

func emptyFixture() *fixture {
	return &fixture{
		modules:      &fakeModuleRepo{},
		gitlab:       &fakeGitLabRepo{items: map[domain.ModuleID]domain.ModuleGitLabProject{}},
		versions:     &fakeVersionRepo{},
		artifacts:    &fakeArtifactRepo{items: map[domain.ModuleVersionID][]domain.Artifact{}},
		bufConfigs:   &fakeBufConfigRepo{items: map[domain.ModuleVersionID]domain.BufConfigInfo{}},
		metadata:     &fakeMetadataRepo{items: map[domain.ModuleVersionID]domain.DescriptorMetadata{}, summary: map[domain.ModuleVersionID]domain.DescriptorMetadataSummary{}},
		reports:      &fakeReportRepo{items: map[domain.ModuleID][]domain.BreakingReport{}, changes: map[domain.BreakingReportID][]domain.BreakingChange{}},
		dependencies: &fakeDependencyRepo{},
	}
}

type fixture struct {
	modules      *fakeModuleRepo
	gitlab       *fakeGitLabRepo
	versions     *fakeVersionRepo
	artifacts    *fakeArtifactRepo
	bufConfigs   *fakeBufConfigRepo
	metadata     *fakeMetadataRepo
	reports      *fakeReportRepo
	dependencies *fakeDependencyRepo
}

func (fixture *fixture) service() *Service {
	return NewService(fixture.modules, fixture.gitlab, fixture.versions, fixture.artifacts, fixture.bufConfigs, fixture.metadata, fixture.reports, fixture.dependencies, nil, GovernanceRepositories{}, nil, buildversion.BuildInfo{})
}

func module(name string, description string, repositoryURL string, updatedAt time.Time) domain.Module {
	moduleName, err := domain.NewModuleName(name)
	if err != nil {
		panic(err)
	}
	return domain.Module{ID: domain.NewModuleID(name + "-id"), Name: moduleName, Description: description, RepositoryURL: repositoryURL, CreatedAt: updatedAt.Add(-time.Hour), UpdatedAt: updatedAt}
}

func moduleVersion(moduleID domain.ModuleID, id string, value string, digest string, createdAt time.Time) domain.ModuleVersion {
	parsed, err := domain.NewVersion(value)
	if err != nil {
		panic(err)
	}
	publishedAt := createdAt
	return domain.ModuleVersion{ID: domain.NewModuleVersionID(id), ModuleID: moduleID, Version: parsed, Status: domain.ModuleVersionStatusPublished, Digest: digest, CreatedAt: createdAt, PublishedAt: &publishedAt}
}

func report(id string, module domain.Module, version domain.ModuleVersion, status domain.BreakingReportStatus, targetRef string, changeCount int, summary string, createdAt time.Time) domain.BreakingReport {
	return domain.BreakingReport{ID: domain.NewBreakingReportID(id), ModuleID: module.ID, ModuleName: module.Name, BaseVersionID: version.ID, BaseVersion: version.Version, TargetRef: targetRef, Status: status, ChangeCount: changeCount, HumanSummary: summary, CreatedAt: createdAt}
}

func dependency(id string, consumer domain.Module, consumerVersion domain.ModuleVersion, provider domain.Module, providerVersion domain.ModuleVersion, source domain.DependencySource, reason domain.DependencyResolutionReason) domain.ModuleDependency {
	return domain.ModuleDependency{
		ID:                      domain.NewModuleDependencyID(id),
		ConsumerModuleID:        consumer.ID,
		ConsumerModuleName:      consumer.Name,
		ConsumerModuleVersionID: consumerVersion.ID,
		ConsumerVersion:         consumerVersion.Version,
		ProviderModuleID:        provider.ID,
		ProviderModuleName:      provider.Name,
		ProviderModuleVersionID: providerVersion.ID,
		ProviderVersion:         providerVersion.Version,
		Source:                  source,
		Reason:                  reason,
		CreatedAt:               at(5),
	}
}

func at(hour int) time.Time {
	return time.Date(2026, 6, 4, hour, 0, 0, 0, time.UTC)
}

func findModuleOverview(t *testing.T, items []ModuleOverview, name string) ModuleOverview {
	t.Helper()
	for _, item := range items {
		if item.Module.Name == name {
			return item
		}
	}
	t.Fatalf("module %q not found in %#v", name, items)
	return ModuleOverview{}
}

type fakeModuleRepo struct {
	items []domain.Module
}

func (repo *fakeModuleRepo) Create(ctx context.Context, module domain.Module) error {
	panic("fakeModuleRepo.Create was called unexpectedly")
}
func (repo *fakeModuleRepo) GetByID(ctx context.Context, id domain.ModuleID) (domain.Module, error) {
	for _, item := range repo.items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.Module{}, domain.ErrNotFound
}
func (repo *fakeModuleRepo) GetByName(ctx context.Context, name domain.ModuleName) (domain.Module, error) {
	for _, item := range repo.items {
		if item.Name == name {
			return item, nil
		}
	}
	return domain.Module{}, domain.ErrNotFound
}
func (repo *fakeModuleRepo) List(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	items := append([]domain.Module(nil), repo.items...)
	sort.SliceStable(items, func(i, j int) bool { return strings.Compare(items[i].Name.String(), items[j].Name.String()) < 0 })
	return paginate(items, limit, offset), nil
}

type fakeGitLabRepo struct {
	items map[domain.ModuleID]domain.ModuleGitLabProject
}

func (repo *fakeGitLabRepo) Upsert(ctx context.Context, mapping domain.ModuleGitLabProject) error {
	panic("fakeGitLabRepo.Upsert was called unexpectedly")
}
func (repo *fakeGitLabRepo) GetByModuleID(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleGitLabProject, error) {
	item, ok := repo.items[moduleID]
	if !ok {
		return domain.ModuleGitLabProject{}, domain.ErrNotFound
	}
	return item, nil
}
func (repo *fakeGitLabRepo) GetByGitLabProject(ctx context.Context, gitLabBaseURL string, gitLabProjectID int64) (domain.ModuleGitLabProject, error) {
	panic("fakeGitLabRepo.GetByGitLabProject was called unexpectedly")
}

type fakeVersionRepo struct {
	items []domain.ModuleVersion
}

func (repo *fakeVersionRepo) Create(ctx context.Context, version domain.ModuleVersion) error {
	panic("fakeVersionRepo.Create was called unexpectedly")
}
func (repo *fakeVersionRepo) UpdateDeprecation(ctx context.Context, id domain.ModuleVersionID, deprecatedAt *time.Time, deprecatedBy string, deprecationReason string) error {
	panic("fakeVersionRepo.UpdateDeprecation was called unexpectedly")
}
func (repo *fakeVersionRepo) GetByID(ctx context.Context, id domain.ModuleVersionID) (domain.ModuleVersion, error) {
	for _, item := range repo.items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.ModuleVersion{}, domain.ErrNotFound
}
func (repo *fakeVersionRepo) GetByModuleAndVersion(ctx context.Context, moduleID domain.ModuleID, version domain.Version) (domain.ModuleVersion, error) {
	for _, item := range repo.items {
		if item.ModuleID == moduleID && item.Version == version {
			return item, nil
		}
	}
	return domain.ModuleVersion{}, domain.ErrNotFound
}
func (repo *fakeVersionRepo) GetLatestByModule(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleVersion, error) {
	items, err := repo.ListByModule(ctx, moduleID, 1, 0)
	if err != nil || len(items) == 0 {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return items[0], nil
}
func (repo *fakeVersionRepo) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ModuleVersion, error) {
	items := make([]domain.ModuleVersion, 0)
	for _, item := range repo.items {
		if item.ModuleID == moduleID {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, limit, offset), nil
}

type fakeArtifactRepo struct {
	items map[domain.ModuleVersionID][]domain.Artifact
}

func (repo *fakeArtifactRepo) Create(ctx context.Context, artifact domain.Artifact) error {
	panic("fakeArtifactRepo.Create was called unexpectedly")
}
func (repo *fakeArtifactRepo) GetByID(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error) {
	panic("fakeArtifactRepo.GetByID was called unexpectedly")
}
func (repo *fakeArtifactRepo) GetByModuleVersionAndKind(ctx context.Context, moduleVersionID domain.ModuleVersionID, kind domain.ArtifactKind) (domain.Artifact, error) {
	panic("fakeArtifactRepo.GetByModuleVersionAndKind was called unexpectedly")
}
func (repo *fakeArtifactRepo) ListByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.Artifact, error) {
	return append([]domain.Artifact(nil), repo.items[moduleVersionID]...), nil
}

type fakeBufConfigRepo struct {
	items map[domain.ModuleVersionID]domain.BufConfigInfo
}

func (repo *fakeBufConfigRepo) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, config domain.BufConfigInfo) error {
	panic("fakeBufConfigRepo.Save was called unexpectedly")
}
func (repo *fakeBufConfigRepo) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.BufConfigInfo, error) {
	item, ok := repo.items[moduleVersionID]
	if !ok {
		return domain.BufConfigInfo{}, domain.ErrNotFound
	}
	return item, nil
}

type fakeMetadataRepo struct {
	items   map[domain.ModuleVersionID]domain.DescriptorMetadata
	summary map[domain.ModuleVersionID]domain.DescriptorMetadataSummary
}

func (repo *fakeMetadataRepo) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, metadata domain.DescriptorMetadata) error {
	panic("fakeMetadataRepo.Save was called unexpectedly")
}
func (repo *fakeMetadataRepo) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadata, error) {
	item, ok := repo.items[moduleVersionID]
	if !ok {
		return domain.DescriptorMetadata{}, domain.ErrNotFound
	}
	return item, nil
}
func (repo *fakeMetadataRepo) GetSummaryByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadataSummary, error) {
	item, ok := repo.summary[moduleVersionID]
	if !ok {
		return domain.DescriptorMetadataSummary{}, domain.ErrNotFound
	}
	return item, nil
}

type fakeReportRepo struct {
	items   map[domain.ModuleID][]domain.BreakingReport
	changes map[domain.BreakingReportID][]domain.BreakingChange
}

func (repo *fakeReportRepo) Create(ctx context.Context, report domain.BreakingReport, changes []domain.BreakingChange) error {
	panic("fakeReportRepo.Create was called unexpectedly")
}
func (repo *fakeReportRepo) GetByID(ctx context.Context, id domain.BreakingReportID) (domain.BreakingReport, []domain.BreakingChange, error) {
	for _, reports := range repo.items {
		for _, report := range reports {
			if report.ID == id {
				return report, append([]domain.BreakingChange(nil), repo.changes[id]...), nil
			}
		}
	}
	return domain.BreakingReport{}, nil, domain.ErrNotFound
}
func (repo *fakeReportRepo) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.BreakingReport, error) {
	items := append([]domain.BreakingReport(nil), repo.items[moduleID]...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return paginate(items, limit, offset), nil
}
func (repo *fakeReportRepo) CountChangesByReport(ctx context.Context, reportID domain.BreakingReportID) (int, error) {
	return len(repo.changes[reportID]), nil
}
func (repo *fakeReportRepo) ListChangesByReport(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.BreakingChange, error) {
	return paginate(append([]domain.BreakingChange(nil), repo.changes[reportID]...), limit, offset), nil
}

type fakeDependencyRepo struct {
	dependencies []domain.ModuleDependency
	unresolved   []domain.UnresolvedProtoDependency
}

func (repo *fakeDependencyRepo) ReplaceByConsumerModuleVersion(ctx context.Context, consumerModuleVersionID domain.ModuleVersionID, dependencies []domain.ModuleDependency, unresolved []domain.UnresolvedProtoDependency) error {
	panic("fakeDependencyRepo.ReplaceByConsumerModuleVersion was called unexpectedly")
}
func (repo *fakeDependencyRepo) ListUpstreamByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleDependency, error) {
	items := make([]domain.ModuleDependency, 0)
	for _, dependency := range repo.dependencies {
		if dependency.ConsumerModuleID == moduleID {
			items = append(items, dependency)
		}
	}
	return items, nil
}
func (repo *fakeDependencyRepo) ListUpstreamByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.ModuleDependency, error) {
	panic("fakeDependencyRepo.ListUpstreamByModuleVersion was called unexpectedly")
}
func (repo *fakeDependencyRepo) ListDownstreamByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleDependency, error) {
	items := make([]domain.ModuleDependency, 0)
	for _, dependency := range repo.dependencies {
		if dependency.ProviderModuleID == moduleID {
			items = append(items, dependency)
		}
	}
	return items, nil
}
func (repo *fakeDependencyRepo) ListAffectedModules(ctx context.Context, providerModuleID domain.ModuleID) ([]domain.AffectedModule, error) {
	modules := dependencyModules(repo.filteredDownstream(providerModuleID), false)
	items := make([]domain.AffectedModule, 0, len(modules))
	for _, module := range modules {
		name, err := domain.NewModuleName(module.Module)
		if err != nil {
			return nil, err
		}
		version, err := domain.NewVersion(module.Version)
		if err != nil {
			return nil, err
		}
		items = append(items, domain.AffectedModule{
			ModuleName:        name,
			LatestVersion:     version,
			DependencySources: dependencySources(module.DependencySources),
			Reasons:           dependencyReasons(module.Reasons),
		})
	}
	return items, nil
}
func (repo *fakeDependencyRepo) ListUnresolvedByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.UnresolvedProtoDependency, error) {
	items := make([]domain.UnresolvedProtoDependency, 0)
	for _, dependency := range repo.unresolved {
		if dependency.ModuleID == moduleID {
			items = append(items, dependency)
		}
	}
	return items, nil
}
func (repo *fakeDependencyRepo) ListUnresolvedByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.UnresolvedProtoDependency, error) {
	panic("fakeDependencyRepo.ListUnresolvedByModuleVersion was called unexpectedly")
}
func (repo *fakeDependencyRepo) filteredDownstream(moduleID domain.ModuleID) []domain.ModuleDependency {
	items := make([]domain.ModuleDependency, 0)
	for _, dependency := range repo.dependencies {
		if dependency.ProviderModuleID == moduleID {
			items = append(items, dependency)
		}
	}
	return items
}

func dependencySources(values []string) []domain.DependencySource {
	items := make([]domain.DependencySource, 0, len(values))
	for _, value := range values {
		items = append(items, domain.DependencySource(value))
	}
	return items
}

func dependencyReasons(values []string) []domain.DependencyResolutionReason {
	items := make([]domain.DependencyResolutionReason, 0, len(values))
	for _, value := range values {
		items = append(items, domain.DependencyResolutionReason(value))
	}
	return items
}

func paginate[T any](items []T, limit int, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	items = items[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}
