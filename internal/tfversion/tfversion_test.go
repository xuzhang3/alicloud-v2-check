package tfversion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		raw               string
		wantV2, wantOnly3 bool
	}{
		{"~> 1.230", true, false},
		{"~> 2.0", true, false},
		{">= 1.0, < 3.0", true, false},
		{"~> 3.0", false, true},
		{">= 3.1.0", false, true},
		{">= 2.0", true, false}, // allows 2.x and 3.x -> in scope
		{"not-a-version", true, false},
	}
	for _, c := range cases {
		v2, only3 := classify(c.raw)
		if v2 != c.wantV2 || only3 != c.wantOnly3 {
			t.Errorf("classify(%q) = (%v,%v), want (%v,%v)", c.raw, v2, only3, c.wantV2, c.wantOnly3)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect_RequiredProviders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "versions.tf", `
terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.230"
    }
  }
}
`)
	v := Detect([]string{filepath.Join(dir, "versions.tf")})
	if len(v.Constraints) != 1 || v.Constraints[0].Raw != "~> 1.230" {
		t.Fatalf("constraints = %+v", v.Constraints)
	}
	if !v.AppliesV2 || v.OnlyV3Plus {
		t.Errorf("v1 should be in scope: AppliesV2=%v OnlyV3Plus=%v", v.AppliesV2, v.OnlyV3Plus)
	}
}

func TestDetect_V3OutOfScope(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "versions.tf", `
terraform {
  required_providers {
    alicloud = { source = "aliyun/alicloud", version = "~> 3.0" }
  }
}
`)
	v := Detect([]string{filepath.Join(dir, "versions.tf")})
	if v.AppliesV2 || !v.OnlyV3Plus {
		t.Errorf("v3 should be out of scope: AppliesV2=%v OnlyV3Plus=%v", v.AppliesV2, v.OnlyV3Plus)
	}
}

func TestDetect_NoneWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `resource "alicloud_vpc" "v" { cidr_block = "10.0.0.0/8" }`)
	v := Detect([]string{filepath.Join(dir, "main.tf")})
	if len(v.Constraints) != 0 || v.OnlyV3Plus {
		t.Errorf("expected no constraints, got %+v (only3=%v)", v.Constraints, v.OnlyV3Plus)
	}
}
