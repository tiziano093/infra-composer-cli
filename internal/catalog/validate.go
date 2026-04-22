package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// providerRe matches the canonical Terraform provider address form
// "<namespace>/<name>" using the same character class the Registry
// accepts: lowercase letters, digits, dashes and underscores.
var providerRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*\/[a-z0-9][a-z0-9_-]*$`)

// semverRe is a permissive semver matcher. We only need to reject
// obviously bogus versions ("", "latest", "v 1.2"), not enforce full
// SemVer 2.0.0 grammar.
var semverRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.\-]+)?$`)

// identRe matches Terraform identifiers (variable, output and module
// names): start with a letter or underscore, then letters, digits,
// underscores or dashes.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// Validate checks structural and semantic invariants of the Schema and
// returns a *ValidationError listing every issue found, or nil when the
// schema is valid.
//
// All issues are reported in a single pass so users can fix them in one
// edit; Validate does not stop at the first problem.
func (s *Schema) Validate() error {
	if s == nil {
		return &ValidationError{Issues: []ValidationIssue{{Message: "schema is nil"}}}
	}
	v := newValidator()

	switch s.SchemaVersion {
	case "":
		v.add("schema_version", "is required")
	case SchemaVersion:
		// ok
	default:
		v.addf("schema_version", "unsupported version %q (expected %q)", s.SchemaVersion, SchemaVersion)
	}

	switch {
	case s.Provider == "":
		v.add("provider", "is required")
	case !providerRe.MatchString(s.Provider):
		v.addf("provider", "must match \"<namespace>/<name>\" (got %q)", s.Provider)
	}

	switch {
	case s.ProviderVersion == "":
		v.add("provider_version", "is required")
	case !semverRe.MatchString(s.ProviderVersion):
		v.addf("provider_version", "must be semver (got %q)", s.ProviderVersion)
	}

	seenModule := make(map[string]int, len(s.Modules))
	// Pre-index output names per module so cross-module references can
	// be resolved in the same single pass without revisiting modules.
	outputsByModule := make(map[string]map[string]struct{}, len(s.Modules))
	for _, m := range s.Modules {
		if m.Name == "" || !identRe.MatchString(m.Name) {
			continue
		}
		if _, dup := outputsByModule[m.Name]; dup {
			continue
		}
		set := make(map[string]struct{}, len(m.Outputs))
		for _, o := range m.Outputs {
			if o.Name != "" {
				set[o.Name] = struct{}{}
			}
		}
		outputsByModule[m.Name] = set
	}
	for i, m := range s.Modules {
		base := fmt.Sprintf("modules[%d]", i)
		validateModule(v, base, m, seenModule, i, outputsByModule)
	}

	if len(v.issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: v.issues}
}

func validateModule(v *validator, base string, m ModuleEntry, seen map[string]int, idx int, outputsByModule map[string]map[string]struct{}) {
	switch {
	case m.Name == "":
		v.add(base+".name", "is required")
	case !identRe.MatchString(m.Name):
		v.addf(base+".name", "invalid identifier %q", m.Name)
	default:
		if prev, dup := seen[m.Name]; dup {
			v.addf(base+".name", "duplicate module name %q (also at modules[%d])", m.Name, prev)
		} else {
			seen[m.Name] = idx
		}
	}

	switch {
	case m.Type == "":
		v.add(base+".type", "is required")
	case !m.Type.IsValid():
		v.addf(base+".type", "must be \"resource\" or \"data\" (got %q)", string(m.Type))
	}

	seenVar := make(map[string]struct{}, len(m.Variables))
	for j, vv := range m.Variables {
		vbase := fmt.Sprintf("%s.variables[%d]", base, j)
		validateVariable(v, vbase, vv, seenVar, m.Name, outputsByModule)
	}

	seenOut := make(map[string]struct{}, len(m.Outputs))
	for j, o := range m.Outputs {
		obase := fmt.Sprintf("%s.outputs[%d]", base, j)
		validateOutput(v, obase, o, seenOut)
	}
}

func validateVariable(v *validator, base string, x Variable, seen map[string]struct{}, moduleName string, outputsByModule map[string]map[string]struct{}) {
	switch {
	case x.Name == "":
		v.add(base+".name", "is required")
	case !identRe.MatchString(x.Name):
		v.addf(base+".name", "invalid identifier %q", x.Name)
	default:
		if _, dup := seen[x.Name]; dup {
			v.addf(base+".name", "duplicate variable name %q", x.Name)
		} else {
			seen[x.Name] = struct{}{}
		}
	}

	if strings.TrimSpace(x.Type) == "" {
		v.add(base+".type", "is required")
	}

	if x.Required && x.Default != nil {
		v.add(base+".default", "must be omitted when required is true")
	}

	for k, ref := range x.References {
		rbase := fmt.Sprintf("%s.references[%d]", base, k)
		validateReference(v, rbase, ref, moduleName, outputsByModule)
	}
}

func validateReference(v *validator, base string, ref VariableReference, owner string, outputsByModule map[string]map[string]struct{}) {
	switch {
	case ref.Module == "":
		v.add(base+".module", "is required")
	case !identRe.MatchString(ref.Module):
		v.addf(base+".module", "invalid identifier %q", ref.Module)
	case owner != "" && ref.Module == owner:
		v.addf(base+".module", "self-reference to %q is not allowed", ref.Module)
	}

	switch {
	case ref.Output == "":
		v.add(base+".output", "is required")
	case !identRe.MatchString(ref.Output):
		v.addf(base+".output", "invalid identifier %q", ref.Output)
	}

	// Only attempt cross-checks when both fields look syntactically sound
	// and we are not already pointing at the owner module.
	if ref.Module == "" || ref.Output == "" || !identRe.MatchString(ref.Module) || !identRe.MatchString(ref.Output) {
		return
	}
	if owner != "" && ref.Module == owner {
		return
	}
	outs, ok := outputsByModule[ref.Module]
	if !ok {
		v.addf(base+".module", "unknown module %q", ref.Module)
		return
	}
	if _, ok := outs[ref.Output]; !ok {
		v.addf(base+".output", "module %q has no output %q", ref.Module, ref.Output)
	}
}

func validateOutput(v *validator, base string, o Output, seen map[string]struct{}) {
	switch {
	case o.Name == "":
		v.add(base+".name", "is required")
	case !identRe.MatchString(o.Name):
		v.addf(base+".name", "invalid identifier %q", o.Name)
	default:
		if _, dup := seen[o.Name]; dup {
			v.addf(base+".name", "duplicate output name %q", o.Name)
		} else {
			seen[o.Name] = struct{}{}
		}
	}
}

// validator is a tiny accumulator kept package-private to keep the
// public API focused on the data types.
type validator struct {
	issues []ValidationIssue
}

func newValidator() *validator { return &validator{} }

func (v *validator) add(field, msg string) {
	v.issues = append(v.issues, ValidationIssue{Field: field, Message: msg})
}

func (v *validator) addf(field, format string, args ...any) {
	v.issues = append(v.issues, ValidationIssue{Field: field, Message: fmt.Sprintf(format, args...)})
}
