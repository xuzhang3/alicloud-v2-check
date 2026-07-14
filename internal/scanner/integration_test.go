package scanner

import "testing"

// TestParity_Testdata asserts the fixed testdata tree produces the exact
// per-category counts, guarding against detection regressions.
func TestParity_Testdata(t *testing.T) {
	fs, n, err := ScanPaths([]string{"../../testdata"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no files scanned")
	}
	c := counts(fs)
	want := map[Category]int{ARG: 7, REF: 37, MODULE: 8, PRESENT: 25}
	for cat, w := range want {
		if c[cat] != w {
			t.Errorf("%s = %d, want %d", cat, c[cat], w)
		}
	}
}

func TestParity_Clean(t *testing.T) {
	fs, _, err := ScanPaths([]string{"../../testdata/clean"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("clean fixture produced %d findings, want 0: %+v", len(fs), fs)
	}
}

func TestParity_ModulesExcludeVPC(t *testing.T) {
	fs, _, err := ScanPaths([]string{"../../testdata/modules"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	c := counts(fs)
	if c[MODULE] != 4 {
		t.Errorf("MODULE = %d, want 4 (vpc must be excluded)", c[MODULE])
	}
	for _, f := range fs {
		if f.Target == "terraform-alicloud-modules/vpc/alicloud" {
			t.Error("vpc module should not be reported")
		}
	}
}
