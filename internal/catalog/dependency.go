package catalog

import (
	"errors"
	"fmt"
	"sort"
)

// Edge is a single dependency edge in the catalog graph: module From
// declares a Variable that references module To's Output.
type Edge struct {
	From     string
	To       string
	Variable string
	Output   string
}

// Graph is the directed dependency graph of a catalog. An edge From → To
// means "From depends on (consumes an output of) To". The graph is
// constructed from VariableReference declarations; modules with no
// outgoing or incoming edges are still represented as nodes.
type Graph struct {
	// nodes is the sorted list of every module name that appears in the
	// source schema, even if it has no edges.
	nodes []string
	// out maps a module to its outgoing edges, sorted deterministically
	// (by To, then Variable, then Output).
	out map[string][]Edge
}

// ErrUnknownModule is returned by Graph.Resolve when the requested root
// is not part of the schema the graph was built from.
var ErrUnknownModule = errors.New("dependency: unknown module")

// CycleError signals that a dependency cycle was detected during
// traversal. Cycle is the ordered list of module names forming the
// cycle, with the first element repeated implicitly at the end:
// e.g. ["a", "b", "c"] means a → b → c → a.
type CycleError struct {
	Cycle []string
}

// Error implements the error interface.
func (e *CycleError) Error() string {
	if len(e.Cycle) == 0 {
		return "dependency cycle detected"
	}
	return "dependency cycle detected: " + cycleString(e.Cycle)
}

func cycleString(c []string) string {
	out := ""
	for i, n := range c {
		if i > 0 {
			out += " → "
		}
		out += n
	}
	out += " → " + c[0]
	return out
}

// BuildGraph constructs a Graph from the validated schema s. The schema
// is expected to have already passed Schema.Validate; BuildGraph does
// not re-validate references.
//
// BuildGraph never fails: if s is nil it returns an empty Graph.
func BuildGraph(s *Schema) *Graph {
	g := &Graph{out: make(map[string][]Edge)}
	if s == nil {
		return g
	}
	known := make(map[string]struct{}, len(s.Modules))
	for _, m := range s.Modules {
		if m.Name == "" {
			continue
		}
		known[m.Name] = struct{}{}
	}
	for _, m := range s.Modules {
		if m.Name == "" {
			continue
		}
		for _, v := range m.Variables {
			for _, ref := range v.References {
				if ref.Module == "" || ref.Output == "" {
					continue
				}
				if _, ok := known[ref.Module]; !ok {
					continue
				}
				if ref.Module == m.Name {
					continue
				}
				g.out[m.Name] = append(g.out[m.Name], Edge{
					From: m.Name, To: ref.Module,
					Variable: v.Name, Output: ref.Output,
				})
			}
		}
	}
	g.nodes = make([]string, 0, len(known))
	for n := range known {
		g.nodes = append(g.nodes, n)
	}
	sort.Strings(g.nodes)
	for k := range g.out {
		sort.SliceStable(g.out[k], func(i, j int) bool {
			a, b := g.out[k][i], g.out[k][j]
			if a.To != b.To {
				return a.To < b.To
			}
			if a.Variable != b.Variable {
				return a.Variable < b.Variable
			}
			return a.Output < b.Output
		})
	}
	return g
}

// Modules returns every module name in the graph in deterministic order.
func (g *Graph) Modules() []string {
	if g == nil {
		return nil
	}
	out := make([]string, len(g.nodes))
	copy(out, g.nodes)
	return out
}

// Has reports whether the named module is part of the graph.
func (g *Graph) Has(name string) bool {
	if g == nil {
		return false
	}
	for _, n := range g.nodes {
		if n == name {
			return true
		}
	}
	return false
}

// Edges returns the outgoing edges of from in deterministic order.
// The slice is freshly allocated and safe to mutate by callers.
func (g *Graph) Edges(from string) []Edge {
	if g == nil {
		return nil
	}
	src := g.out[from]
	out := make([]Edge, len(src))
	copy(out, src)
	return out
}

// Cycles returns every elementary cycle in the graph. Each cycle is the
// ordered list of nodes participating in it (starting from the smallest
// node name to keep results deterministic). An empty result means the
// graph is a DAG.
//
// The implementation is a standard 3-colour DFS that records the active
// stack and walks back to the cycle start when a grey node is hit. It is
// O(V + E) in the worst case; the catalog dimension makes this trivially
// fast in practice.
func (g *Graph) Cycles() [][]string {
	if g == nil {
		return nil
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	colour := make(map[string]int, len(g.nodes))
	stack := make([]string, 0, len(g.nodes))
	onStack := make(map[string]int, len(g.nodes))
	seenCycle := make(map[string]struct{})
	var cycles [][]string

	var visit func(n string)
	visit = func(n string) {
		colour[n] = grey
		onStack[n] = len(stack)
		stack = append(stack, n)
		for _, e := range g.out[n] {
			switch colour[e.To] {
			case white:
				visit(e.To)
			case grey:
				start := onStack[e.To]
				cyc := append([]string(nil), stack[start:]...)
				key := cycleKey(cyc)
				if _, dup := seenCycle[key]; !dup {
					seenCycle[key] = struct{}{}
					cycles = append(cycles, normaliseCycle(cyc))
				}
			}
		}
		stack = stack[:len(stack)-1]
		delete(onStack, n)
		colour[n] = black
	}

	for _, n := range g.nodes {
		if colour[n] == white {
			visit(n)
		}
	}
	sort.SliceStable(cycles, func(i, j int) bool {
		return cycleKey(cycles[i]) < cycleKey(cycles[j])
	})
	return cycles
}

// normaliseCycle rotates c so it begins with its lexicographically
// smallest member, producing a stable canonical form.
func normaliseCycle(c []string) []string {
	if len(c) == 0 {
		return c
	}
	min := 0
	for i := 1; i < len(c); i++ {
		if c[i] < c[min] {
			min = i
		}
	}
	out := make([]string, 0, len(c))
	out = append(out, c[min:]...)
	out = append(out, c[:min]...)
	return out
}

func cycleKey(c []string) string {
	n := normaliseCycle(c)
	out := ""
	for i, s := range n {
		if i > 0 {
			out += "|"
		}
		out += s
	}
	return out
}

// DependencyNode is one node in a resolved dependency tree returned by
// Graph.Resolve. Children are sorted deterministically (matching the
// underlying edge order). EdgeFromParent is the edge that reached this
// node from its parent (zero value at the root).
type DependencyNode struct {
	Module         string
	Depth          int
	EdgeFromParent Edge
	Children       []DependencyNode
}

// Resolve walks the graph starting from root and returns the dependency
// tree truncated to maxDepth (0 means unlimited). Cycles abort the walk
// with a *CycleError; callers that want to enumerate cycles up-front
// should call Cycles first.
//
// When the same dependency is reached by multiple parents it is expanded
// every time it appears (a tree, not a DAG view) so that compose-side
// consumers can wire each call site explicitly. Use Cycles to detect
// pathological structures before calling Resolve in unbounded mode.
func (g *Graph) Resolve(root string, maxDepth int) (*DependencyNode, error) {
	if g == nil || !g.Has(root) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownModule, root)
	}
	visiting := make(map[string]struct{})
	stack := []string{}
	node, err := g.resolve(root, Edge{}, 0, maxDepth, visiting, &stack)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (g *Graph) resolve(name string, edge Edge, depth, maxDepth int, visiting map[string]struct{}, stack *[]string) (*DependencyNode, error) {
	if _, on := visiting[name]; on {
		// Build cycle path: slice from first occurrence of name in stack.
		idx := -1
		for i, s := range *stack {
			if s == name {
				idx = i
				break
			}
		}
		var cyc []string
		if idx >= 0 {
			cyc = append(cyc, (*stack)[idx:]...)
		} else {
			cyc = []string{name}
		}
		return nil, &CycleError{Cycle: normaliseCycle(cyc)}
	}
	node := &DependencyNode{Module: name, Depth: depth, EdgeFromParent: edge}
	if maxDepth > 0 && depth >= maxDepth {
		return node, nil
	}
	visiting[name] = struct{}{}
	*stack = append(*stack, name)
	defer func() {
		delete(visiting, name)
		*stack = (*stack)[:len(*stack)-1]
	}()
	for _, e := range g.out[name] {
		child, err := g.resolve(e.To, e, depth+1, maxDepth, visiting, stack)
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, *child)
	}
	return node, nil
}
