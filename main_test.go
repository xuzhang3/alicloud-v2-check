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

func TestCLI_ScanTestdata_ExitAndSummary(t *testing.T) {
	out, _, code := runArgs("--format", "text", "--lang", "en", "--no-color", "testdata")
	if code != 1 {
		t.Errorf("exit=%d want 1 (findings present)", code)
	}
	if !strings.Contains(out, "52 to fix") {
		t.Errorf("expected 52 actionable in summary, got:\n%s", out)
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
	out, _, _ := runArgs("--format", "text", "--lang", "zh", "--no-color", "testdata/modules")
	if !strings.Contains(out, "【类别说明】") || !strings.Contains(out, "[MODULE]") {
		t.Errorf("zh output missing expected markers:\n%s", out)
	}
}

func TestCLI_LangEN(t *testing.T) {
	out, _, _ := runArgs("--format", "text", "--lang", "en", "--no-color", "testdata/modules")
	if !strings.Contains(out, "[Legend]") || !strings.Contains(out, "[MODULE]") {
		t.Errorf("en output missing expected markers:\n%s", out)
	}
}

func TestCLI_Markdown(t *testing.T) {
	out, _, code := runArgs("--format", "markdown", "--lang", "en", "testdata")
	if code != 1 {
		t.Errorf("markdown exit=%d want 1", code)
	}
	if !strings.Contains(out, "# Alicloud Provider v2 Breaking Change Report") {
		t.Error("markdown title missing")
	}
	if !strings.Contains(out, "| File | Resource | Field | Fix |") {
		t.Errorf("markdown table header missing:\n%s", out)
	}
}

func TestCLI_OutputToFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "report.md")
	out, _, code := runArgs("--format", "markdown", "-o", fp, "testdata/modules")
	if code != 1 {
		t.Errorf("exit=%d want 1", code)
	}
	if out != "" {
		t.Errorf("stdout should be empty when -o used, got: %q", out)
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[MODULE]") {
		t.Errorf("report file missing content:\n%s", data)
	}
}

func TestCLI_Engines(t *testing.T) {
	for _, eng := range []string{"auto", "hcl", "regex"} {
		out, _, code := runArgs("--format", "text", "--engine", eng, "--lang", "en", "--no-color", "testdata")
		if code != 1 {
			t.Errorf("engine %s exit=%d want 1", eng, code)
		}
		if !strings.Contains(out, "52 to fix") {
			t.Errorf("engine %s: expected 52 actionable in summary", eng)
		}
	}
}

func TestCLI_BadEngine(t *testing.T) {
	if _, _, code := runArgs("--engine", "bogus", "testdata"); code != 2 {
		t.Errorf("bad --engine exit=%d want 2", code)
	}
}

func TestCLI_BadFormat(t *testing.T) {
	if _, _, code := runArgs("--format", "xml", "testdata"); code != 2 {
		t.Errorf("bad --format exit=%d want 2", code)
	}
}

func TestCLI_BadLang(t *testing.T) {
	if _, _, code := runArgs("--lang", "fr", "testdata"); code != 2 {
		t.Errorf("bad --lang exit=%d want 2", code)
	}
}

func TestCLI_MarkdownAlias(t *testing.T) {
	out, _, _ := runArgs("--format", "md", "--lang", "en", "testdata/modules")
	if !strings.Contains(out, "| File | Module | Fix |") {
		t.Errorf("md alias should render markdown:\n%s", out)
	}
}

func TestCLI_Exclude(t *testing.T) {
	// excluding the modules subdir drops MODULE findings from testdata/modules
	out, _, _ := runArgs("--format", "text", "--lang", "en", "--no-color", "--exclude", "**/modules/**", "testdata/modules")
	if strings.Contains(out, "[MODULE]") {
		t.Error("excluded modules dir should yield no MODULE findings")
	}
}

func TestCLI_FailOnArgAndRef(t *testing.T) {
	// datasources has REF but no ARG
	if _, _, code := runArgs("--fail-on", "arg", "testdata/datasources"); code != 0 {
		t.Errorf("--fail-on arg on REF-only should be 0, got %d", code)
	}
	if _, _, code := runArgs("--fail-on", "ref", "testdata/datasources"); code != 1 {
		t.Errorf("--fail-on ref on REF should be 1, got %d", code)
	}
}

func TestCLI_AutoLangFromEnv(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("LC_ALL", "")
	out, _, _ := runArgs("--format", "text", "--no-color", "testdata/modules") // no --lang -> auto
	if !strings.Contains(out, "汇总:") {
		t.Errorf("auto-lang from $LANG=zh should render Chinese:\n%s", out)
	}
}

func TestCLI_TextToFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "r.txt")
	if _, _, code := runArgs("--lang", "en", "-o", fp, "testdata/clean"); code != 0 {
		t.Errorf("clean text to file exit=%d want 0", code)
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "No affected resources") {
		t.Errorf("clean file content unexpected:\n%s", data)
	}
}

func TestCLI_GroupByResource(t *testing.T) {
	out, _, code := runArgs("--format", "text", "--group-by", "resource", "--lang", "en", "--no-color", "testdata/resources")
	if code != 1 {
		t.Errorf("exit=%d want 1", code)
	}
	// resource name as a section header, category tagged per finding
	if !strings.Contains(out, "alicloud_api_gateway_instance  (3)") {
		t.Errorf("resource group header missing:\n%s", out)
	}
	if !strings.Contains(out, ":5  [ARG]") || !strings.Contains(out, ":11  [REF]") {
		t.Errorf("category tags missing in resource grouping:\n%s", out)
	}
}

func TestCLI_BadGroupBy(t *testing.T) {
	if _, _, code := runArgs("--group-by", "bogus", "testdata"); code != 2 {
		t.Errorf("bad --group-by exit=%d want 2", code)
	}
}

func TestCLI_Tree(t *testing.T) {
	out, _, _ := runArgs("--format", "text", "--lang", "en", "--no-color", "testdata")
	if !strings.Contains(out, "Workspace structure") {
		t.Errorf("tree header missing:\n%s", out)
	}
	// affected file annotated, clean file checked
	if !strings.Contains(out, "main.tf  ⚠ 4") || !strings.Contains(out, "main.tf  ✓") {
		t.Errorf("tree badges missing:\n%s", out)
	}
	// tree branch glyphs present
	if !strings.Contains(out, "└── ") || !strings.Contains(out, "├── ") {
		t.Error("tree connectors missing")
	}
}

func TestCLI_LegendAlwaysShown(t *testing.T) {
	out, _, _ := runArgs("--format", "text", "--lang", "zh", "--no-color", "testdata/modules")
	if !strings.Contains(out, "【类别说明】") {
		t.Error("legend should always be shown")
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
