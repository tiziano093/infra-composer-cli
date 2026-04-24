package registry

import (
	"sort"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

func TestTranslateSchema_FlatAttributes(t *testing.T) {
	raw := &tfjson.Schema{Block: &tfjson.SchemaBlock{
		Description: "Virtual Private Cloud",
		Attributes: map[string]*tfjson.SchemaAttribute{
			"cidr_block": {
				AttributeType: cty.String, Required: true,
				Description: "CIDR block",
			},
			"id": {
				AttributeType: cty.String, Computed: true,
			},
			"tags": {
				AttributeType: cty.Map(cty.String), Optional: true,
			},
			"secret_token": {
				AttributeType: cty.String, Optional: true, Sensitive: true,
			},
		},
	}}
	rs := translateSchema("aws_vpc", KindResource, raw, "https://registry/x")
	if rs.Description != "Virtual Private Cloud" {
		t.Fatalf("description: %q", rs.Description)
	}
	if rs.Source != "https://registry/x" {
		t.Fatalf("source: %q", rs.Source)
	}
	if rs.Group != "network" {
		t.Fatalf("inferGroup: %q", rs.Group)
	}

	gotInputs := indexInputs(rs.Inputs)
	if _, ok := gotInputs["id"]; ok {
		t.Errorf("computed-only attribute should not be input: id")
	}
	if in := gotInputs["cidr_block"]; !in.Required || in.Type != "string" {
		t.Errorf("cidr_block: %+v", in)
	}
	if in := gotInputs["secret_token"]; !in.Sensitive {
		t.Errorf("secret_token sensitive lost: %+v", in)
	}
	if in := gotInputs["tags"]; in.Type != "map(string)" {
		t.Errorf("tags type: %q", in.Type)
	}

	gotOutputs := indexOutputs(rs.Outputs)
	if _, ok := gotOutputs["id"]; !ok {
		t.Error("id missing from outputs")
	}
	if _, ok := gotOutputs["cidr_block"]; ok {
		t.Error("required attribute leaked into outputs")
	}
}

func TestTranslateSchema_NestedBlocks(t *testing.T) {
	raw := &tfjson.Schema{Block: &tfjson.SchemaBlock{
		Attributes: map[string]*tfjson.SchemaAttribute{
			"name": {AttributeType: cty.String, Required: true},
		},
		NestedBlocks: map[string]*tfjson.SchemaBlockType{
			"ingress": {
				NestingMode: tfjson.SchemaNestingModeList,
				MinItems:    1,
				Block: &tfjson.SchemaBlock{
					Description: "ingress rules",
					Attributes: map[string]*tfjson.SchemaAttribute{
						"port": {AttributeType: cty.Number, Required: true},
					},
				},
			},
			"settings": {
				NestingMode: tfjson.SchemaNestingModeSingle,
				Block: &tfjson.SchemaBlock{
					Attributes: map[string]*tfjson.SchemaAttribute{
						"enabled": {AttributeType: cty.Bool, Optional: true},
					},
				},
			},
		},
	}}
	rs := translateSchema("aws_security_group", KindResource, raw, "")

	ins := indexInputs(rs.Inputs)
	if in, ok := ins["ingress"]; !ok || in.Type != "list(any)" || !in.Required {
		t.Errorf("ingress nested block: %+v ok=%v", in, ok)
	}
	if in, ok := ins["settings"]; !ok || in.Type != "any" || in.Required {
		t.Errorf("settings nested block: %+v ok=%v", in, ok)
	}
	if _, ok := ins["ingress.port"]; ok {
		t.Error("nested attribute should not appear flattened (current contract)")
	}

	// Child attrs must be collected on the parent InputSpec.
	ingress := ins["ingress"]
	if len(ingress.Attrs) != 1 || ingress.Attrs[0].Name != "port" || ingress.Attrs[0].Type != "number" || !ingress.Attrs[0].Required {
		t.Errorf("ingress.Attrs: %+v", ingress.Attrs)
	}
	settings := ins["settings"]
	if len(settings.Attrs) != 1 || settings.Attrs[0].Name != "enabled" || settings.Attrs[0].Type != "bool" {
		t.Errorf("settings.Attrs: %+v", settings.Attrs)
	}
}

func TestTranslateSchema_OutputsCapturesComputedNested(t *testing.T) {
	raw := &tfjson.Schema{Block: &tfjson.SchemaBlock{
		Attributes: map[string]*tfjson.SchemaAttribute{
			"id": {AttributeType: cty.String, Computed: true},
		},
		NestedBlocks: map[string]*tfjson.SchemaBlockType{
			"network_interface": {
				NestingMode: tfjson.SchemaNestingModeList,
				Block: &tfjson.SchemaBlock{
					Attributes: map[string]*tfjson.SchemaAttribute{
						"private_ip": {AttributeType: cty.String, Computed: true},
					},
				},
			},
		},
	}}
	rs := translateSchema("aws_instance", KindResource, raw, "")

	outs := indexOutputs(rs.Outputs)
	if _, ok := outs["id"]; !ok {
		t.Error("id output missing")
	}
	if _, ok := outs["network_interface.private_ip"]; !ok {
		t.Errorf("nested computed output missing; got %v", outs)
	}
}

func TestTranslateSchema_NilBlockSafe(t *testing.T) {
	rs := translateSchema("foo", KindResource, &tfjson.Schema{}, "")
	if rs.Name != "foo" || rs.Inputs != nil || rs.Outputs != nil {
		t.Fatalf("nil-block translation unexpected: %+v", rs)
	}
}

func TestInferGroup(t *testing.T) {
	cases := map[string]string{
		"aws_vpc":            "network",
		"aws_subnet":         "network",
		"aws_instance":       "compute",
		"aws_s3_bucket":      "storage",
		"aws_iam_role":       "iam",
		"aws_cloudwatch_log": "observability",
		"random_string":      "",
	}
	for name, want := range cases {
		if got := inferGroup(name); got != want {
			t.Errorf("inferGroup(%q)=%q want %q", name, got, want)
		}
	}
}

func indexInputs(in []InputSpec) map[string]InputSpec {
	out := make(map[string]InputSpec, len(in))
	for _, x := range in {
		out[x.Name] = x
	}
	return out
}

func indexOutputs(in []OutputSpec) map[string]OutputSpec {
	out := make(map[string]OutputSpec, len(in))
	for _, x := range in {
		out[x.Name] = x
	}
	return out
}

func TestInputs_OutputsAreSorted(t *testing.T) {
	raw := &tfjson.Schema{Block: &tfjson.SchemaBlock{
		Attributes: map[string]*tfjson.SchemaAttribute{
			"zeta":  {AttributeType: cty.String, Required: true},
			"alpha": {AttributeType: cty.String, Required: true},
			"out_z": {AttributeType: cty.String, Computed: true},
			"out_a": {AttributeType: cty.String, Computed: true},
		},
	}}
	rs := translateSchema("foo", KindResource, raw, "")
	if !sort.SliceIsSorted(rs.Inputs, func(i, j int) bool { return rs.Inputs[i].Name < rs.Inputs[j].Name }) {
		t.Errorf("inputs not sorted: %+v", rs.Inputs)
	}
	if !sort.SliceIsSorted(rs.Outputs, func(i, j int) bool { return rs.Outputs[i].Name < rs.Outputs[j].Name }) {
		t.Errorf("outputs not sorted: %+v", rs.Outputs)
	}
}
