package scanner

import (
	"errors"
	"os"
	"strings"

	"github.com/aliyun/alicloud-v2-check/internal/rules"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ErrParse is returned by ScanFileHCL when the file cannot be parsed as HCL.
// Callers may fall back to the regex scanner.
var ErrParse = errors.New("hcl parse error")

// ScanFileHCL scans a single .tf file using the official HashiCorp HCL parser.
// It builds the AST and filters for v2 breaking-change patterns, which avoids
// the false positives a line-based regex can hit (heredoc/string literals,
// multi-line blocks). Returns ErrParse if the file is not valid HCL.
func ScanFileHCL(path string) ([]Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, diag := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diag.HasErrors() {
		return nil, ErrParse
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, ErrParse
	}
	lines := strings.Split(string(src), "\n")
	var findings []Finding
	walkBody(body, "", path, lines, &findings)
	return findings, nil
}

func codeLine(lines []string, n int) string {
	if n >= 1 && n <= len(lines) {
		return lines[n-1]
	}
	return ""
}

// walkBody recurses the AST. enclosing is the affected resource/data type of
// the enclosing block, if any (used for ARG confidence).
func walkBody(body *hclsyntax.Body, enclosing, path string, lines []string, out *[]Finding) {
	for name, attr := range body.Attributes {
		// [ARG] a map-assign argument (parsed as an attribute, not a block).
		if rules.IsBlockArg(name) {
			conf := Medium
			if rules.IsAffectedType(enclosing) {
				conf = High
			}
			ln := attr.NameRange.Start.Line
			*out = append(*out, Finding{
				File: path, Line: ln, Category: ARG, Target: enclosing,
				Attr: name, Confidence: conf, Code: codeLine(lines, ln),
			})
		}
		// [REF] scan the expression for `.attr["key"]` traversals.
		scanExprRef(attr.Expr, path, lines, out)
	}

	for _, blk := range body.Blocks {
		newEnclosing := enclosing
		switch blk.Type {
		case "resource":
			if len(blk.Labels) > 0 {
				if attrs, ok := rules.AffectedResources[blk.Labels[0]]; ok {
					newEnclosing = blk.Labels[0]
					*out = append(*out, presentHCL(path, lines, blk, attrs, "resource"))
				}
			}
		case "data":
			if len(blk.Labels) > 0 {
				if attrs, ok := rules.AffectedDataSources[blk.Labels[0]]; ok {
					newEnclosing = blk.Labels[0]
					*out = append(*out, presentHCL(path, lines, blk, attrs, "data source"))
				}
			}
		case "module":
			if src, ok := blk.Body.Attributes["source"]; ok {
				if v, d := src.Expr.Value(nil); !d.HasErrors() && v.Type() == cty.String {
					base := strings.TrimRight(strings.SplitN(v.AsString(), "//", 2)[0], "/")
					if rules.IsAffectedModule(base) {
						ln := src.SrcRange.Start.Line
						*out = append(*out, Finding{
							File: path, Line: ln, Category: MODULE, Target: base,
							Confidence: High, Code: codeLine(lines, ln),
						})
					}
				}
			}
		}
		walkBody(blk.Body, newEnclosing, path, lines, out)
	}
}

func presentHCL(path string, lines []string, blk *hclsyntax.Block, attrs []string, _ string) Finding {
	ln := blk.TypeRange.Start.Line
	return Finding{
		File: path, Line: ln, Category: PRESENT, Target: blk.Labels[0],
		Attr: strings.Join(attrs, ", "), Confidence: High, Code: codeLine(lines, ln),
	}
}

// scanExprRef finds `.attr["key"]` references (attr changed TypeMap->TypeList).
// Because HCL folds a literal string index into the traversal, we scan each
// variable traversal for an affected TraverseAttr immediately followed by a
// string TraverseIndex. String/heredoc literals never appear as variables, so
// they cannot produce a false positive.
func scanExprRef(expr hclsyntax.Expression, path string, lines []string, out *[]Finding) {
	for _, tr := range expr.Variables() {
		for i := 1; i < len(tr); i++ {
			idx, ok := tr[i].(hcl.TraverseIndex)
			if !ok || idx.Key.Type() != cty.String {
				continue
			}
			attrStep, ok := tr[i-1].(hcl.TraverseAttr)
			if !ok || !rules.IsMapIndexAttr(attrStep.Name) {
				continue
			}
			key := idx.Key.AsString()
			refType := lastAlicloudToken(tr[:i])
			conf := Medium
			if rules.IsAffectedType(refType) {
				conf = High
			}
			ln := idx.SrcRange.Start.Line
			*out = append(*out, Finding{
				File: path, Line: ln, Category: REF, Target: refType,
				Attr: attrStep.Name, Key: key, Confidence: conf, Code: codeLine(lines, ln),
			})
		}
	}
}

// lastAlicloudToken returns the last root/attr name starting with "alicloud_".
func lastAlicloudToken(steps hcl.Traversal) string {
	found := ""
	for _, s := range steps {
		switch t := s.(type) {
		case hcl.TraverseRoot:
			if strings.HasPrefix(t.Name, "alicloud_") {
				found = t.Name
			}
		case hcl.TraverseAttr:
			if strings.HasPrefix(t.Name, "alicloud_") {
				found = t.Name
			}
		}
	}
	return found
}
