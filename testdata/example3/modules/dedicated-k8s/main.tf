variable "name" {
  type = string
}

variable "vswitch_id" {
  type = string
}

# 专有版 Kubernetes 集群
resource "alicloud_cs_kubernetes" "this" {
  name                  = var.name
  master_vswitch_ids    = [var.vswitch_id, var.vswitch_id, var.vswitch_id]
  master_instance_types = ["ecs.g6.xlarge", "ecs.g6.xlarge", "ecs.g6.xlarge"]
  pod_cidr              = "172.20.0.0/16"
  service_cidr          = "172.21.0.0/20"

  runtime = {
    name    = "containerd"
    version = "1.6.20"
  }
}

# 边缘集群
resource "alicloud_cs_edge_kubernetes" "edge" {
  name                  = "${var.name}-edge"
  worker_vswitch_ids    = [var.vswitch_id]
  worker_instance_types = ["ecs.g6.large"]
  worker_number         = 2

  runtime = {
    name    = "containerd"
    version = "1.6.20"
  }
}

output "api_server_intranet" {
  value = alicloud_cs_kubernetes.this.connections["api_server_intranet"]
}

output "master_public_ip" {
  value = alicloud_cs_edge_kubernetes.edge.connections["master_public_ip"]
}

output "cluster_cert" {
  value = alicloud_cs_kubernetes.this.certificate_authority["cluster_cert"]
}
