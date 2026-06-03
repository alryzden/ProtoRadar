package domain

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

type DescriptorMetadataSummary struct {
	FileCount      int
	PackageCount   int
	ImportCount    int
	ServiceCount   int
	MethodCount    int
	MessageCount   int
	FieldCount     int
	EnumCount      int
	EnumValueCount int
}

func (metadata DescriptorMetadata) Summary() DescriptorMetadataSummary {
	var summary DescriptorMetadataSummary
	packages := map[string]struct{}{}
	for _, file := range metadata.Files {
		summary.FileCount++
		if file.PackageName != "" {
			packages[file.PackageName] = struct{}{}
		}
		summary.ImportCount += len(file.Imports)
		for _, service := range file.Services {
			summary.ServiceCount++
			summary.MethodCount += len(service.Methods)
		}
		for _, message := range file.Messages {
			addMessageSummary(&summary, message)
		}
		for _, enum := range file.Enums {
			addEnumSummary(&summary, enum)
		}
	}
	summary.PackageCount = len(packages)
	return summary
}

func addMessageSummary(summary *DescriptorMetadataSummary, message ProtoMessage) {
	summary.MessageCount++
	summary.FieldCount += len(message.Fields)
	for _, nested := range message.Messages {
		addMessageSummary(summary, nested)
	}
	for _, enum := range message.Enums {
		addEnumSummary(summary, enum)
	}
}

func addEnumSummary(summary *DescriptorMetadataSummary, enum ProtoEnum) {
	summary.EnumCount++
	summary.EnumValueCount += len(enum.Values)
}
