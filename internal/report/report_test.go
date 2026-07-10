package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aliyun/alicloud-v2-check/internal/scanner"
	"github.com/aliyun/alicloud-v2-check/internal/tfversion"
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

func TestText_QuietOmitsLegend(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, sample(), Options{Roots: []string{"."}, Quiet: true})
	if strings.Contains(buf.String(), "【类别说明】") {
		t.Error("quiet should omit legend")
	}
}

func TestText_EmptyClean(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, nil, Options{Roots: []string{"."}})
	if !strings.Contains(buf.String(), "未发现") {
		t.Error("clean message missing")
	}
}

func TestJSON_Shape(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sample(), []string{"."}, 2, Options{}); err != nil {
		t.Fatal(err)
	}
	var r JSONReport
	if err := json.Unmarshal(buf.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.ScannedFiles != 2 {
		t.Errorf("scanned=%d want 2", r.ScannedFiles)
	}
	if r.ActionableCount != 3 {
		t.Errorf("actionable=%d want 3", r.ActionableCount)
	}
	if len(r.Findings) != 4 {
		t.Errorf("findings=%d want 4", len(r.Findings))
	}
	// ensure non-escaped output for readability
	if strings.Contains(buf.String(), `\u`) {
		t.Error("JSON should not HTML/unicode-escape")
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

func TestJSON_LocalizedMessageEN(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sample(), []string{"."}, 1, Options{Lang: LangEN}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "must become block syntax") {
		t.Error("JSON message should be localized to EN")
	}
}

func TestAutoLang(t *testing.T) {
	if autoLang("zh_CN.UTF-8") != LangZH {
		t.Error("zh_CN should map to zh")
	}
	if autoLang("en_US.UTF-8") != LangEN {
		t.Error("en_US should map to en")
	}
	if autoLang("") != LangEN {
		t.Error("empty should default to en")
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
