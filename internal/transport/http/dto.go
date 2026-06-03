package httptransport

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

type errorResponse struct {
	Error string `json:"error"`
}

type createModuleRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	RepositoryURL string `json:"repository_url"`
}

type moduleDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	RepositoryURL string    `json:"repository_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type listModulesResponse struct {
	Modules []moduleDTO `json:"modules"`
}

type moduleVersionDTO struct {
	ID          string     `json:"id"`
	ModuleID    string     `json:"module_id"`
	Version     string     `json:"version"`
	Digest      string     `json:"digest"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type listModuleVersionsResponse struct {
	Versions []moduleVersionDTO `json:"versions"`
}

type artifactDTO struct {
	ID              string    `json:"id"`
	ModuleVersionID string    `json:"module_version_id"`
	Kind            string    `json:"kind"`
	StorageKey      string    `json:"storage_key"`
	ChecksumSHA256  string    `json:"checksum_sha256"`
	SizeBytes       int64     `json:"size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type artifactSummaryDTO struct {
	Kind           string `json:"kind"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

type bufInfoDTO struct {
	ConfigPresent         bool     `json:"config_present"`
	LockPresent           bool     `json:"lock_present"`
	BufYAMLDigest         string   `json:"buf_yaml_digest,omitempty"`
	BufLockDigest         string   `json:"buf_lock_digest,omitempty"`
	ModulePaths           []string `json:"module_paths,omitempty"`
	Deps                  []string `json:"deps,omitempty"`
	LintEnabled           bool     `json:"lint_enabled"`
	LintStatus            string   `json:"lint_status"`
	LintReport            string   `json:"lint_report,omitempty"`
	BreakingConfigPresent bool     `json:"breaking_config_present"`
}

type metadataSummaryDTO struct {
	Files      int `json:"files"`
	Packages   int `json:"packages,omitempty"`
	Imports    int `json:"imports"`
	Services   int `json:"services"`
	Methods    int `json:"methods"`
	Messages   int `json:"messages"`
	Fields     int `json:"fields"`
	Enums      int `json:"enums"`
	EnumValues int `json:"enum_values"`
}

type publishModuleVersionResponse struct {
	Module           string             `json:"module"`
	Version          string             `json:"version"`
	Status           string             `json:"status"`
	SourceArtifact   artifactSummaryDTO `json:"source_artifact"`
	BufImageArtifact artifactSummaryDTO `json:"buf_image_artifact"`
	Buf              bufInfoDTO         `json:"buf"`
	MetadataSummary  metadataSummaryDTO `json:"metadata_summary"`
	CreatedAt        time.Time          `json:"created_at"`
}

type moduleVersionDetailsDTO struct {
	ID              string               `json:"id"`
	ModuleID        string               `json:"module_id"`
	Module          string               `json:"module"`
	Version         string               `json:"version"`
	Digest          string               `json:"digest"`
	Status          string               `json:"status"`
	Artifacts       []artifactSummaryDTO `json:"artifacts"`
	Buf             bufInfoDTO           `json:"buf"`
	MetadataSummary metadataSummaryDTO   `json:"metadata_summary"`
	CreatedAt       time.Time            `json:"created_at"`
	PublishedAt     *time.Time           `json:"published_at,omitempty"`
}

type descriptorMetadataDTO struct {
	Files []protoFileDTO `json:"files"`
}

type protoFileDTO struct {
	Path        string            `json:"path"`
	PackageName string            `json:"package_name"`
	Syntax      string            `json:"syntax"`
	Imports     []protoImportDTO  `json:"imports,omitempty"`
	Services    []protoServiceDTO `json:"services,omitempty"`
	Messages    []protoMessageDTO `json:"messages,omitempty"`
	Enums       []protoEnumDTO    `json:"enums,omitempty"`
}

type protoImportDTO struct {
	Path   string `json:"path"`
	Public bool   `json:"public"`
	Weak   bool   `json:"weak"`
}

type protoServiceDTO struct {
	Name     string           `json:"name"`
	FullName string           `json:"full_name"`
	Methods  []protoMethodDTO `json:"methods,omitempty"`
}

type protoMethodDTO struct {
	Name            string `json:"name"`
	InputType       string `json:"input_type"`
	OutputType      string `json:"output_type"`
	ClientStreaming bool   `json:"client_streaming"`
	ServerStreaming bool   `json:"server_streaming"`
}

type protoMessageDTO struct {
	Name     string            `json:"name"`
	FullName string            `json:"full_name"`
	Fields   []protoFieldDTO   `json:"fields,omitempty"`
	Messages []protoMessageDTO `json:"messages,omitempty"`
	Enums    []protoEnumDTO    `json:"enums,omitempty"`
}

type protoFieldDTO struct {
	Name       string `json:"name"`
	Number     int32  `json:"number"`
	Type       string `json:"type"`
	TypeName   string `json:"type_name"`
	Label      string `json:"label"`
	JSONName   string `json:"json_name"`
	OneofName  string `json:"oneof_name"`
	IsRepeated bool   `json:"is_repeated"`
	IsMap      bool   `json:"is_map"`
}

type protoEnumDTO struct {
	Name     string              `json:"name"`
	FullName string              `json:"full_name"`
	Values   []protoEnumValueDTO `json:"values,omitempty"`
}

type protoEnumValueDTO struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

type createAPITokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type createAPITokenResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func moduleResponse(module domain.Module) moduleDTO {
	return moduleDTO{
		ID:            module.ID.String(),
		Name:          module.Name.String(),
		Description:   module.Description,
		RepositoryURL: module.RepositoryURL,
		CreatedAt:     module.CreatedAt,
		UpdatedAt:     module.UpdatedAt,
	}
}

func moduleVersionResponse(version domain.ModuleVersion) moduleVersionDTO {
	return moduleVersionDTO{
		ID:          version.ID.String(),
		ModuleID:    version.ModuleID.String(),
		Version:     version.Version.String(),
		Digest:      version.Digest,
		Status:      version.Status.String(),
		CreatedAt:   version.CreatedAt,
		PublishedAt: version.PublishedAt,
	}
}

func artifactResponse(artifact domain.Artifact) artifactDTO {
	return artifactDTO{
		ID:              artifact.ID.String(),
		ModuleVersionID: artifact.ModuleVersionID.String(),
		Kind:            artifact.Kind.String(),
		StorageKey:      artifact.StorageKey,
		ChecksumSHA256:  artifact.ChecksumSHA256,
		SizeBytes:       artifact.SizeBytes,
		CreatedAt:       artifact.CreatedAt,
	}
}

func artifactSummaryResponse(artifact domain.Artifact) artifactSummaryDTO {
	return artifactSummaryDTO{
		Kind:           artifact.Kind.String(),
		SizeBytes:      artifact.SizeBytes,
		ChecksumSHA256: artifact.ChecksumSHA256,
	}
}

func bufInfoResponse(config domain.BufConfigInfo, lint domain.BufLintResult) bufInfoDTO {
	lintStatus := lint.Status.String()
	if lintStatus == "" {
		lintStatus = domain.BufLintStatusNotRun.String()
	}
	return bufInfoDTO{
		ConfigPresent:         config.BufYAMLPresent,
		LockPresent:           config.BufLockPresent,
		BufYAMLDigest:         config.BufYAMLDigest,
		BufLockDigest:         config.BufLockDigest,
		ModulePaths:           config.ModulePaths,
		Deps:                  config.Deps,
		LintEnabled:           config.LintEnabled,
		LintStatus:            lintStatus,
		LintReport:            lint.Report,
		BreakingConfigPresent: config.BreakingConfigPresent,
	}
}

func metadataSummaryResponse(summary domain.DescriptorMetadataSummary) metadataSummaryDTO {
	return metadataSummaryDTO{
		Files:      summary.FileCount,
		Packages:   summary.PackageCount,
		Imports:    summary.ImportCount,
		Services:   summary.ServiceCount,
		Methods:    summary.MethodCount,
		Messages:   summary.MessageCount,
		Fields:     summary.FieldCount,
		Enums:      summary.EnumCount,
		EnumValues: summary.EnumValueCount,
	}
}

func moduleVersionDetailsResponse(moduleName string, details registry.ModuleVersionDetailsResponse) moduleVersionDetailsDTO {
	artifacts := make([]artifactSummaryDTO, 0, len(details.Artifacts))
	for _, artifact := range details.Artifacts {
		artifacts = append(artifacts, artifactSummaryResponse(artifact))
	}
	return moduleVersionDetailsDTO{
		ID:              details.Version.ID.String(),
		ModuleID:        details.Version.ModuleID.String(),
		Module:          moduleName,
		Version:         details.Version.Version.String(),
		Digest:          details.Version.Digest,
		Status:          details.Version.Status.String(),
		Artifacts:       artifacts,
		Buf:             bufInfoResponse(details.BufConfig, details.LintResult),
		MetadataSummary: metadataSummaryResponse(details.MetadataSummary),
		CreatedAt:       details.Version.CreatedAt,
		PublishedAt:     details.Version.PublishedAt,
	}
}

func descriptorMetadataResponse(metadata domain.DescriptorMetadata) descriptorMetadataDTO {
	files := make([]protoFileDTO, 0, len(metadata.Files))
	for _, file := range metadata.Files {
		files = append(files, protoFileResponse(file))
	}
	return descriptorMetadataDTO{Files: files}
}

func protoFileResponse(file domain.ProtoFile) protoFileDTO {
	imports := make([]protoImportDTO, 0, len(file.Imports))
	for _, protoImport := range file.Imports {
		imports = append(imports, protoImportDTO{
			Path:   protoImport.Path,
			Public: protoImport.Public,
			Weak:   protoImport.Weak,
		})
	}
	services := make([]protoServiceDTO, 0, len(file.Services))
	for _, service := range file.Services {
		services = append(services, protoServiceResponse(service))
	}
	messages := make([]protoMessageDTO, 0, len(file.Messages))
	for _, message := range file.Messages {
		messages = append(messages, protoMessageResponse(message))
	}
	enums := make([]protoEnumDTO, 0, len(file.Enums))
	for _, enum := range file.Enums {
		enums = append(enums, protoEnumResponse(enum))
	}
	return protoFileDTO{
		Path:        file.Path,
		PackageName: file.PackageName,
		Syntax:      file.Syntax,
		Imports:     imports,
		Services:    services,
		Messages:    messages,
		Enums:       enums,
	}
}

func protoServiceResponse(service domain.ProtoService) protoServiceDTO {
	methods := make([]protoMethodDTO, 0, len(service.Methods))
	for _, method := range service.Methods {
		methods = append(methods, protoMethodDTO{
			Name:            method.Name,
			InputType:       method.InputType,
			OutputType:      method.OutputType,
			ClientStreaming: method.ClientStreaming,
			ServerStreaming: method.ServerStreaming,
		})
	}
	return protoServiceDTO{Name: service.Name, FullName: service.FullName, Methods: methods}
}

func protoMessageResponse(message domain.ProtoMessage) protoMessageDTO {
	fields := make([]protoFieldDTO, 0, len(message.Fields))
	for _, field := range message.Fields {
		fields = append(fields, protoFieldDTO{
			Name:       field.Name,
			Number:     field.Number,
			Type:       field.Type,
			TypeName:   field.TypeName,
			Label:      field.Label,
			JSONName:   field.JSONName,
			OneofName:  field.OneofName,
			IsRepeated: field.IsRepeated,
			IsMap:      field.IsMap,
		})
	}
	messages := make([]protoMessageDTO, 0, len(message.Messages))
	for _, nested := range message.Messages {
		messages = append(messages, protoMessageResponse(nested))
	}
	enums := make([]protoEnumDTO, 0, len(message.Enums))
	for _, enum := range message.Enums {
		enums = append(enums, protoEnumResponse(enum))
	}
	return protoMessageDTO{
		Name:     message.Name,
		FullName: message.FullName,
		Fields:   fields,
		Messages: messages,
		Enums:    enums,
	}
}

func protoEnumResponse(enum domain.ProtoEnum) protoEnumDTO {
	values := make([]protoEnumValueDTO, 0, len(enum.Values))
	for _, value := range enum.Values {
		values = append(values, protoEnumValueDTO{Name: value.Name, Number: value.Number})
	}
	return protoEnumDTO{Name: enum.Name, FullName: enum.FullName, Values: values}
}
