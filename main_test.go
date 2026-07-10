package main

import (
	"bytes"
	"strings"
	"testing"
)

func runArgs(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return out.String(), errb.String(), code
}

func TestCLI_Help(t *testing.T) {
	out, _, code := runArgs("--help")
	if code != 0 {
		t.Errorf("help exit=%d want 0", code)
	}
	if !strings.Contains(out, "用法:") {
		t.Error("help text missing")
	}
}

func TestCLI_Version(t *testing.T) {
	out, _, code := runArgs("--version")
	if code != 0 || !strings.Contains(out, "alicloud-v2-check") {
		t.Errorf("version bad: code=%d out=%q", code, out)
	}
}

func TestCLI_ScanTestdata_ExitAndJSON(t *testing.T) {
	out, _, code := runArgs("--json", "testdata")
	if code != 1 {
		t.Errorf("exit=%d want 1 (findings present)", code)
	}
	if !strings.Contains(out, `"actionable_count": 16`) {
		t.Errorf("expected actionable_count 16 in JSON, got:\n%s", out)
	}
}

func TestCLI_Clean_Exit0(t *testing.T) {
	_, _, code := runArgs("testdata/clean")
	if code != 0 {
		t.Errorf("clean exit=%d want 0", code)
	}
}

func TestCLI_FailOnModule(t *testing.T) {
	// datasources has REF but no MODULE -> --fail-on module should be 0
	_, _, code := runArgs("--fail-on", "module", "testdata/datasources")
	if code != 0 {
		t.Errorf("fail-on module on REF-only exit=%d want 0", code)
	}
	// modules dir has MODULE -> should be 1
	_, _, code = runArgs("--fail-on", "module", "testdata/modules")
	if code != 1 {
		t.Errorf("fail-on module on modules exit=%d want 1", code)
	}
}

func TestCLI_FailOnNone(t *testing.T) {
	_, _, code := runArgs("--fail-on", "none", "testdata")
	if code != 0 {
		t.Errorf("fail-on none exit=%d want 0", code)
	}
}

func TestCLI_BadFlag(t *testing.T) {
	_, errOut, code := runArgs("--nope")
	if code != 2 {
		t.Errorf("bad flag exit=%d want 2", code)
	}
	if !strings.Contains(errOut, "未知选项") {
		t.Error("expected unknown-flag error")
	}
}

func TestCLI_BadFailOn(t *testing.T) {
	_, _, code := runArgs("--fail-on", "bogus", "testdata")
	if code != 2 {
		t.Errorf("bad --fail-on exit=%d want 2", code)
	}
}

func TestCLI_QuietOmitsLegend(t *testing.T) {
	out, _, _ := runArgs("--quiet", "--no-color", "testdata/modules")
	if strings.Contains(out, "【类别说明】") {
		t.Error("quiet should omit legend")
	}
}
