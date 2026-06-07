package archtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type packageInfo struct {
	ImportPath  string
	Dir         string
	Name        string
	ModulePath  string
	Imports     []string
	TestImports []string
}

type goListPackage struct {
	ImportPath   string
	Dir          string
	Name         string
	Imports      []string
	TestImports  []string
	XTestImports []string
	Module       *struct {
		Path string
		Main bool
		Dir  string
	}
}

type importSet int

const (
	productionImports importSet = iota
	testImports
	allImports
)

type boundaryRule struct {
	Name                  string
	PackagePrefix         string
	ForbiddenPrefixes     []string
	ForbiddenExactImports []string
	ForbiddenSubstrings   []string
	ImportSet             importSet
	SuggestedFix          string
	Exceptions            []allowedImport
}

type allowedImport struct {
	Package string
	Import  string
	Reason  string
}

type boundaryViolation struct {
	Rule         string
	Package      string
	Import       string
	ImportKind   string
	SuggestedFix string
}

type sourceCallViolation struct {
	File         string
	Line         int
	Call         string
	SuggestedFix string
}

func TestPackageInventorySmoke(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)

	if modulePath != "github.com/alryzden/ProtoRadar" {
		t.Fatalf("module path = %q, want github.com/alryzden/ProtoRadar", modulePath)
	}
	if len(packages) == 0 {
		t.Fatal("go list returned no packages")
	}
	if !hasPackage(packages, modulePath+"/internal/domain") {
		t.Fatalf("package inventory does not include %s/internal/domain", modulePath)
	}
}

func TestBoundaryRuleDetectsSyntheticViolation(t *testing.T) {
	packages := []packageInfo{
		{
			ImportPath: "github.com/alryzden/ProtoRadar/internal/domain",
			Imports:    []string{"github.com/alryzden/ProtoRadar/internal/infrastructure/bufcli"},
		},
	}
	rule := boundaryRule{
		Name:              "synthetic domain rule",
		PackagePrefix:     "github.com/alryzden/ProtoRadar/internal/domain",
		ForbiddenPrefixes: []string{"github.com/alryzden/ProtoRadar/internal/infrastructure"},
		ImportSet:         productionImports,
		SuggestedFix:      "move concrete adapter usage behind a domain port and wire it in bootstrap",
	}

	violations := checkBoundaryRule(packages, rule)
	if len(violations) != 1 {
		t.Fatalf("violations = %#v, want one synthetic violation", violations)
	}
	violation := violations[0]
	if violation.Package != packages[0].ImportPath || violation.Import != packages[0].Imports[0] || violation.Rule != rule.Name {
		t.Fatalf("violation = %#v", violation)
	}
	if violation.ImportKind != "production" {
		t.Fatalf("import kind = %q, want production", violation.ImportKind)
	}
}

func TestBoundaryRuleHonorsPackageImportExceptions(t *testing.T) {
	packages := []packageInfo{
		{
			ImportPath: "github.com/alryzden/ProtoRadar/internal/cli",
			Imports:    []string{"github.com/alryzden/ProtoRadar/internal/infrastructure/gitlabapi"},
		},
	}
	rule := boundaryRule{
		Name:              "synthetic CLI rule",
		PackagePrefix:     "github.com/alryzden/ProtoRadar/internal/cli",
		ForbiddenPrefixes: []string{"github.com/alryzden/ProtoRadar/internal/infrastructure"},
		ImportSet:         productionImports,
		SuggestedFix:      "route server-side behavior through the REST API client",
		Exceptions: []allowedImport{
			{
				Package: "github.com/alryzden/ProtoRadar/internal/cli",
				Import:  "github.com/alryzden/ProtoRadar/internal/infrastructure/gitlabapi",
				Reason:  "GitLab MR command currently runs local MR bot orchestration.",
			},
		},
	}

	if violations := checkBoundaryRule(packages, rule); len(violations) != 0 {
		t.Fatalf("violations = %#v, want none", violations)
	}
}

func TestDomainDoesNotImportInfrastructure(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "domain production imports must not depend on concrete outer layers",
		PackagePrefix: modulePath + "/internal/domain",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/adapters",
			modulePath + "/internal/app",
			modulePath + "/internal/cli",
			modulePath + "/internal/config",
			modulePath + "/internal/infrastructure",
			modulePath + "/internal/observability",
			modulePath + "/internal/repository",
			modulePath + "/internal/transport",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
			"github.com/bufbuild",
			"github.com/jackc/pgx",
			"github.com/minio",
			"github.com/pressly/goose",
			"github.com/xanzy/go-gitlab",
		},
		ForbiddenExactImports: []string{
			"database/sql",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"gitlabapi",
			"kafka",
			"/enterprise",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "move the dependency behind a domain port or value object and wire concrete implementations in bootstrap/adapters",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestUsecaseDoesNotImportConcreteInfrastructure(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "usecase production imports must not depend on concrete infrastructure",
		PackagePrefix: modulePath + "/internal/usecase",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/adapters",
			modulePath + "/internal/app",
			modulePath + "/internal/cli",
			modulePath + "/internal/infrastructure",
			modulePath + "/internal/repository/postgres",
			modulePath + "/internal/transport/http",
			modulePath + "/internal/transport/web",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
			"github.com/bufbuild",
			"github.com/minio",
			"github.com/pressly/goose",
			"github.com/xanzy/go-gitlab",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"gitlabapi",
			"kafka",
			"minio-go",
			"topicresolver",
			"topicrouting",
			"/enterprise",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "depend on application ports or integration contracts, and keep concrete adapters in infrastructure/bootstrap",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestHTTPTransportDoesNotImportPostgres(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "HTTP transport production imports must not depend on persistence or transport infrastructure",
		PackagePrefix: modulePath + "/internal/transport/http",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/repository/postgres",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
			"github.com/jackc/pgx",
			"github.com/minio",
		},
		ForbiddenExactImports: []string{
			"database/sql",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"kafka",
			"minio-go",
		},
		ImportSet:    productionImports,
		SuggestedFix: "depend on usecase/application interfaces and keep Postgres, S3, and event transport adapters in bootstrap/infrastructure",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestWebTransportDoesNotImportPostgres(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "Web transport production imports must not depend on concrete infrastructure",
		PackagePrefix: modulePath + "/internal/transport/web",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/infrastructure",
			modulePath + "/internal/repository/postgres",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
			"github.com/jackc/pgx",
			"github.com/minio",
		},
		ForbiddenExactImports: []string{
			"database/sql",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"kafka",
			"minio-go",
		},
		ImportSet:    productionImports,
		SuggestedFix: "read through usecase/query services and keep concrete infrastructure behind bootstrap wiring",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestCLIDoesNotImportServerInfrastructure(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "CLI production imports must not depend on server infrastructure",
		PackagePrefix: modulePath + "/internal/cli",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/infrastructure",
			modulePath + "/internal/repository/postgres",
			modulePath + "/internal/transport/http",
			modulePath + "/internal/transport/web",
			modulePath + "/internal/usecase",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
			"github.com/bufbuild",
			"github.com/minio",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"kafka",
			"minio-go",
		},
		ImportSet:    productionImports,
		SuggestedFix: "route server behavior through internal/cli/api REST calls, or keep client-side adapters narrowly isolated",
		Exceptions: []allowedImport{
			{
				Package: modulePath + "/internal/cli",
				Import:  modulePath + "/internal/infrastructure/gitlabapi",
				Reason:  "The GitLab MR command currently performs client-side GitLab API calls for local MR bot orchestration.",
			},
			{
				Package: modulePath + "/internal/cli",
				Import:  modulePath + "/internal/usecase/gitlabmr",
				Reason:  "The GitLab MR command currently reuses the transport-neutral MR bot orchestration usecase locally.",
			},
		},
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestOutboxRemainsTransportNeutral(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "outbox production imports must remain transport neutral",
		PackagePrefix: modulePath + "/internal/outbox",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/cli",
			modulePath + "/internal/repository/postgres",
			modulePath + "/internal/transport/http",
			modulePath + "/internal/transport/web",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"kafka",
			"/enterprise",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "keep outbox as application-level durable record/publisher ports and wire concrete dispatchers outside the package",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestIdentityRemainsTransportFree(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "identity production imports must remain transport and adapter free",
		PackagePrefix: modulePath + "/internal/identity",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/cli",
			modulePath + "/internal/repository/postgres",
			modulePath + "/internal/transport/http",
		},
		ForbiddenSubstrings: []string{
			"/enterprise",
			"/ldap",
			"/oidc",
			"advanced-rbac",
			"advanced_rbac",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "keep identity as an application boundary and provide concrete auth providers from downstream/bootstrap wiring",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestAuthorizationRemainsTransportFree(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rule := boundaryRule{
		Name:          "authorization production imports must remain transport and adapter free",
		PackagePrefix: modulePath + "/internal/authorization",
		ForbiddenPrefixes: []string{
			modulePath + "/internal/cli",
			modulePath + "/internal/repository/postgres",
			modulePath + "/internal/transport/http",
		},
		ForbiddenSubstrings: []string{
			"/enterprise",
			"/ldap",
			"/oidc",
			"/rbac",
			"advanced-rbac",
			"advanced_rbac",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "keep authorization as a concrete-type-neutral application boundary and wire advanced policy implementations outside OSS core",
	}

	assertNoBoundaryViolations(t, packages, rule)
}

func TestOSSCoreDoesNotImportEnterprisePackages(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	rules := []boundaryRule{
		ossEnterpriseBoundaryRule(modulePath, modulePath+"/internal"),
		ossEnterpriseBoundaryRule(modulePath, modulePath+"/cmd"),
	}
	for _, rule := range rules {
		assertNoBoundaryViolations(t, packages, rule)
	}
}

func TestConfigParsingStaysInConfigPackage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)

	assertPackageImports(t, packages, modulePath+"/internal/config", []string{"os", "time"})

	violations := scanForbiddenConfigCalls(t, repoRoot, modulePath, packages)
	if len(violations) == 0 {
		return
	}

	var message strings.Builder
	fmt.Fprintf(&message, "config parsing boundary: found %d forbidden raw config parsing call(s)", len(violations))
	for _, violation := range violations {
		fmt.Fprintf(
			&message,
			"\n- %s:%d calls %s; fix: %s",
			violation.File,
			violation.Line,
			violation.Call,
			violation.SuggestedFix,
		)
	}
	t.Fatal(message.String())
}

func TestBootstrapIsOnlyImportedByCompositionRoots(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	violations := checkForbiddenImportOutsideAllowed(
		packages,
		modulePath+"/internal/app/bootstrap",
		[]string{
			modulePath + "/cmd",
			modulePath + "/internal/app/bootstrap",
			modulePath + "/internal/app/runtime",
		},
		"depend on application ports instead of importing bootstrap wiring",
	)
	assertNoImportOwnershipViolations(t, "bootstrap imports must stay in composition roots", violations)
}

func TestForbiddenConcreteDependenciesStayOutOfCore(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packages, modulePath := loadPackageInventory(t, repoRoot)
	coreRules := []boundaryRule{
		forbiddenCoreDependencyRule(modulePath, modulePath+"/internal/domain"),
		forbiddenCoreDependencyRule(modulePath, modulePath+"/internal/usecase"),
		forbiddenCoreDependencyRule(modulePath, modulePath+"/internal/outbox"),
	}
	for _, rule := range coreRules {
		assertNoBoundaryViolations(t, packages, rule)
	}

	violations := checkForbiddenImportOutsideAllowed(
		packages,
		modulePath+"/internal/repository/postgres",
		[]string{
			modulePath + "/internal/app/bootstrap",
			modulePath + "/internal/repository/postgres",
		},
		"depend on repository ports and wire PostgreSQL implementations in bootstrap",
	)
	assertNoImportOwnershipViolations(t, "repository/postgres imports must stay in persistence adapter or bootstrap", violations)

	violations = checkForbiddenImportOutsideAllowed(
		packages,
		"github.com/pressly/goose",
		[]string{
			modulePath + "/internal/repository/postgres",
		},
		"keep Goose migration execution inside the PostgreSQL adapter",
	)
	assertNoImportOwnershipViolations(t, "Goose imports must stay in PostgreSQL migration infrastructure", violations)

	violations = checkForbiddenImportOutsideAllowed(
		packages,
		"github.com/testcontainers/testcontainers-go",
		nil,
		"keep Testcontainers usage in tests only",
	)
	assertNoImportOwnershipViolations(t, "Testcontainers imports must not appear in production code", violations)
}

func ossEnterpriseBoundaryRule(modulePath, packagePrefix string) boundaryRule {
	return boundaryRule{
		Name:          "OSS production packages must not import enterprise packages",
		PackagePrefix: packagePrefix,
		ForbiddenPrefixes: []string{
			modulePath + "/internal/enterprise",
		},
		ForbiddenSubstrings: []string{
			"/enterprise/",
			"enterprise/auth",
			"enterprise/license",
			"enterprise/rbac",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "depend on OSS extension interfaces and wire enterprise implementations only in downstream/private composition roots",
	}
}

func forbiddenCoreDependencyRule(modulePath, packagePrefix string) boundaryRule {
	return boundaryRule{
		Name:          "core production imports must not depend on concrete dependencies",
		PackagePrefix: packagePrefix,
		ForbiddenPrefixes: []string{
			modulePath + "/internal/infrastructure/bufcli",
			modulePath + "/internal/infrastructure/gitlabapi",
			modulePath + "/internal/infrastructure/kafka",
			modulePath + "/internal/infrastructure/objectstorage/s3",
			modulePath + "/internal/repository/postgres",
			modulePath + "/internal/transport/http",
			modulePath + "/internal/transport/web",
			"github.com/IBM/sarama",
			"github.com/Shopify/sarama",
			"github.com/bufbuild",
			"github.com/jackc/pgx",
			"github.com/minio",
			"github.com/pressly/goose",
			"github.com/xanzy/go-gitlab",
		},
		ForbiddenExactImports: []string{
			"database/sql",
		},
		ForbiddenSubstrings: []string{
			"/sarama",
			"gitlabapi",
			"kafka",
			"minio-go",
			"/enterprise",
			"protoradar-enterprise",
		},
		ImportSet:    productionImports,
		SuggestedFix: "depend on domain/application ports and keep concrete dependencies in infrastructure, repository, transport, or bootstrap",
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", wd)
		}
	}
}

func assertPackageImports(t *testing.T, packages []packageInfo, importPath string, imports []string) {
	t.Helper()

	for _, pkg := range packages {
		if pkg.ImportPath != importPath {
			continue
		}
		for _, required := range imports {
			if !containsString(pkg.Imports, required) {
				t.Fatalf("%s does not import %s; config parsing ownership test expected it to own that dependency", importPath, required)
			}
		}
		return
	}
	t.Fatalf("package inventory does not include %s", importPath)
}

func scanForbiddenConfigCalls(t *testing.T, repoRoot, modulePath string, packages []packageInfo) []sourceCallViolation {
	t.Helper()

	dirToImportPath := map[string]string{}
	for _, pkg := range packages {
		if pkg.Dir != "" {
			dirToImportPath[filepath.Clean(pkg.Dir)] = pkg.ImportPath
		}
	}

	var violations []sourceCallViolation
	for _, root := range []string{"internal", "cmd"} {
		walkRoot := filepath.Join(repoRoot, root)
		err := filepath.WalkDir(walkRoot, func(filePath string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || entry.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}

			packagePath := dirToImportPath[filepath.Clean(filepath.Dir(filePath))]
			if packagePath == "" {
				return nil
			}

			fileSet := token.NewFileSet()
			parsed, err := parser.ParseFile(fileSet, filePath, nil, parser.ImportsOnly)
			if err != nil {
				return fmt.Errorf("parse imports for %s: %w", filePath, err)
			}
			importNames := importNameByPath(parsed)

			parsed, err = parser.ParseFile(fileSet, filePath, nil, 0)
			if err != nil {
				return fmt.Errorf("parse %s: %w", filePath, err)
			}

			ast.Inspect(parsed, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				importPath := importNames[ident.Name]
				callName := importPath + "." + selector.Sel.Name

				switch callName {
				case "os.Getenv", "os.LookupEnv":
					if allowsEnvConfigCall(modulePath, packagePath) {
						return true
					}
					position := fileSet.Position(call.Pos())
					violations = append(violations, sourceCallViolation{
						File:         relativePath(repoRoot, filePath),
						Line:         position.Line,
						Call:         callName,
						SuggestedFix: "move application env parsing into internal/config or use typed config passed from bootstrap",
					})
				case "time.ParseDuration":
					if packagePath == modulePath+"/internal/config" {
						return true
					}
					position := fileSet.Position(call.Pos())
					violations = append(violations, sourceCallViolation{
						File:         relativePath(repoRoot, filePath),
						Line:         position.Line,
						Call:         callName,
						SuggestedFix: "parse raw duration strings in internal/config and pass typed time.Duration values outward",
					})
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s for config parsing calls: %v", walkRoot, err)
		}
	}
	return violations
}

func importNameByPath(file *ast.File) map[string]string {
	names := map[string]string{}
	for _, imported := range file.Imports {
		importPath := strings.Trim(imported.Path.Value, `"`)
		if imported.Name != nil {
			if imported.Name.Name == "." || imported.Name.Name == "_" {
				continue
			}
			names[imported.Name.Name] = importPath
			continue
		}
		names[path.Base(importPath)] = importPath
	}
	return names
}

func allowsEnvConfigCall(modulePath, packagePath string) bool {
	if packagePath == modulePath+"/internal/config" {
		return true
	}
	// CLI packages own client-side environment handling such as PROTORADAR_TOKEN,
	// PROTORADAR_SERVER_URL, and GitLab CI variables. Server config still belongs
	// to internal/config and must be passed into bootstrap as typed config.
	return matchesPackagePrefix(packagePath, modulePath+"/internal/cli")
}

func loadPackageInventory(t *testing.T, repoRoot string) ([]packageInfo, string) {
	t.Helper()

	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -json ./... failed from %s: %v\n%s", repoRoot, err, output)
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	var packages []packageInfo
	modulePath := ""
	for decoder.More() {
		var listed goListPackage
		if err := decoder.Decode(&listed); err != nil {
			t.Fatalf("decode go list package JSON: %v", err)
		}
		if listed.ImportPath == "" {
			continue
		}
		pkg := packageInfo{
			ImportPath:  listed.ImportPath,
			Dir:         listed.Dir,
			Name:        listed.Name,
			Imports:     append([]string(nil), listed.Imports...),
			TestImports: append(append([]string(nil), listed.TestImports...), listed.XTestImports...),
		}
		if listed.Module != nil {
			pkg.ModulePath = listed.Module.Path
			if listed.Module.Main && modulePath == "" {
				modulePath = listed.Module.Path
			}
		}
		packages = append(packages, pkg)
	}
	if modulePath == "" {
		t.Fatal("go list output did not include a main module path")
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].ImportPath < packages[j].ImportPath
	})
	return packages, modulePath
}

func assertNoBoundaryViolations(t *testing.T, packages []packageInfo, rule boundaryRule) {
	t.Helper()

	violations := checkBoundaryRule(packages, rule)
	if len(violations) == 0 {
		return
	}

	var message strings.Builder
	fmt.Fprintf(&message, "%s: found %d forbidden import(s)", rule.Name, len(violations))
	for _, violation := range violations {
		fmt.Fprintf(
			&message,
			"\n- package %s has forbidden %s import %s; fix: %s",
			violation.Package,
			violation.ImportKind,
			violation.Import,
			violation.SuggestedFix,
		)
	}
	t.Fatal(message.String())
}

func assertNoImportOwnershipViolations(t *testing.T, ruleName string, violations []boundaryViolation) {
	t.Helper()

	if len(violations) == 0 {
		return
	}

	var message strings.Builder
	fmt.Fprintf(&message, "%s: found %d forbidden import(s)", ruleName, len(violations))
	for _, violation := range violations {
		fmt.Fprintf(
			&message,
			"\n- package %s has forbidden %s import %s; fix: %s",
			violation.Package,
			violation.ImportKind,
			violation.Import,
			violation.SuggestedFix,
		)
	}
	t.Fatal(message.String())
}

func checkBoundaryRule(packages []packageInfo, rule boundaryRule) []boundaryViolation {
	var violations []boundaryViolation
	for _, pkg := range packages {
		if !matchesPackagePrefix(pkg.ImportPath, rule.PackagePrefix) {
			continue
		}
		for _, imported := range importsForRule(pkg, rule.ImportSet) {
			if isAllowedImport(pkg.ImportPath, imported.path, rule.Exceptions) {
				continue
			}
			if !isForbiddenImport(imported.path, rule) {
				continue
			}
			violations = append(violations, boundaryViolation{
				Rule:         rule.Name,
				Package:      pkg.ImportPath,
				Import:       imported.path,
				ImportKind:   imported.kind,
				SuggestedFix: rule.SuggestedFix,
			})
		}
	}
	return violations
}

func checkForbiddenImportOutsideAllowed(packages []packageInfo, forbiddenImport string, allowedPackagePrefixes []string, suggestedFix string) []boundaryViolation {
	var violations []boundaryViolation
	for _, pkg := range packages {
		if packageMatchesAnyPrefix(pkg.ImportPath, allowedPackagePrefixes) {
			continue
		}
		for _, imported := range pkg.Imports {
			if !matchesPackagePrefix(imported, forbiddenImport) {
				continue
			}
			violations = append(violations, boundaryViolation{
				Package:      pkg.ImportPath,
				Import:       imported,
				ImportKind:   "production",
				SuggestedFix: suggestedFix,
			})
		}
	}
	return violations
}

type checkedImport struct {
	path string
	kind string
}

func importsForRule(pkg packageInfo, set importSet) []checkedImport {
	var imports []checkedImport
	if set == productionImports || set == allImports {
		for _, imported := range pkg.Imports {
			imports = append(imports, checkedImport{path: imported, kind: "production"})
		}
	}
	if set == testImports || set == allImports {
		for _, imported := range pkg.TestImports {
			imports = append(imports, checkedImport{path: imported, kind: "test"})
		}
	}
	return imports
}

func isForbiddenImport(imported string, rule boundaryRule) bool {
	for _, forbidden := range rule.ForbiddenExactImports {
		if imported == forbidden {
			return true
		}
	}
	for _, forbidden := range rule.ForbiddenPrefixes {
		if matchesPackagePrefix(imported, forbidden) {
			return true
		}
	}
	for _, forbidden := range rule.ForbiddenSubstrings {
		if strings.Contains(strings.ToLower(imported), strings.ToLower(forbidden)) {
			return true
		}
	}
	return false
}

func matchesPackagePrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func packageMatchesAnyPrefix(packagePath string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if matchesPackagePrefix(packagePath, prefix) {
			return true
		}
	}
	return false
}

func isAllowedImport(packagePath, importPath string, exceptions []allowedImport) bool {
	for _, exception := range exceptions {
		if packagePath == exception.Package && importPath == exception.Import {
			return true
		}
	}
	return false
}

func hasPackage(packages []packageInfo, importPath string) bool {
	for _, pkg := range packages {
		if pkg.ImportPath == importPath {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func relativePath(base, target string) string {
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return relative
}
