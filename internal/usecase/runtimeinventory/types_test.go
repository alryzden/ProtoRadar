package runtimeinventory

import "testing"

func TestParseModuleVersionReference(t *testing.T) {
	reference, err := ParseModuleVersionReference("user-api@v1.2.3")
	if err != nil {
		t.Fatalf("parse module reference: %v", err)
	}
	if reference.Module != "user-api" || reference.Version != "v1.2.3" {
		t.Fatalf("reference = %#v", reference)
	}
}

func TestParseModuleVersionReferenceTrimsWhitespace(t *testing.T) {
	reference, err := ParseModuleVersionReference(" user-api @ v1.2.3 ")
	if err != nil {
		t.Fatalf("parse module reference: %v", err)
	}
	if reference.Module != "user-api" || reference.Version != "v1.2.3" {
		t.Fatalf("reference = %#v", reference)
	}
}

func TestParseModuleVersionReferenceInvalid(t *testing.T) {
	invalid := []string{
		"",
		"   ",
		"user-api",
		"@v1.2.3",
		"user-api@",
		"user-api@v1@extra",
	}

	for _, value := range invalid {
		if _, err := ParseModuleVersionReference(value); err != ErrInvalidModuleVersionReference {
			t.Fatalf("reference %q error = %v, want %v", value, err, ErrInvalidModuleVersionReference)
		}
	}
}
