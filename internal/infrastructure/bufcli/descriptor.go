package bufcli

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func ExtractDescriptorMetadata(image []byte) (domain.DescriptorMetadata, error) {
	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(image, &set); err != nil {
		return domain.DescriptorMetadata{}, err
	}

	files := make([]domain.ProtoFile, 0, len(set.File))
	for _, file := range set.File {
		protoFile := domain.ProtoFile{
			Path:        file.GetName(),
			PackageName: file.GetPackage(),
			Syntax:      file.GetSyntax(),
			Imports:     extractImports(file),
			Services:    extractServices(file),
			Messages:    extractMessages(file.GetPackage(), "", file.GetMessageType()),
			Enums:       extractEnums(file.GetPackage(), "", file.GetEnumType()),
		}
		files = append(files, protoFile)
	}
	return domain.DescriptorMetadata{Files: files}, nil
}

func extractImports(file *descriptorpb.FileDescriptorProto) []domain.ProtoImport {
	imports := make([]domain.ProtoImport, 0, len(file.Dependency))
	public := map[int32]bool{}
	for _, index := range file.PublicDependency {
		public[index] = true
	}
	weak := map[int32]bool{}
	for _, index := range file.WeakDependency {
		weak[index] = true
	}
	for index, path := range file.Dependency {
		imports = append(imports, domain.ProtoImport{
			Path:   path,
			Public: public[int32(index)],
			Weak:   weak[int32(index)],
		})
	}
	return imports
}

func extractServices(file *descriptorpb.FileDescriptorProto) []domain.ProtoService {
	services := make([]domain.ProtoService, 0, len(file.Service))
	for _, service := range file.Service {
		fullName := joinProtoName(file.GetPackage(), service.GetName())
		methods := make([]domain.ProtoMethod, 0, len(service.Method))
		for _, method := range service.Method {
			methods = append(methods, domain.ProtoMethod{
				Name:            method.GetName(),
				InputType:       method.GetInputType(),
				OutputType:      method.GetOutputType(),
				ClientStreaming: method.GetClientStreaming(),
				ServerStreaming: method.GetServerStreaming(),
			})
		}
		services = append(services, domain.ProtoService{
			Name:     service.GetName(),
			FullName: fullName,
			Methods:  methods,
		})
	}
	return services
}

func extractMessages(packageName string, parentFullName string, descriptors []*descriptorpb.DescriptorProto) []domain.ProtoMessage {
	messages := make([]domain.ProtoMessage, 0, len(descriptors))
	for _, message := range descriptors {
		fullName := joinProtoName(packageName, message.GetName())
		if parentFullName != "" {
			fullName = parentFullName + "." + message.GetName()
		}
		messages = append(messages, domain.ProtoMessage{
			Name:     message.GetName(),
			FullName: fullName,
			Fields:   extractFields(message),
			Messages: extractMessages(packageName, fullName, message.GetNestedType()),
			Enums:    extractEnums(packageName, fullName, message.GetEnumType()),
		})
	}
	return messages
}

func extractFields(message *descriptorpb.DescriptorProto) []domain.ProtoField {
	mapFields := map[string]bool{}
	for _, nested := range message.NestedType {
		if nested.GetOptions().GetMapEntry() {
			mapFields["."+nested.GetName()] = true
		}
	}

	fields := make([]domain.ProtoField, 0, len(message.Field))
	for _, field := range message.Field {
		fields = append(fields, domain.ProtoField{
			Name:       field.GetName(),
			Number:     field.GetNumber(),
			Type:       field.GetType().String(),
			TypeName:   field.GetTypeName(),
			Label:      field.GetLabel().String(),
			JSONName:   field.GetJsonName(),
			OneofName:  oneofName(message, field),
			IsRepeated: field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED,
			IsMap:      isMapField(field, mapFields),
		})
	}
	return fields
}

func oneofName(message *descriptorpb.DescriptorProto, field *descriptorpb.FieldDescriptorProto) string {
	if field.OneofIndex == nil {
		return ""
	}
	index := int(field.GetOneofIndex())
	if index < 0 || index >= len(message.OneofDecl) {
		return ""
	}
	return message.OneofDecl[index].GetName()
}

func isMapField(field *descriptorpb.FieldDescriptorProto, mapFields map[string]bool) bool {
	if field.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED || field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return false
	}
	for suffix := range mapFields {
		if hasProtoNameSuffix(field.GetTypeName(), suffix) {
			return true
		}
	}
	return false
}

func extractEnums(packageName string, parentFullName string, descriptors []*descriptorpb.EnumDescriptorProto) []domain.ProtoEnum {
	enums := make([]domain.ProtoEnum, 0, len(descriptors))
	for _, enum := range descriptors {
		fullName := joinProtoName(packageName, enum.GetName())
		if parentFullName != "" {
			fullName = parentFullName + "." + enum.GetName()
		}
		values := make([]domain.ProtoEnumValue, 0, len(enum.Value))
		for _, value := range enum.Value {
			values = append(values, domain.ProtoEnumValue{
				Name:   value.GetName(),
				Number: value.GetNumber(),
			})
		}
		enums = append(enums, domain.ProtoEnum{
			Name:     enum.GetName(),
			FullName: fullName,
			Values:   values,
		})
	}
	return enums
}

func joinProtoName(packageName string, name string) string {
	if packageName == "" {
		return name
	}
	return packageName + "." + name
}

func hasProtoNameSuffix(name string, suffix string) bool {
	if len(name) < len(suffix) {
		return false
	}
	return name[len(name)-len(suffix):] == suffix
}
