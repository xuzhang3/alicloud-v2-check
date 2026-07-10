package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runArgs(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return out.String(), errb.String(), code
}

func writeTF(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCLI_Help(t *testing.T) {
	out, _, code := runArgs("--help")
	if code != 0 {
		t.Errorf("help exit=%d want 0", code)
	}
	if !strings.Contains(out, "alicloud-v2-check") || !strings.Contains(out, "--engine") {
		t.Errorf("help text missing expected content:\n%s", out)
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
	_, _, code := runArgs("--fail-on", "module", "testdata/datasources")
	if code != 0 {
		t.Errorf("fail-on module on REF-only exit=%d want 0", code)
	}
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
	_, _, code := runArgs("--nope")
	if code != 2 {
		t.Errorf("bad flag exit=%d want 2", code)
	}
}

func TestCLI_BadFailOn(t *testing.T) {
	_, _, code := runArgs("--fail-on", "bogus", "testdata")
	if code != 2 {
		t.Errorf("bad --fail-on exit=%d want 2", code)
	}
}

func TestCLI_LangZH(t *testing.T) {
	out, _, _ := runArgs("--lang", "zh", "--no-color", "testdata/modules")
	if !strings.Contains(out, "【类别说明】") || !strings.Contains(out, "模块:") {
		t.Errorf("zh output missing expected markers:\n%s", out)
	}
}

func TestCLI_LangEN(t *testing.T) {
	out, _, _ := runArgs("--lang", "en", "--no-color", "testdata/modules")
	if !strings.Contains(out, "[Legend]") || !strings.Contains(out, "Module:") {
		t.Errorf("en output missing expected markers:\n%s", out)
	}
}

func TestCLI_QuietOmitsLegend(t *testing.T) {
	out, _, _ := runArgs("--lang", "zh", "--quiet", "--no-color", "testdata/modules")
	if strings.Contains(out, "【类别说明】") {
		t.Error("quiet should omit legend")
	}
}

// Version gating: a config pinned to v3+ is out of scope and skipped, unless
// --ignore-version is set.
func TestCLI_VersionGating(t *testing.T) {
	dir := t.TempDir()
	writeTF(t, dir, "versions.tf", `
terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 3.0"
    }
  }
}
`)
	writeTF(t, dir, "main.tf", `
resource "alicloud_cs_kubernetes" "k" {
  runtime = { name = "containerd" }
}
`)
	// v3+ -> skipped -> exit 0
	out, _, code := runArgs("--lang", "en", "--no-color", dir)
	if code != 0 {
		t.Errorf("v3 constraint should skip -> exit 0, got %d", code)
	}
	if !strings.Contains(out, "v3") && !strings.Contains(out, "does not apply") {
		t.Errorf("expected v3 skip notice, got:\n%s", out)
	}

	// --ignore-version -> scans anyway -> ARG found -> exit 1
	_, _, code = runArgs("--lang", "en", "--no-color", "--ignore-version", dir)
	if code != 1 {
		t.Errorf("--ignore-version should scan -> exit 1, got %d", code)
	}
}

func TestCLI_VersionRelevant_V1(t *testing.T) {
	dir := t.TempDir()
	writeTF(t, dir, "versions.tf", `
terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.230"
    }
  }
}
`)
	writeTF(t, dir, "main.tf", `
resource "alicloud_cs_kubernetes" "k" {
  runtime = { name = "containerd" }
}
`)
	_, _, code := runArgs("--lang", "en", "--no-color", dir)
	if code != 1 {
		t.Errorf("v1 constraint in scope -> should scan and find ARG -> exit 1, got %d", code)
	}
}
