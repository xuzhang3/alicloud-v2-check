// Package report renders scanner findings as text or JSON and computes the
// process exit code.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aliyun/alicloud-v2-check/internal/scanner"
)

// order controls category grouping in the text report.
var order = []scanner.Category{scanner.ARG, scanner.REF, scanner.MODULE, scanner.PRESENT}

var title = map[scanner.Category]string{
	scanner.ARG:     "map 赋值参数需改为 block 写法",
	scanner.REF:     "map 下标引用需改为 list 下标",
	scanner.MODULE:  "引用了已知受影响的模块",
	scanner.PRESENT: "出现受影响的资源 / 数据源（信息）",
}

var legend = map[scanner.Category]string{
	scanner.ARG:     "map 赋值参数 → 改成 block 写法。  例: `runtime = { ... }`  →  `runtime { ... }`",
	scanner.REF:     "map 下标引用 → 改成 list 下标。  例: `x.connections[\"key\"]`  →  `x.connections[0].key`",
	scanner.MODULE:  "引用了已知受影响的模块。  需升级模块版本并核对其 output 引用写法。",
	scanner.PRESENT: "（信息）出现受影响的资源/数据源。  未必有错，升级后请核对其 map→list 属性。",
}

// ANSI colors per category (empty when color disabled).
func colorFor(cat scanner.Category, color bool) (string, string) {
	if !color {
		return "", ""
	}
	const reset = "\x1b[0m"
	switch cat {
	case scanner.ARG:
		return "\x1b[33m", reset // yellow
	case scanner.REF:
		return "\x1b[36m", reset // cyan
	case scanner.MODULE:
		return "\x1b[35m", reset // magenta
	default:
		return "\x1b[90m", reset // grey
	}
}

// Options controls rendering.
type Options struct {
	Roots []string
	Color bool
	Quiet bool // omit the legend
}

// Text writes the human-readable report.
func Text(w io.Writer, findings []scanner.Finding, opts Options) {
	line := strings.Repeat("=", 72)
	sub := strings.Repeat("-", 72)
	fmt.Fprintln(w, line)
	fmt.Fprintln(w, "Alicloud Provider v2 Breaking Change 扫描报告")
	fmt.Fprintln(w, "扫描路径: "+strings.Join(opts.Roots, ", "))
	fmt.Fprintln(w, line)

	if len(findings) == 0 {
		fmt.Fprintln(w, "\n未发现受影响的资源、写法或模块。可放心升级到 v2（仍建议先跑 terraform plan 复核）。")
		return
	}

	if !opts.Quiet {
		fmt.Fprintln(w, "\n【类别说明】所有 v2 breaking change 本质都是属性从 TypeMap 变为 TypeList：")
		for _, cat := range order {
			fmt.Fprintf(w, "  [%-7s] %s\n", cat, legend[cat])
		}
		fmt.Fprintln(w, "  标注 [启发式/需人工确认] = 仅凭属性名匹配、无法确定所属资源类型，需人工判断。")
	}

	byCat := map[scanner.Category][]scanner.Finding{}
	for _, f := range findings {
		byCat[f.Category] = append(byCat[f.Category], f)
	}

	for _, cat := range order {
		items := byCat[cat]
		if len(items) == 0 {
			continue
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].File != items[j].File {
				return items[i].File < items[j].File
			}
			return items[i].Line < items[j].Line
		})
		c, reset := colorFor(cat, opts.Color)
		fmt.Fprintln(w, "\n"+sub)
		fmt.Fprintf(w, "%s[%s] %s  （%d 处）%s\n", c, cat, title[cat], len(items), reset)
		fmt.Fprintln(w, sub)
		for _, f := range items {
			conf := ""
			if f.Confidence != scanner.High {
				conf = "  [启发式/需人工确认]"
			}
			fmt.Fprintf(w, "  文件: %s:%d%s\n", f.File, f.Line, conf)
			if cat == scanner.MODULE {
				fmt.Fprintf(w, "  模块: %s\n", orDash(f.Target))
			} else {
				fmt.Fprintf(w, "  资源: %s\n", orUnknown(f.Target))
				fmt.Fprintf(w, "  字段: %s\n", orDash(f.Attr))
			}
			fmt.Fprintf(w, "  建议: %s\n", f.Message)
			fmt.Fprintf(w, "  代码: %s\n\n", strings.TrimSpace(f.Code))
		}
	}

	actionable := CountActionable(findings)
	fmt.Fprintln(w, line)
	fmt.Fprintf(w, "汇总: 需处理 %d 处（ARG/REF/MODULE），信息提示 %d 处。\n", actionable, len(findings)-actionable)
	fmt.Fprintln(w, "参考: 官方 version-2-upgrade 指南。")
	fmt.Fprintln(w, line)
}

// JSONReport is the machine-readable envelope.
type JSONReport struct {
	Roots           []string          `json:"roots"`
	ScannedFiles    int               `json:"scanned_files"`
	ActionableCount int               `json:"actionable_count"`
	Findings        []scanner.Finding `json:"findings"`
}

// JSON writes the findings as indented JSON.
func JSON(w io.Writer, findings []scanner.Finding, roots []string, scanned int) error {
	if findings == nil {
		findings = []scanner.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(JSONReport{
		Roots:           roots,
		ScannedFiles:    scanned,
		ActionableCount: CountActionable(findings),
		Findings:        findings,
	})
}

// CountActionable returns the number of non-PRESENT findings.
func CountActionable(findings []scanner.Finding) int {
	n := 0
	for _, f := range findings {
		if f.Actionable() {
			n++
		}
	}
	return n
}

// FailOn selects which categories cause a non-zero exit code.
type FailOn string

const (
	FailNone   FailOn = "none"
	FailModule FailOn = "module"
	FailRef    FailOn = "ref"
	FailArg    FailOn = "arg"
	FailAny    FailOn = "any"
)

// ExitCode computes the process exit code for the given fail-on policy.
//
//	any    -> 1 if any ARG/REF/MODULE finding exists
//	arg    -> 1 if any ARG finding exists
//	ref    -> 1 if any ARG or REF finding exists
//	module -> 1 if any MODULE finding exists
//	none   -> always 0
func ExitCode(findings []scanner.Finding, policy FailOn) int {
	var arg, ref, mod bool
	for _, f := range findings {
		switch f.Category {
		case scanner.ARG:
			arg = true
		case scanner.REF:
			ref = true
		case scanner.MODULE:
			mod = true
		}
	}
	switch policy {
	case FailNone:
		return 0
	case FailArg:
		if arg {
			return 1
		}
	case FailRef:
		if arg || ref {
			return 1
		}
	case FailModule:
		if mod {
			return 1
		}
	default: // any
		if arg || ref || mod {
			return 1
		}
	}
	return 0
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func orUnknown(s string) string {
	if s == "" {
		return "(无法确定,需人工确认)"
	}
	return s
}
