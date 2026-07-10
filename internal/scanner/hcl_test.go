package scanner

import (
	"errors"
	"testing"
)

// The HCL AST engine must NOT flag a `.attr["k"]` that appears inside a string
// / heredoc literal, whereas the regex engine (line-based) does. This is the
// core precision win.
func TestHCL_NoHeredocFalsePositive(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "doc.tf", `
resource "alicloud_oss_bucket_object" "doc" {
  bucket  = "docs"
  key     = "note.md"
  content = <<-EOT
    old syntax example:
    value = alicloud_cs_kubernetes.k.connections["api_server_internet"]
  EOT
}
`)
	// regex engine: the heredoc line looks like a real reference -> false positive
	rfs, err := ScanFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if counts(rfs)[REF] == 0 {
		t.Error("expected regex engine to (falsely) flag the heredoc line")
	}

	// HCL engine: the text is a literal, not a variable -> no REF
	hfs, err := ScanFileHCL(p)
	if err != nil {
		t.Fatal(err)
	}
	if counts(hfs)[REF] != 0 {
		t.Errorf("HCL engine should not flag heredoc literal, got %d REF", counts(hfs)[REF])
	}
}

// A real interpolation inside a template IS a reference and should be caught.
func TestHCL_InterpolationIsCaught(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "tmpl.tf", `
output "url" {
  value = "https://${alicloud_cs_kubernetes.k.connections["api_server_internet"]}/x"
}
`)
	hfs, err := ScanFileHCL(p)
	if err != nil {
		t.Fatal(err)
	}
	if counts(hfs)[REF] != 1 {
		t.Errorf("interpolated reference should be caught, got %d REF", counts(hfs)[REF])
	}
}

func TestHCL_ParseErrorSentinel(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "broken.tf", `resource "alicloud_cs_kubernetes" "k" {  runtime = { `) // unterminated
	_, err := ScanFileHCL(p)
	if !errors.Is(err, ErrParse) {
		t.Errorf("expected ErrParse for broken HCL, got %v", err)
	}
}

// auto engine must fall back to regex for files HCL can't parse.
func TestAutoEngine_FallbackOnParseError(t *testing.T) {
	dir := t.TempDir()
	// broken syntax but the module source line is still regex-matchable
	write(t, dir, "broken.tf", `
module "rds" {
  source = "terraform-alicloud-modules/rds/alicloud"
  this is not valid hcl {{{
`)
	fs, _, err := ScanPaths([]string{dir}, Options{Engine: EngineAuto})
	if err != nil {
		t.Fatal(err)
	}
	if counts(fs)[MODULE] != 1 {
		t.Errorf("auto engine should regex-fallback and find MODULE, got %d", counts(fs)[MODULE])
	}
}

// hcl engine skips unparseable files (no error, no findings from them).
func TestHCLEngine_SkipsUnparseable(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "broken.tf", `module "rds" { source = "terraform-alicloud-modules/rds/alicloud" {{{`)
	fs, _, err := ScanPaths([]string{dir}, Options{Engine: EngineHCL})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("hcl engine should skip unparseable file, got %d findings", len(fs))
	}
}

// Both engines must agree on the fixed, valid testdata tree.
func TestEngineParity_Testdata(t *testing.T) {
	hfs, _, err := ScanPaths([]string{"../../testdata"}, Options{Engine: EngineHCL})
	if err != nil {
		t.Fatal(err)
	}
	rfs, _, err := ScanPaths([]string{"../../testdata"}, Options{Engine: EngineRegex})
	if err != nil {
		t.Fatal(err)
	}
	h, r := counts(hfs), counts(rfs)
	for _, cat := range []Category{ARG, REF, MODULE, PRESENT} {
		if h[cat] != r[cat] {
			t.Errorf("engine mismatch on %s: hcl=%d regex=%d", cat, h[cat], r[cat])
		}
	}
}
