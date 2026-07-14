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
		case "resource", "data":
			if len(blk.Labels) > 0 && rules.IsAffectedType(blk.Labels[0]) {
				newEnclosing = blk.Labels[0]
				ln := blk.TypeRange.Start.Line
				*out = append(*out, newPresent(path, ln, blk.Labels[0], codeLine(lines, ln)))
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

// refKey identifies a REF finding by line + attribute name for deduplication.
type refKey struct {
	line int
	attr string
}

// scanExprRef finds `.attr["key"]` references (attr changed TypeMap->TypeList).
// It uses two strategies:
//  1. expr.Variables() catches flat traversals (e.g. x.name[0].attr["key"]).
//  2. walkExprTree catches RelativeTraversalExpr nodes that appear after
//     dynamic indices ([count.index]) or splats ([*]), where Variables()
//     only returns the prefix before the dynamic index.
func scanExprRef(expr hclsyntax.Expression, path string, lines []string, out *[]Finding) {
	seen := map[refKey]bool{}

	// Strategy 1: static traversals
	for _, tr := range expr.Variables() {
		for i := 1; i < len(tr); i++ {
			idx, ok := tr[i].(hcl.TraverseIndex)
			if !ok {
				continue
			}
			attrStep, ok := tr[i-1].(hcl.TraverseAttr)
			if !ok || !rules.IsMapIndexAttr(attrStep.Name) {
				continue
			}
			// Only flag string-keyed indices (map access). Numeric indices
			// like [0] are already list indices and don't need migration.
			if idx.Key.Type() != cty.String {
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
			seen[refKey{ln, attrStep.Name}] = true
		}
	}
	// Strategy 2: dynamic-index / splat sub-expressions
	walkExprTree(expr, "", path, lines, out, seen)
}

// walkExprTree recursively visits sub-expressions to find
// RelativeTraversalExpr nodes containing .attr["key"] patterns that
// Variables() cannot see (they follow a dynamic index or splat).
// ctxType is the alicloud type token propagated from a parent expression
// (e.g. SplatExpr passes its Source type into Each).
func walkExprTree(expr hclsyntax.Expression, ctxType, path string, lines []string, out *[]Finding, seen map[refKey]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *hclsyntax.RelativeTraversalExpr:
		// Determine the type from the Source, falling back to the propagated context.
		refType := sourceType(e.Source)
		if refType == "" {
			refType = ctxType
		}
		for i := 1; i < len(e.Traversal); i++ {
			idx, ok := e.Traversal[i].(hcl.TraverseIndex)
			if !ok {
				continue
			}
			attrStep, ok := e.Traversal[i-1].(hcl.TraverseAttr)
			if !ok || !rules.IsMapIndexAttr(attrStep.Name) {
				continue
			}
			// Only flag string-keyed indices (map access).
			if idx.Key.Type() != cty.String {
				continue
			}
			key := idx.Key.AsString()
			conf := Medium
			if rules.IsAffectedType(refType) {
				conf = High
			}
			ln := idx.SrcRange.Start.Line
			if seen[refKey{ln, attrStep.Name}] {
				continue
			}
			seen[refKey{ln, attrStep.Name}] = true
			*out = append(*out, Finding{
				File: path, Line: ln, Category: REF, Target: refType,
				Attr: attrStep.Name, Key: key, Confidence: conf, Code: codeLine(lines, ln),
			})
		}
		walkExprTree(e.Source, refType, path, lines, out, seen)
	case *hclsyntax.IndexExpr:
		walkExprTree(e.Collection, ctxType, path, lines, out, seen)
		walkExprTree(e.Key, ctxType, path, lines, out, seen)
	case *hclsyntax.SplatExpr:
		// Propagate the Source type into Each (where AnonSymbolExpr lives).
		srcType := sourceType(e.Source)
		if srcType == "" {
			srcType = ctxType
		}
		walkExprTree(e.Source, ctxType, path, lines, out, seen)
		walkExprTree(e.Each, srcType, path, lines, out, seen)
	case *hclsyntax.ConditionalExpr:
		walkExprTree(e.Condition, ctxType, path, lines, out, seen)
		walkExprTree(e.TrueResult, ctxType, path, lines, out, seen)
		walkExprTree(e.FalseResult, ctxType, path, lines, out, seen)
	case *hclsyntax.TemplateExpr:
		for _, p := range e.Parts {
			walkExprTree(p, ctxType, path, lines, out, seen)
		}
	case *hclsyntax.TemplateWrapExpr:
		walkExprTree(e.Wrapped, ctxType, path, lines, out, seen)
	case *hclsyntax.TupleConsExpr:
		for _, el := range e.Exprs {
			walkExprTree(el, ctxType, path, lines, out, seen)
		}
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			walkExprTree(item.KeyExpr, ctxType, path, lines, out, seen)
			walkExprTree(item.ValueExpr, ctxType, path, lines, out, seen)
		}
	case *hclsyntax.FunctionCallExpr:
		for _, arg := range e.Args {
			walkExprTree(arg, ctxType, path, lines, out, seen)
		}
	case *hclsyntax.ForExpr:
		walkExprTree(e.CollExpr, ctxType, path, lines, out, seen)
		walkExprTree(e.KeyExpr, ctxType, path, lines, out, seen)
		walkExprTree(e.ValExpr, ctxType, path, lines, out, seen)
		walkExprTree(e.CondExpr, ctxType, path, lines, out, seen)
	}
}

// sourceType extracts the alicloud type token from a sub-expression that feeds
// into a RelativeTraversalExpr. It handles IndexExpr (x[dyn]) and SplatExpr
// (x[*]) sources by looking at their Collection/Source traversal.
func sourceType(expr hclsyntax.Expression) string {
	switch e := expr.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		return lastAlicloudToken(e.Traversal)
	case *hclsyntax.IndexExpr:
		return sourceType(e.Collection)
	case *hclsyntax.SplatExpr:
		return sourceType(e.Source)
	case *hclsyntax.RelativeTraversalExpr:
		return sourceType(e.Source)
	}
	return ""
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
