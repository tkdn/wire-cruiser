// Package render turns a graph.Injector into human-readable output.
package render

import (
	"strings"

	"github.com/tkdn/wire-cruiser/internal/graph"
)

// Tree renders the injector's dependency graph as an indented tree.
// Repeated occurrences of a shared node print without children,
// marked with " (…)".
func Tree(inj *graph.Injector) string {
	var sb strings.Builder
	sb.WriteString(inj.PkgName)
	sb.WriteString(".")
	sb.WriteString(inj.Name)
	sb.WriteString(" → ")
	sb.WriteString(graph.TypeString(inj.Result))
	sb.WriteString("\n")
	writeNodes(&sb, inj.Roots, "", map[*graph.Node]bool{})
	return sb.String()
}

func writeNodes(sb *strings.Builder, nodes []*graph.Node, prefix string, seen map[*graph.Node]bool) {
	for i, n := range nodes {
		connector, childPrefix := "├─ ", prefix+"│  "
		if i == len(nodes)-1 {
			connector, childPrefix = "└─ ", prefix+"   "
		}
		sb.WriteString(prefix)
		sb.WriteString(connector)
		sb.WriteString(label(n))
		if seen[n] && len(n.Deps) > 0 {
			sb.WriteString(" (…)\n")
			continue
		}
		seen[n] = true
		sb.WriteString("\n")
		writeNodes(sb, n.Deps, childPrefix, seen)
	}
}

func label(n *graph.Node) string {
	switch n.Kind {
	case graph.KindArg:
		return "(arg) " + graph.TypeString(n.Type)
	case graph.KindMissing:
		return "[MISSING] " + graph.TypeString(n.Type)
	default:
		s := n.Provider + " → " + graph.TypeString(n.Type)
		if n.Bind != nil {
			s += " (bind: " + graph.TypeString(n.Bind) + ")"
		}
		return s
	}
}
