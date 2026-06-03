package domain

import "testing"

func TestDescriptorMetadataSummaryCountsNestedMetadata(t *testing.T) {
	metadata := DescriptorMetadata{
		Files: []ProtoFile{
			{
				Path:        "user/v1/user.proto",
				PackageName: "user.v1",
				Syntax:      "proto3",
				Imports: []ProtoImport{
					{Path: "google/protobuf/timestamp.proto"},
					{Path: "common/v1/common.proto", Public: true},
				},
				Services: []ProtoService{
					{
						Name:     "UserService",
						FullName: "user.v1.UserService",
						Methods: []ProtoMethod{
							{Name: "GetUser", InputType: ".user.v1.GetUserRequest", OutputType: ".user.v1.User"},
							{Name: "WatchUsers", InputType: ".user.v1.WatchUsersRequest", OutputType: ".user.v1.User", ServerStreaming: true},
						},
					},
				},
				Messages: []ProtoMessage{
					{
						Name:     "User",
						FullName: "user.v1.User",
						Fields: []ProtoField{
							{Name: "id", Number: 1, Type: "string", Label: "optional", JSONName: "id"},
							{Name: "tags", Number: 2, Type: "string", Label: "repeated", JSONName: "tags", IsRepeated: true},
						},
						Messages: []ProtoMessage{
							{
								Name:     "Profile",
								FullName: "user.v1.User.Profile",
								Fields: []ProtoField{
									{Name: "display_name", Number: 1, Type: "string", JSONName: "displayName"},
								},
							},
						},
						Enums: []ProtoEnum{
							{
								Name:     "State",
								FullName: "user.v1.User.State",
								Values: []ProtoEnumValue{
									{Name: "STATE_UNSPECIFIED", Number: 0},
									{Name: "STATE_ACTIVE", Number: 1},
								},
							},
						},
					},
				},
				Enums: []ProtoEnum{
					{
						Name:     "Role",
						FullName: "user.v1.Role",
						Values: []ProtoEnumValue{
							{Name: "ROLE_UNSPECIFIED", Number: 0},
						},
					},
				},
			},
			{
				Path:        "billing/v1/billing.proto",
				PackageName: "billing.v1",
				Syntax:      "proto3",
			},
		},
	}

	summary := metadata.Summary()
	if summary.FileCount != 2 {
		t.Fatalf("file count = %d", summary.FileCount)
	}
	if summary.PackageCount != 2 {
		t.Fatalf("package count = %d", summary.PackageCount)
	}
	if summary.ImportCount != 2 {
		t.Fatalf("import count = %d", summary.ImportCount)
	}
	if summary.ServiceCount != 1 {
		t.Fatalf("service count = %d", summary.ServiceCount)
	}
	if summary.MethodCount != 2 {
		t.Fatalf("method count = %d", summary.MethodCount)
	}
	if summary.MessageCount != 2 {
		t.Fatalf("message count = %d", summary.MessageCount)
	}
	if summary.FieldCount != 3 {
		t.Fatalf("field count = %d", summary.FieldCount)
	}
	if summary.EnumCount != 2 {
		t.Fatalf("enum count = %d", summary.EnumCount)
	}
	if summary.EnumValueCount != 3 {
		t.Fatalf("enum value count = %d", summary.EnumValueCount)
	}
}
