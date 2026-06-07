package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
)

func (app App) output() io.Writer {
	if app.Out == nil {
		return io.Discard
	}
	return app.Out
}
func printModuleDependencies(w io.Writer, graph api.ModuleDependencyGraph) {
	fmt.Fprintf(w, "Module: %s\n\n", graph.Module)
	fmt.Fprintln(w, "Downstream consumers:")
	printDependencyModules(w, graph.Downstream, "No downstream modules are currently known to depend on this module.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Upstream dependencies:")
	printDependencyModules(w, graph.Upstream, "No upstream dependencies are currently known for this module.")
	if len(graph.Unresolved) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Unresolved dependencies:")
		for _, dependency := range graph.Unresolved {
			target := strings.TrimSpace(dependency.ImportPath)
			if target == "" {
				target = strings.TrimSpace(dependency.ReferencedSymbol)
			}
			if target == "" {
				target = "(unknown dependency)"
			}
			fmt.Fprintf(w, "- %s\n", target)
			writeOutputLine(w, "  reason", dependency.Reason)
		}
	}
}

func printAffectedModules(w io.Writer, affected api.AffectedModules) {
	fmt.Fprintf(w, "Module: %s\n\n", affected.Module)
	fmt.Fprintln(w, "Affected modules:")
	printDependencyModules(w, affected.AffectedModules, "No downstream modules are currently known to depend on this module.")
}

func printDependencyModules(w io.Writer, modules []api.DependencyModule, emptyMessage string) {
	if len(modules) == 0 {
		fmt.Fprintf(w, "%s\n", emptyMessage)
		return
	}
	for _, module := range modules {
		version := strings.TrimSpace(module.LatestVersion)
		if version == "" {
			version = "unknown"
		}
		fmt.Fprintf(w, "- %s@%s\n", module.Module, version)
		if len(module.DependencySources) > 0 {
			fmt.Fprintf(w, "  sources: %s\n", strings.Join(module.DependencySources, ", "))
		}
		for _, reason := range module.Reasons {
			writeOutputLine(w, "  reason", reason)
		}
	}
}

func writeOutputLine(w io.Writer, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(w, "%s: %s\n", label, value)
}

func writeFallbackLine(builder *strings.Builder, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(builder, "   %s: %s\n", label, value)
}
