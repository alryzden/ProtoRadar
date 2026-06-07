package web

type pageData struct {
	Title                  string
	Active                 string
	BasePath               string
	StaticPath             string
	Status                 string
	Module                 string
	Version                string
	ReportID               string
	Message                string
	EmptyTitle             string
	EmptyBody              string
	Query                  string
	Modules                []moduleRow
	ModuleView             *moduleDetailView
	DependencyView         *moduleDependencyView
	VersionView            *versionDetailView
	ReportFilters          reportFilters
	Reports                []reportRow
	ReportView             *breakingReportDetailView
	ApprovalRequestView    *approvalRequestDetailView
	RuntimeFilters         runtimeFilters
	RuntimeServices        []runtimeServiceRow
	RuntimeServiceView     *runtimeServiceDetailView
	RuntimeEnvironmentView *runtimeEnvironmentView
	ModuleRuntimeView      *moduleRuntimeUsagesView
	AboutView              *aboutView
}

type moduleRow struct {
	Name                string
	Description         string
	RepositoryURL       string
	GitLabProjectURL    string
	GitLabProjectLabel  string
	LatestVersion       string
	VersionCount        int
	LastPublishedOrSeen string
	BreakingReportCount int
	LastBreakingStatus  string
	HasLatestVersion    bool
	HasGitLabProject    bool
	HasRepositoryURL    bool
	HasLastBreaking     bool
	ModuleURL           string
	LatestVersionURL    string
	FilteredReportsURL  string
}

type moduleDetailView struct {
	Name               string
	Description        string
	RepositoryURL      string
	GitLabProjectURL   string
	GitLabProjectLabel string
	LatestVersion      string
	CreatedAt          string
	UpdatedAt          string
	HasRepositoryURL   bool
	HasGitLabProject   bool
	HasLatestVersion   bool
	Versions           []versionRow
	RecentReports      []reportRow
	FilteredReportsURL string
	DependencyGraphURL string
	RuntimeUsagesURL   string
	Owners             []moduleOwnerRow
	HasOwners          bool
}

type moduleOwnerRow struct {
	ID          string
	SubjectType string
	Subject     string
	Role        string
	CreatedAt   string
}

type moduleDependencyView struct {
	ModuleName    string
	ModuleURL     string
	Downstream    []dependencyModuleRow
	Upstream      []dependencyModuleRow
	Unresolved    []unresolvedDependencyRow
	HasDownstream bool
	HasUpstream   bool
	HasUnresolved bool
}

type dependencyModuleRow struct {
	Module            string
	Version           string
	DependencySources string
	Reasons           string
	ModuleURL         string
}

type unresolvedDependencyRow struct {
	Source           string
	ImportPath       string
	ReferencedSymbol string
	Reason           string
}

type versionRow struct {
	Version            string
	Status             string
	CreatedAt          string
	SourceDigest       string
	SourceDigestFull   string
	BufImageDigest     string
	BufImageDigestFull string
	LintStatus         string
	Files              int
	Services           int
	Methods            int
	Messages           int
	Enums              int
	VersionURL         string
	HasSourceDigest    bool
	HasBufImageDigest  bool
	HasLintStatus      bool
}

type reportRow struct {
	ID             string
	Module         string
	CreatedAt      string
	BaseVersion    string
	TargetRef      string
	Status         string
	ChangeCount    int
	ReportURL      string
	ModuleURL      string
	BaseVersionURL string
}

type versionDetailView struct {
	ModuleURL         string
	ModulesURL        string
	ModuleName        string
	Version           string
	Status            string
	CreatedAt         string
	LintStatus        string
	CompileStatus     string
	MetadataCounts    metadataCounts
	Artifacts         []artifactRow
	BufConfig         bufConfigView
	ProtoFiles        []protoFileRow
	Imports           []importRow
	Methods           []methodRow
	Fields            []fieldRow
	EnumValues        []enumValueRow
	RelatedReports    []reportRow
	HasLintStatus     bool
	HasMetadata       bool
	HasArtifacts      bool
	HasRelatedReports bool
	HasModulePaths    bool
	HasDeps           bool
}

type metadataCounts struct {
	Files      int
	Packages   int
	Imports    int
	Services   int
	Methods    int
	Messages   int
	Fields     int
	Enums      int
	EnumValues int
}

type artifactRow struct {
	Kind           string
	Checksum       string
	ChecksumFull   string
	SizeBytes      int64
	DownloadURL    string
	HasChecksum    bool
	HasDownloadURL bool
}

type bufConfigView struct {
	ConfigPresent         string
	LockPresent           string
	ModulePaths           []string
	Deps                  []string
	LintEnabled           string
	LintStatus            string
	BreakingConfigPresent string
	HasLintStatus         bool
}

type protoFileRow struct {
	Path        string
	PackageName string
	Syntax      string
	ImportCount int
}

type importRow struct {
	FilePath   string
	ImportPath string
	Public     string
	Weak       string
}

type methodRow struct {
	FilePath        string
	ServiceFullName string
	Name            string
	InputType       string
	OutputType      string
	ClientStreaming string
	ServerStreaming string
}

type fieldRow struct {
	FilePath        string
	MessageFullName string
	Number          int32
	Name            string
	Type            string
	TypeName        string
	Label           string
	Repeated        string
	Map             string
}

type enumValueRow struct {
	FilePath     string
	EnumFullName string
	Name         string
	Number       int32
}

type reportFilters struct {
	Module     string
	Status     string
	Query      string
	HasFilters bool
}

type runtimeFilters struct {
	Query       string
	Environment string
	DriftStatus string
	HasFilters  bool
}

type breakingReportDetailView struct {
	ID                 string
	Module             string
	BaseVersion        string
	TargetRef          string
	Status             string
	ChangeCount        int
	CreatedAt          string
	HumanSummary       string
	ModuleURL          string
	BaseVersionURL     string
	ReportsURL         string
	Changes            []changeRow
	AffectedModules    []dependencyModuleRow
	RuntimeImpact      []runtimeImpactRow
	Approval           *approvalRequestView
	HasChanges         bool
	HasAffectedModules bool
	HasRuntimeImpact   bool
	HasApproval        bool
}

type changeRow struct {
	FilePath    string
	PackageName string
	Symbol      string
	RuleID      string
	Message     string
	Severity    string
}

type runtimeServiceRow struct {
	ServiceName         string
	Environments        string
	LastReportedAt      string
	UpToDateCount       int
	BehindLatestCount   int
	UnknownVersionCount int
	DeprecatedCount     int
	ServiceURL          string
	EnvironmentURLs     []runtimeEnvironmentLink
}

type runtimeEnvironmentLink struct {
	Name string
	URL  string
}

type runtimeServiceDetailView struct {
	ServiceName    string
	ServicesURL    string
	Deployments    []runtimeDeploymentView
	HasDeployments bool
}

type runtimeDeploymentView struct {
	ID             string
	ServiceName    string
	ServiceURL     string
	Environment    string
	EnvironmentURL string
	GitCommit      string
	BuildVersion   string
	ReportedAt     string
	Usages         []runtimeUsageRow
	HasUsages      bool
}

type runtimeUsageRow struct {
	Module        string
	Version       string
	LatestVersion string
	DriftStatus   string
	DriftReason   string
	ModuleURL     string
}

type runtimeEnvironmentView struct {
	Environment    string
	ServicesURL    string
	Deployments    []runtimeDeploymentView
	HasDeployments bool
}

type moduleRuntimeUsagesView struct {
	Module    string
	ModuleURL string
	Usages    []moduleRuntimeUsageRow
	HasUsages bool
}

type moduleRuntimeUsageRow struct {
	ServiceName    string
	ServiceURL     string
	Environment    string
	EnvironmentURL string
	Version        string
	LatestVersion  string
	DriftStatus    string
	DriftReason    string
	ReportedAt     string
}

type runtimeImpactRow struct {
	ServiceName    string
	ServiceURL     string
	Environment    string
	EnvironmentURL string
	UsedVersion    string
	BuildVersion   string
	GitCommit      string
	ReportedAt     string
	ImpactStatus   string
	Reason         string
	DriftStatus    string
	DriftReason    string
}

type approvalRequestView struct {
	ID                 string
	Module             string
	ModuleURL          string
	BreakingReportID   string
	BreakingReportURL  string
	TargetRef          string
	Status             string
	RequiredApprovals  int
	ReceivedApprovals  int
	Requirements       []approvalRequirementRow
	Decisions          []approvalDecisionRow
	CreatedAt          string
	UpdatedAt          string
	RequestURL         string
	HasRequirements    bool
	HasDecisions       bool
	MissingWarnings    []string
	HasMissingWarnings bool
}

type approvalRequirementRow struct {
	ID               string
	RequirementType  string
	TargetModuleName string
	TargetModuleURL  string
	RequiredRole     string
	Status           string
	Reason           string
}

type approvalDecisionRow struct {
	ID            string
	RequirementID string
	Decision      string
	DecidedBy     string
	Comment       string
	CreatedAt     string
}

type approvalRequestDetailView struct {
	Request    approvalRequestView
	AuditTrail []governanceAuditEventRow
	HasAudit   bool
}

type governanceAuditEventRow struct {
	ID          string
	EventType   string
	Actor       string
	Module      string
	PayloadJSON string
	CreatedAt   string
}

type aboutView struct {
	Edition             string
	Version             string
	Commit              string
	BuildDate           string
	Enabled             []capabilityRow
	Unavailable         []capabilityRow
	HasEnabled          bool
	HasUnavailable      bool
	HasCommit           bool
	HasBuildDate        bool
	DisplayEditionTitle string
}

type capabilityRow struct {
	Name string
}
