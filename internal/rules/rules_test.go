package rules

import "testing"

func TestCatalogCounts(t *testing.T) {
	if len(AffectedResources) != 5 {
		t.Errorf("AffectedResources = %d, want 5", len(AffectedResources))
	}
	if len(AffectedDataSources) != 8 {
		t.Errorf("AffectedDataSources = %d, want 8", len(AffectedDataSources))
	}
	if len(AffectedModules) != 4 {
		t.Errorf("AffectedModules = %d, want 4", len(AffectedModules))
	}
}

func TestIsAffectedType(t *testing.T) {
	cases := map[string]bool{
		"alicloud_cs_kubernetes":         true,
		"alicloud_instance_types":        true,
		"alicloud_cs_cluster_credential": true,
		"alicloud_vpc":                   false,
		"alicloud_instance":              false,
	}
	for in, want := range cases {
		if got := IsAffectedType(in); got != want {
			t.Errorf("IsAffectedType(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsAffectedModule(t *testing.T) {
	if !IsAffectedModule("terraform-alicloud-modules/rds/alicloud") {
		t.Error("rds module should be affected")
	}
	if IsAffectedModule("terraform-alicloud-modules/vpc/alicloud") {
		t.Error("vpc module should not be affected")
	}
}

func TestIsBlockArg(t *testing.T) {
	if !IsBlockArg("runtime") || !IsBlockArg("to_connect_vpc_ip_block") {
		t.Error("runtime/to_connect_vpc_ip_block should be block args")
	}
	if IsBlockArg("connections") {
		t.Error("connections is not a block arg")
	}
}
