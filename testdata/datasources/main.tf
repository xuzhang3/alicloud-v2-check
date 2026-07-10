# Fixture: affected data sources in v1 syntax. Expect REF + PRESENT (no ARG).

data "alicloud_db_instance_classes" "db" {
  engine = "MySQL"
}

output "db_min" {
  value = data.alicloud_db_instance_classes.db.instance_classes[0].storage_range["min"]
}

data "alicloud_instance_types" "t" {
  cpu_core_count = 2
}

output "gpu" {
  value = data.alicloud_instance_types.t.instance_types[0].gpu["amount"]
}

output "burst" {
  value = data.alicloud_instance_types.t.instance_types[0].burstable_instance["baseline_credit"]
}

data "alicloud_cs_kubernetes_clusters" "c" {
  name_regex = "x"
}

output "clusters_conn" {
  value = data.alicloud_cs_kubernetes_clusters.c.clusters[0].connections["api_server_internet"]
}
