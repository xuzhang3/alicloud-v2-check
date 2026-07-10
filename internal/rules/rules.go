// Package rules holds the alicloud provider v2 breaking-change catalog.
//
// 所有 v2 breaking change 本质都是属性从 TypeMap 变为 TypeList。清单来源：
// https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/guides/version-2-upgrade.html.markdown
package rules

import "slices"

// AffectedResources maps a resource type to the attributes that changed
// from TypeMap to TypeList in provider v2.
var AffectedResources = map[string][]string{
	"alicloud_api_gateway_instance":  {"to_connect_vpc_ip_block"},
	"alicloud_cr_repo":               {"domain_list"},
	"alicloud_cs_edge_kubernetes":    {"runtime", "certificate_authority", "connections"},
	"alicloud_cs_kubernetes":         {"runtime", "certificate_authority", "connections"},
	"alicloud_cs_managed_kubernetes": {"certificate_authority", "connections"},
}

// AffectedDataSources maps a data source type to the changed attributes.
var AffectedDataSources = map[string][]string{
	"alicloud_cr_repos":                          {"domain_list"},
	"alicloud_cs_cluster_credential":             {"certificate_authority"},
	"alicloud_cs_edge_kubernetes_clusters":       {"connections"},
	"alicloud_cs_kubernetes_clusters":            {"connections"},
	"alicloud_cs_managed_kubernetes_clusters":    {"connections"},
	"alicloud_cs_serverless_kubernetes_clusters": {"connections"},
	"alicloud_db_instance_classes":               {"storage_range"},
	"alicloud_instance_types":                    {"gpu", "burstable_instance", "local_storage"},
}

// BlockArgAttrs are user-writable arguments whose assignment syntax changed
// from a map (`attr = { ... }`) to a nested block (`attr { ... }`).
var BlockArgAttrs = []string{"runtime", "to_connect_vpc_ip_block"}

// MapIndexAttrs are every attribute (across all affected types) that changed
// TypeMap -> TypeList, so a `.attr["key"]` reference must become `.attr[0].key`.
var MapIndexAttrs = []string{
	"to_connect_vpc_ip_block",
	"domain_list",
	"runtime",
	"certificate_authority",
	"connections",
	"storage_range",
	"gpu",
	"burstable_instance",
	"local_storage",
}

// AffectedModules are the known terraform-alicloud-modules that internally use
// affected resources and must be upgraded for v2.
var AffectedModules = []string{
	"terraform-alicloud-modules/rds/alicloud",
	"terraform-alicloud-modules/rds-mysql/alicloud",
	"terraform-alicloud-modules/rds-postgres/alicloud",
	"terraform-alicloud-modules/multi-zone-infrastructure-with-ots/alicloud",
}

// SkipDirs are directory names ignored during the recursive walk.
var SkipDirs = map[string]bool{
	".terraform":   true,
	".git":         true,
	".idea":        true,
	".vscode":      true,
	"node_modules": true,
}

// IsAffectedType reports whether a resource/data type is in either catalog.
func IsAffectedType(t string) bool {
	if _, ok := AffectedResources[t]; ok {
		return true
	}
	_, ok := AffectedDataSources[t]
	return ok
}

// IsAffectedModule reports whether a module source (base path, without //subdir)
// is a known-affected module.
func IsAffectedModule(source string) bool {
	return slices.Contains(AffectedModules, source)
}

// IsBlockArg reports whether attr is a map-assign argument needing block syntax.
func IsBlockArg(attr string) bool {
	return slices.Contains(BlockArgAttrs, attr)
}
