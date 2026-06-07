package httptransport

import "github.com/alryzden/ProtoRadar/internal/domain"

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
