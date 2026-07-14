// Package report renders scanner findings as text or Markdown and computes the
// process exit code.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aliyun/alicloud-v2-check/internal/scanner"
	"github.com/aliyun/alicloud-v2-check/internal/tfversion"
)

// VersionNote builds the provider-version gating notice for the report. Returns
// "" when no alicloud version constraint was detected.
func VersionNote(v tfversion.Verdict, lang Lang, ignore bool) string {
	if len(v.Constraints) == 0 {
		return ""
	}
	bd := b(lang)
	raws := make([]string, 0, len(v.Constraints))
	for _, c := range v.Constraints {
		raws = append(raws, fmt.Sprintf("%s (%s:%d)", c.Raw, c.File, c.Line))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, bd.versionDetected, strings.Join(raws, "; "))
	switch {
	case ignore:
		fmt.Fprintf(&sb, "\n%s", bd.versionOverride)
	case v.OnlyV3Plus:
		fmt.Fprintf(&sb, "\n%s", bd.versionSkip)
	case v.AppliesV2:
		fmt.Fprintf(&sb, "\n%s", bd.versionRelevant)
	}
	return sb.String()
}

// UpgradeGuideURL is the official aliyun/alicloud provider v2 upgrade guide.
const UpgradeGuideURL = "https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/guides/version-2-upgrade.html.markdown"

// order controls category grouping in the text report.
var order = []scanner.Category{scanner.ARG, scanner.REF, scanner.MODULE, scanner.PRESENT}

// groupSorted buckets findings by category, each bucket sorted by file then line.
func groupSorted(findings []scanner.Finding) map[scanner.Category][]scanner.Finding {
	by := map[scanner.Category][]scanner.Finding{}
	for _, f := range findings {
		by[f.Category] = append(by[f.Category], f)
	}
	for _, items := range by {
		sort.Slice(items, func(i, j int) bool {
			if items[i].File != items[j].File {
				return items[i].File < items[j].File
			}
			return items[i].Line < items[j].Line
		})
	}
	return by
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

// GroupBy selects how findings are grouped in the report.
type GroupBy string

const (
	GroupByCategory GroupBy = "category" // ARG/REF/MODULE/PRESENT (default)
	GroupByResource GroupBy = "resource" // by affected resource / module name
)

// Options controls rendering.
type Options struct {
	Roots   []string
	Color   bool
	Lang    Lang    // output language (default zh)
	GroupBy GroupBy // grouping mode (default category)
	// VersionNote, if set, is printed near the top (provider-version gating).
	VersionNote string
}

type resourceGroup struct {
	title string
	items []scanner.Finding
}

// groupByResource buckets findings by Target (resource type / module / unknown),
// returning groups sorted by title, each group's items sorted by file:line.
func groupByResource(findings []scanner.Finding, bd bundle) []resourceGroup {
	buckets := map[string][]scanner.Finding{}
	for _, f := range findings {
		key := f.Target
		if key == "" {
			key = bd.unknownType
		}
		buckets[key] = append(buckets[key], f)
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	groups := make([]resourceGroup, 0, len(keys))
	for _, k := range keys {
		items := buckets[k]
		sort.Slice(items, func(i, j int) bool {
			if items[i].File != items[j].File {
				return items[i].File < items[j].File
			}
			return items[i].Line < items[j].Line
		})
		groups = append(groups, resourceGroup{title: k, items: items})
	}
	return groups
}

// writeBlock prints one finding block. When withCat is true (resource grouping)
// the category is tagged on the file line and the redundant resource/module line
// is omitted; otherwise (category grouping) the resource/module line is shown.
func writeBlock(w io.Writer, f scanner.Finding, bd bundle, lang Lang, withCat bool) {
	conf := ""
	if f.Confidence != scanner.High {
		conf = "  " + bd.heurTag
	}
	cat := ""
	if withCat {
		cat = "  [" + string(f.Category) + "]"
	}
	fmt.Fprintf(w, "  %s: %s:%d%s%s\n", bd.lblFile, f.File, f.Line, cat, conf)
	switch {
	case f.Category == scanner.MODULE:
		if !withCat {
			fmt.Fprintf(w, "  %s: %s\n", bd.lblModule, orDash(f.Target))
		}
	case !withCat:
		fmt.Fprintf(w, "  %s: %s\n", bd.lblResource, orUnknown(f.Target, bd))
		fmt.Fprintf(w, "  %s: %s\n", bd.lblField, orDash(f.Attr))
	case f.Attr != "":
		fmt.Fprintf(w, "  %s: %s\n", bd.lblField, f.Attr)
	}
	fmt.Fprintf(w, "  %s: %s\n", bd.lblAdvice, localize(f, lang))
	fmt.Fprintf(w, "  %s: %s\n\n", bd.lblCode, strings.TrimSpace(f.Code))
}

// Text writes the human-readable report.
func Text(w io.Writer, findings []scanner.Finding, opts Options) {
	bd := b(opts.Lang)
	line := strings.Repeat("=", 72)
	sub := strings.Repeat("-", 72)
	fmt.Fprintln(w, line)
	fmt.Fprintln(w, bd.reportTitle)
	fmt.Fprintln(w, bd.scanPath+strings.Join(opts.Roots, ", "))
	fmt.Fprintln(w, line)

	if opts.VersionNote != "" {
		fmt.Fprintln(w, "\n"+opts.VersionNote)
	}

	if len(findings) == 0 {
		fmt.Fprintln(w, "\n"+bd.clean)
		return
	}

	fmt.Fprintln(w, "\n"+bd.legendHead)
	for _, cat := range order {
		fmt.Fprintf(w, "  [%-7s] %s\n", cat, bd.legend[cat])
	}
	fmt.Fprintln(w, bd.heuristic)

	if opts.GroupBy == GroupByResource {
		for _, g := range groupByResource(findings, bd) {
			fmt.Fprintln(w, "\n"+sub)
			fmt.Fprintf(w, "%s  (%d)\n", g.title, len(g.items))
			fmt.Fprintln(w, sub)
			for _, f := range g.items {
				writeBlock(w, f, bd, opts.Lang, true)
			}
		}
	} else {
		byCat := groupSorted(findings)
		for _, cat := range order {
			items := byCat[cat]
			if len(items) == 0 {
				continue
			}
			c, reset := colorFor(cat, opts.Color)
			fmt.Fprintln(w, "\n"+sub)
			fmt.Fprintf(w, "%s[%s] %s  (%d)%s\n", c, cat, bd.catTitle[cat], len(items), reset)
			fmt.Fprintln(w, sub)
			for _, f := range items {
				writeBlock(w, f, bd, opts.Lang, false)
			}
		}
	}

	actionable := CountActionable(findings)
	fmt.Fprintln(w, line)
	fmt.Fprintf(w, bd.summary+"\n", actionable, len(findings)-actionable)
	fmt.Fprintln(w, bd.refLine)
	fmt.Fprintln(w, "      "+UpgradeGuideURL)
	fmt.Fprintln(w, line)
}

// Markdown writes a Markdown report (good for PRs, wikis, issues).
func Markdown(w io.Writer, findings []scanner.Finding, opts Options) {
	bd := b(opts.Lang)
	fmt.Fprintf(w, "# %s\n\n", bd.reportTitle)
	fmt.Fprintf(w, "- %s`%s`\n", bd.scanPath, strings.Join(opts.Roots, "`, `"))
	actionable := CountActionable(findings)
	fmt.Fprintf(w, "- "+bd.summary+"\n", actionable, len(findings)-actionable)
	fmt.Fprintf(w, "- %s [%s](%s)\n", bd.refLine, "version-2-upgrade", UpgradeGuideURL)

	if opts.VersionNote != "" {
		fmt.Fprintln(w)
		for ln := range strings.SplitSeq(opts.VersionNote, "\n") {
			fmt.Fprintf(w, "> %s\n", ln)
		}
	}

	if len(findings) == 0 {
		fmt.Fprintf(w, "\n> %s\n", bd.clean)
		return
	}

	fmt.Fprintf(w, "\n## %s\n\n", strings.Trim(bd.legendHead, "【】:："))
	for _, cat := range order {
		fmt.Fprintf(w, "- **%s** — %s\n", cat, bd.legend[cat])
	}

	byCat := groupSorted(findings)
	for _, cat := range order {
		items := byCat[cat]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n## [%s] %s (%d)\n\n", cat, bd.catTitle[cat], len(items))
		if cat == scanner.MODULE {
			fmt.Fprintf(w, "| %s | %s | %s |\n|---|---|---|\n", bd.lblFile, bd.lblModule, bd.lblAdvice)
			for _, f := range items {
				fmt.Fprintf(w, "| `%s:%d` | %s | %s |\n",
					f.File, f.Line, mdCell(orDash(f.Target)), mdCell(localize(f, opts.Lang)))
			}
			continue
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n|---|---|---|---|\n",
			bd.lblFile, bd.lblResource, bd.lblField, bd.lblAdvice)
		for _, f := range items {
			conf := ""
			if f.Confidence != scanner.High {
				conf = " " + bd.heurTag
			}
			fmt.Fprintf(w, "| `%s:%d`%s | %s | %s | %s |\n",
				f.File, f.Line, conf, mdCell(orUnknown(f.Target, bd)),
				mdCell(orDash(f.Attr)), mdCell(localize(f, opts.Lang)))
		}
	}
}

// mdCell escapes pipes/newlines so a value is safe inside a Markdown table cell.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
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

func orUnknown(s string, bd bundle) string {
	if s == "" {
		return bd.unknownType
	}
	return s
}
