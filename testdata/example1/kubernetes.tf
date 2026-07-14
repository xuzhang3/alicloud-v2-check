# 托管版 ACK 集群 + 凭证数据源

resource "alicloud_cs_managed_kubernetes" "prod" {
  name                 = "${local.name_prefix}-ack"
  cluster_spec         = "ack.pro.small"
  version              = var.k8s_version
  worker_vswitch_ids   = [for vsw in alicloud_vswitch.app : vsw.id]
  new_nat_gateway      = true
  pod_cidr             = "10.64.0.0/12"
  service_cidr         = "10.96.0.0/16"
  slb_internet_enabled = true
  tags                 = local.common_tags
}

data "alicloud_cs_cluster_credential" "prod" {
  cluster_id = alicloud_cs_managed_kubernetes.prod.id
}

# 供 CI 拉取 kubeconfig / 证书使用
output "cluster_ca" {
  value     = data.alicloud_cs_cluster_credential.prod.certificate_authority["cluster_cert"]
  sensitive = true
}

output "cluster_client_cert" {
  value     = alicloud_cs_managed_kubernetes.prod.certificate_authority["client_cert"]
  sensitive = true
}

output "cluster_api_internet" {
  value = alicloud_cs_managed_kubernetes.prod.connections["api_server_internet"]
}
