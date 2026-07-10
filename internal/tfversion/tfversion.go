// Package tfversion detects the aliyun/alicloud provider version constraint in
// a Terraform config and decides whether the v1 -> v2 breaking-change check is
// in scope. The check applies when the constraint allows v1 or v2; if a config
// is already pinned to v3+, the v1->v2 changes are moot.
package tfversion

import (
	"os"
	"strings"

	goversion "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Constraint is one detected alicloud provider version constraint.
type Constraint struct {
	File string
	Line int
	Raw  string
}

// Verdict summarizes applicability across all detected constraints.
type Verdict struct {
	Constraints []Constraint
	// AppliesV2 is true if any constraint allows a v1 or v2 version (in scope).
	AppliesV2 bool
	// OnlyV3Plus is true if constraints were found and all of them allow only
	// v3+ (out of scope for the v1->v2 upgrade check).
	OnlyV3Plus bool
}

// Detect scans the given files for aliyun/alicloud version constraints and
// returns a verdict. Best-effort: unparseable files are skipped.
func Detect(files []string) Verdict {
	var v Verdict
	sawV3 := false
	for _, f := range files {
		for _, c := range detectFile(f) {
			v.Constraints = append(v.Constraints, c)
			a, only3 := classify(c.Raw)
			if a {
				v.AppliesV2 = true
			}
			if only3 {
				sawV3 = true
			}
		}
	}
	v.OnlyV3Plus = sawV3 && !v.AppliesV2
	return v
}

func detectFile(path string) []Constraint {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	f, diag := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diag.HasErrors() {
		return nil
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	var out []Constraint
	for _, blk := range body.Blocks {
		switch blk.Type {
		case "terraform":
			for _, inner := range blk.Body.Blocks {
				if inner.Type != "required_providers" {
					continue
				}
				if attr, ok := inner.Body.Attributes["alicloud"]; ok {
					if raw, line, ok := versionFromObject(attr); ok {
						out = append(out, Constraint{File: path, Line: line, Raw: raw})
					}
				}
			}
		case "provider":
			// legacy: provider "alicloud" { version = "..." }
			if len(blk.Labels) == 1 && blk.Labels[0] == "alicloud" {
				if attr, ok := blk.Body.Attributes["version"]; ok {
					if v, d := attr.Expr.Value(nil); !d.HasErrors() && v.Type() == cty.String {
						out = append(out, Constraint{File: path, Line: attr.SrcRange.Start.Line, Raw: v.AsString()})
					}
				}
			}
		}
	}
	return out
}

// versionFromObject reads {source=..., version=...} for the alicloud provider.
func versionFromObject(attr *hclsyntax.Attribute) (string, int, bool) {
	v, d := attr.Expr.Value(nil)
	if d.HasErrors() || !v.Type().IsObjectType() {
		return "", 0, false
	}
	if v.Type().HasAttribute("source") {
		src := v.GetAttr("source")
		if src.Type() == cty.String {
			s := strings.ToLower(src.AsString())
			if s != "aliyun/alicloud" && !strings.HasSuffix(s, "/aliyun/alicloud") {
				return "", 0, false
			}
		}
	}
	if !v.Type().HasAttribute("version") {
		return "", 0, false
	}
	ver := v.GetAttr("version")
	if ver.Type() != cty.String {
		return "", 0, false
	}
	return ver.AsString(), attr.SrcRange.Start.Line, true
}

// classify returns (appliesV2, onlyV3Plus) for a raw constraint string.
func classify(raw string) (appliesV2, onlyV3Plus bool) {
	c, err := goversion.NewConstraint(raw)
	if err != nil {
		return true, false // can't parse -> assume in scope, don't skip
	}
	allows := func(s string) bool {
		v, e := goversion.NewVersion(s)
		return e == nil && c.Check(v)
	}
	v1or2 := allows("1.0.0") || allows("1.999.0") || allows("2.0.0") || allows("2.999.0")
	v3plus := allows("3.0.0") || allows("3.999.0") || allows("4.0.0")
	return v1or2, v3plus && !v1or2
}
