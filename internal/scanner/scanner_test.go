package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func counts(fs []Finding) map[Category]int {
	m := map[Category]int{}
	for _, f := range fs {
		m[f.Category]++
	}
	return m
}

func TestScanFile_AllCategories(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "main.tf", `
resource "alicloud_cs_kubernetes" "k" {
  runtime = {
    name = "containerd"
  }
}
output "cert" {
  value = alicloud_cs_kubernetes.k.certificate_authority["cluster_cert"]
}
module "rds" {
  source = "terraform-alicloud-modules/rds/alicloud"
}
`)
	fs, err := ScanFile(p)
	if err != nil {
		t.Fatal(err)
	}
	c := counts(fs)
	if c[ARG] != 1 {
		t.Errorf("ARG=%d want 1", c[ARG])
	}
	if c[REF] != 1 {
		t.Errorf("REF=%d want 1", c[REF])
	}
	if c[MODULE] != 1 {
		t.Errorf("MODULE=%d want 1", c[MODULE])
	}
	if c[PRESENT] != 1 {
		t.Errorf("PRESENT=%d want 1", c[PRESENT])
	}
}

func TestScanFile_NegativeCases(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "clean.tf", `
resource "alicloud_cs_kubernetes" "ok" {
  runtime {
    name = "containerd"
  }
}
output "cert" {
  value = alicloud_cs_kubernetes.ok.certificate_authority[0].cluster_cert
}
locals {
  tags = { env = "prod" }
  env  = local.tags["env"]
}
# comment: alicloud_cs_kubernetes.x.connections["api_server_internet"]
resource "alicloud_vpc" "v" {
  cidr_block = "10.0.0.0/8"
}
`)
	fs, err := ScanFile(p)
	if err != nil {
		t.Fatal(err)
	}
	c := counts(fs)
	if c[ARG]+c[REF]+c[MODULE] != 0 {
		t.Errorf("expected 0 actionable, got ARG=%d REF=%d MODULE=%d", c[ARG], c[REF], c[MODULE])
	}
	// one PRESENT for the affected resource declaration
	if c[PRESENT] != 1 {
		t.Errorf("PRESENT=%d want 1", c[PRESENT])
	}
}

func TestScanFile_Confidence(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "m.tf", `
output "x" {
  value = module.k8s.connections["api_server_internet"]
}
`)
	fs, _ := ScanFile(p)
	if len(fs) != 1 || fs[0].Category != REF {
		t.Fatalf("want 1 REF, got %+v", fs)
	}
	if fs[0].Confidence != Medium {
		t.Errorf("confidence=%s want MEDIUM (module output, untyped)", fs[0].Confidence)
	}
}

func TestScanFile_FormattingVariants(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "fmt.tf", `
resource "alicloud_cs_kubernetes" "c" {
  runtime={
    name = "containerd"
  }
}
output "a" {
  value = alicloud_cs_kubernetes.c.connections["api_server_internet"] # fix me
}
output "b" {
  value = alicloud_cs_kubernetes.c.certificate_authority[ "cluster_cert" ]
}
`)
	fs, _ := ScanFile(p)
	c := counts(fs)
	if c[ARG] != 1 || c[REF] != 2 {
		t.Errorf("ARG=%d REF=%d want 1/2", c[ARG], c[REF])
	}
}

func TestScanPaths_ExcludeAndDedup(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a/main.tf", `module "rds" { source = "terraform-alicloud-modules/rds/alicloud" }`)
	write(t, dir, ".claude/x.tf", `module "rds" { source = "terraform-alicloud-modules/rds/alicloud" }`)

	fs, n, err := ScanPaths([]string{dir}, Options{Excludes: []string{"**/.claude/**"}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("scanned files=%d want 1 (.claude excluded)", n)
	}
	if len(fs) != 1 {
		t.Errorf("findings=%d want 1", len(fs))
	}
}
