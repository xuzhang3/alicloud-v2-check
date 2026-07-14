package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuzhang3/alicloud-v2-check/internal/scanner"
	"github.com/xuzhang3/alicloud-v2-check/internal/tfversion"
)

func sample() []scanner.Finding {
	return []scanner.Finding{
		{File: "a.tf", Line: 3, Category: scanner.ARG, Target: "alicloud_cs_kubernetes", Attr: "runtime", Confidence: scanner.High, Message: "m", Code: "runtime = {"},
		{File: "a.tf", Line: 8, Category: scanner.REF, Target: "alicloud_cs_kubernetes", Attr: "connections", Confidence: scanner.High, Message: "m", Code: "x"},
		{File: "b.tf", Line: 2, Category: scanner.MODULE, Target: "terraform-alicloud-modules/rds/alicloud", Confidence: scanner.High, Message: "m", Code: "s"},
		{File: "b.tf", Line: 1, Category: scanner.PRESENT, Target: "alicloud_cs_kubernetes", Attr: "runtime", Confidence: scanner.High, Message: "m", Code: "r"},
	}
}

func TestText_HasLegendAndFileLine(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, sample(), Options{Roots: []string{"."}})
	out := buf.String()
	if !strings.Contains(out, "【类别说明】") {
		t.Error("legend missing")
	}
	if !strings.Contains(out, "文件: a.tf:3") {
		t.Error("file:line missing")
	}
	if !strings.Contains(out, "模块: terraform-alicloud-modules/rds/alicloud") {
		t.Error("module target missing")
	}
}

func TestText_EmptyClean(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, nil, Options{Roots: []string{"."}})
	if !strings.Contains(buf.String(), "未发现") {
		t.Error("clean message missing")
	}
}

func TestMarkdown_TablesAndClean(t *testing.T) {
	var buf bytes.Buffer
	Markdown(&buf, sample(), Options{Roots: []string{"."}, Lang: LangEN})
	out := buf.String()
	if !strings.Contains(out, "# Alicloud Provider v2 Breaking Change Report") {
		t.Error("md title missing")
	}
	if !strings.Contains(out, "| File | Resource | Field | Fix |") {
		t.Error("md ARG/REF table header missing")
	}
	if !strings.Contains(out, "| File | Module | Fix |") {
		t.Error("md MODULE table header missing")
	}
	// clean case
	var cb bytes.Buffer
	Markdown(&cb, nil, Options{Roots: []string{"."}, Lang: LangEN})
	if !strings.Contains(cb.String(), "No affected resources") {
		t.Error("md clean message missing")
	}
}

func TestMarkdown_PipeEscaping(t *testing.T) {
	f := []scanner.Finding{{File: "a.tf", Line: 1, Category: scanner.REF, Target: "alicloud_x", Attr: "connections", Key: "a|b", Confidence: scanner.High}}
	var buf bytes.Buffer
	Markdown(&buf, f, Options{Roots: []string{"."}, Lang: LangEN})
	if strings.Contains(buf.String(), "a|b\"]`") { // raw pipe would break the table
		t.Error("pipe in key should be escaped in markdown cell")
	}
}

func TestVersionNote(t *testing.T) {
	v3 := tfversion.Verdict{
		Constraints: []tfversion.Constraint{{File: "v.tf", Line: 5, Raw: "~> 3.0"}},
		OnlyV3Plus:  true,
	}
	if got := VersionNote(v3, LangEN, false); !strings.Contains(got, "v3") {
		t.Errorf("en v3 note missing v3 mention: %q", got)
	}
	if got := VersionNote(v3, LangZH, false); !strings.Contains(got, "v3") {
		t.Errorf("zh v3 note missing: %q", got)
	}
	if got := VersionNote(v3, LangEN, true); !strings.Contains(got, "ignore-version") {
		t.Errorf("override note missing: %q", got)
	}
	v1 := tfversion.Verdict{
		Constraints: []tfversion.Constraint{{File: "v.tf", Line: 5, Raw: "~> 1.230"}},
		AppliesV2:   true,
	}
	if got := VersionNote(v1, LangEN, false); !strings.Contains(got, "scope") {
		t.Errorf("relevant note missing: %q", got)
	}
	if got := VersionNote(tfversion.Verdict{}, LangEN, false); got != "" {
		t.Errorf("no constraints should yield empty note, got %q", got)
	}
}

func TestAutoLang(t *testing.T) {
	if AutoLang("zh_CN.UTF-8") != LangZH {
		t.Error("zh_CN should map to zh")
	}
	if AutoLang("en_US.UTF-8") != LangEN {
		t.Error("en_US should map to en")
	}
	if AutoLang("") != LangEN {
		t.Error("empty should default to en")
	}
}

func TestText_GroupByResource(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, sample(), Options{Roots: []string{"."}, Lang: LangEN, GroupBy: GroupByResource})
	out := buf.String()
	// sample has 2 findings on alicloud_cs_kubernetes (ARG line 3, REF line 8) + PRESENT
	if !strings.Contains(out, "alicloud_cs_kubernetes  (3)") {
		t.Errorf("resource header/count wrong:\n%s", out)
	}
	if !strings.Contains(out, ":3  [ARG]") || !strings.Contains(out, ":8  [REF]") {
		t.Errorf("category tags missing:\n%s", out)
	}
	// module grouped under its own path
	if !strings.Contains(out, "terraform-alicloud-modules/rds/alicloud  (1)") {
		t.Errorf("module group missing:\n%s", out)
	}
}

func TestTree(t *testing.T) {
	files := []string{
		"ws/example1/a.tf",
		"ws/example1/b.tf",
		"ws/clean/c.tf",
	}
	findings := []scanner.Finding{
		{File: "ws/example1/a.tf", Line: 1, Category: scanner.ARG, Confidence: scanner.High},
		{File: "ws/example1/a.tf", Line: 2, Category: scanner.REF, Confidence: scanner.High},
		{File: "ws/example1/b.tf", Line: 3, Category: scanner.MODULE, Confidence: scanner.High},
		{File: "ws/example1/a.tf", Line: 4, Category: scanner.PRESENT, Confidence: scanner.High}, // not actionable
	}
	var buf bytes.Buffer
	Tree(&buf, files, findings, Options{Lang: LangEN}, false)
	out := buf.String()

	if !strings.Contains(out, "Workspace structure") {
		t.Error("missing header")
	}
	// a.tf has 2 actionable (ARG+REF, PRESENT excluded)
	if !strings.Contains(out, "a.tf  ⚠ 2") {
		t.Errorf("a.tf badge wrong:\n%s", out)
	}
	// b.tf has 1 (MODULE)
	if !strings.Contains(out, "b.tf  ⚠ 1") {
		t.Errorf("b.tf badge wrong:\n%s", out)
	}
	// clean file
	if !strings.Contains(out, "c.tf  ✓") {
		t.Errorf("c.tf should be clean:\n%s", out)
	}
	// example1 dir aggregate = 3
	if !strings.Contains(out, "example1  (3)") {
		t.Errorf("example1 aggregate wrong:\n%s", out)
	}
	// connectors present
	if !strings.Contains(out, "├── ") || !strings.Contains(out, "└── ") {
		t.Error("missing tree connectors")
	}
}

func TestTreePlain(t *testing.T) {
	files := []string{"ws/a.tf", "ws/b.tf"}
	findings := []scanner.Finding{
		{File: "ws/a.tf", Line: 1, Category: scanner.ARG, Confidence: scanner.High},
	}
	var buf bytes.Buffer
	Tree(&buf, files, findings, Options{Lang: LangEN}, true)
	out := buf.String()
	// ASCII-only connectors
	if !strings.Contains(out, "|-- ") || !strings.Contains(out, "+-- ") {
		t.Errorf("plain tree should use ASCII connectors:\n%s", out)
	}
	// plain badges
	if !strings.Contains(out, "a.tf  1") {
		t.Errorf("plain badge wrong:\n%s", out)
	}
	if !strings.Contains(out, "b.tf  ok") {
		t.Errorf("plain clean badge wrong:\n%s", out)
	}
	// no Unicode
	if strings.ContainsAny(out, "├└│⚠✓") {
		t.Error("plain tree should not contain Unicode")
	}
}

func TestExitCode(t *testing.T) {
	fs := sample()
	cases := map[FailOn]int{
		FailNone: 0, FailAny: 1, FailArg: 1, FailRef: 1, FailModule: 1,
	}
	for pol, want := range cases {
		if got := ExitCode(fs, pol); got != want {
			t.Errorf("ExitCode(%s)=%d want %d", pol, got, want)
		}
	}
	// module-only findings: arg/ref policies should be 0, module/any -> 1
	modOnly := []scanner.Finding{{Category: scanner.MODULE}}
	if ExitCode(modOnly, FailArg) != 0 {
		t.Error("arg policy should not fail on module-only")
	}
	if ExitCode(modOnly, FailRef) != 0 {
		t.Error("ref policy should not fail on module-only")
	}
	if ExitCode(modOnly, FailModule) != 1 {
		t.Error("module policy should fail on module-only")
	}
}
