package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aliyun/alicloud-v2-check/internal/scanner"
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
