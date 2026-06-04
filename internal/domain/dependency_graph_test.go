package domain

import "testing"

func TestDependencySourceValidation(t *testing.T) {
	valid := []DependencySource{
		DependencySourceImport,
		DependencySourceTypeReference,
		DependencySourceMethodReference,
		DependencySourcePackageFallback,
	}
	for _, source := range valid {
		if !source.IsValid() {
			t.Fatalf("source %q should be valid", source)
		}
		if source.String() == "" {
			t.Fatalf("source %q should have string value", source)
		}
	}

	for _, source := range []DependencySource{"", "kafka", "postgres"} {
		if source.IsValid() {
			t.Fatalf("source %q should be invalid", source)
		}
	}
}
