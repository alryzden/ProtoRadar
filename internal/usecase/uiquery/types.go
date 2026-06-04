package uiquery

import (
	"time"
)

type ListModuleOverviewsInput struct {
	Query string
}

type GetModuleOverviewInput struct {
	Module string
}

type GetVersionOverviewInput struct {
	Module  string
	Version string
}

type ListBreakingReportOverviewsInput struct {
	Module string
	Status string
	Query  string
}

type GetBreakingReportDetailsInput struct {
	ReportID string
}

type GetModuleDependencyGraphInput struct {
	Module string
}

type ModuleOverview struct {
	Module              ModuleInfo
	GitLabProject       *GitLabProjectInfo
	LatestVersion       *VersionSummary
	VersionCount        int
	LastPublishedOrSeen time.Time
	BreakingReportCount int
	LastBreakingStatus  string
	Versions            []VersionSummary
	RecentReports       []BreakingReportSummary
}

type ModuleInfo struct {
	Name          string
	Description   string
	RepositoryURL string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type GitLabProjectInfo struct {
	BaseURL     string
	ProjectID   int64
	ProjectPath string
	UpdatedAt   time.Time
}

type VersionSummary struct {
	Version         string
	Status          string
	Digest          string
	CreatedAt       time.Time
	PublishedAt     *time.Time
	Artifacts       []ArtifactSummary
	LintStatus      string
	MetadataSummary DescriptorMetadataSummary
}

type VersionOverview struct {
	Module         ModuleInfo
	Version        VersionSummary
	Artifacts      []ArtifactSummary
	BufConfig      BufConfigSummary
	Metadata       DescriptorMetadata
	MetadataCounts DescriptorMetadataSummary
	RelatedReports []BreakingReportSummary
}

type ArtifactSummary struct {
	Kind           string
	ChecksumSHA256 string
	SizeBytes      int64
}

type BufConfigSummary struct {
	ConfigPresent         bool
	LockPresent           bool
	BufYAMLDigest         string
	BufLockDigest         string
	ModulePaths           []string
	Deps                  []string
	LintEnabled           bool
	LintStatus            string
	BreakingConfigPresent bool
}

type DescriptorMetadataSummary struct {
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

type DescriptorMetadata struct {
	Files []ProtoFile
}

type ProtoFile struct {
	Path        string
	PackageName string
	Syntax      string
	Imports     []ProtoImport
	Services    []ProtoService
	Messages    []ProtoMessage
	Enums       []ProtoEnum
}

type ProtoImport struct {
	Path   string
	Public bool
	Weak   bool
}

type ProtoService struct {
	Name     string
	FullName string
	Methods  []ProtoMethod
}

type ProtoMethod struct {
	Name            string
	InputType       string
	OutputType      string
	ClientStreaming bool
	ServerStreaming bool
}

type ProtoMessage struct {
	Name     string
	FullName string
	Fields   []ProtoField
	Messages []ProtoMessage
	Enums    []ProtoEnum
}

type ProtoField struct {
	Name       string
	Number     int32
	Type       string
	TypeName   string
	Label      string
	JSONName   string
	OneofName  string
	IsRepeated bool
	IsMap      bool
}

type ProtoEnum struct {
	Name     string
	FullName string
	Values   []ProtoEnumValue
}

type ProtoEnumValue struct {
	Name   string
	Number int32
}

type BreakingReportSummary struct {
	ID          string
	Module      string
	BaseVersion string
	TargetRef   string
	Status      string
	ChangeCount int
	CreatedAt   time.Time
}

type BreakingReportDetails struct {
	Report          BreakingReportSummary
	Summary         string
	Changes         []BreakingChangeSummary
	AffectedModules []DependencyModule
}

type BreakingChangeSummary struct {
	Category    string
	FilePath    string
	PackageName string
	Symbol      string
	RuleID      string
	Message     string
	Severity    string
	CreatedAt   time.Time
}

type ModuleDependencyGraph struct {
	Module     ModuleInfo
	Downstream []DependencyModule
	Upstream   []DependencyModule
	Unresolved []UnresolvedDependency
}

type DependencyModule struct {
	Module            string
	Version           string
	DependencySources []string
	Reasons           []string
}

type UnresolvedDependency struct {
	Source           string
	ImportPath       string
	ReferencedSymbol string
	Reason           string
}
