# Example 7: Multi-cluster & conditional deployment patterns.
#
# Realistic customer scenario: a platform team manages Kubernetes clusters
# across multiple availability zones, optional staging environments, and
# per-microservice container registries. After upgrading to provider v2,
# all map-index references (connections["..."], certificate_authority["..."],
# gpu["..."]) must become list-index syntax.

variable "region" {
  type    = string
  default = "cn-hangzhou"
}

variable "zones" {
  type    = list(string)
  default = ["cn-hangzhou-h", "cn-hangzhou-i", "cn-hangzhou-j"]
}

variable "create_staging" {
  description = "Whether to provision a staging cluster"
  type        = bool
  default     = true
}

variable "services" {
  description = "Microservices that need a container registry repo"
  type        = set(string)
  default     = ["order-service", "payment-service", "user-service"]
}

variable "need_gpu_nodes" {
  description = "Whether the cluster requires GPU-capable instance types"
  type        = bool
  default     = true
}

locals {
  name_prefix = "platform-team"
  common_tags = {
    Team      = "platform"
    ManagedBy = "terraform"
  }
}

# ────────────────────────────────────────────────────────────
# 1. Production ACK cluster (always created)
# ────────────────────────────────────────────────────────────

resource "alicloud_vpc" "prod" {
  vpc_name   = "${local.name_prefix}-prod-vpc"
  cidr_block = "10.0.0.0/8"
  tags       = local.common_tags
}

resource "alicloud_vswitch" "prod" {
  for_each = toset(var.zones)

  vpc_id       = alicloud_vpc.prod.id
  cidr_block   = cidrsubnet(alicloud_vpc.prod.cidr_block, 8, index(var.zones, each.value))
  zone_id      = each.value
  vswitch_name = "${local.name_prefix}-prod-${each.value}"
}

resource "alicloud_cs_managed_kubernetes" "prod" {
  name               = "${local.name_prefix}-prod-ack"
  cluster_spec       = "ack.pro.small"
  worker_vswitch_ids = [for vsw in alicloud_vswitch.prod : vsw.id]
  pod_cidr           = "10.64.0.0/12"
  service_cidr       = "10.96.0.0/16"
  slb_internet_enabled = true
  tags               = local.common_tags
}

# Customer exports the API endpoint to feed into CI/CD or monitoring.
output "prod_api_endpoint" {
  value = alicloud_cs_managed_kubernetes.prod.connections["api_server_internet"]
}

output "prod_ca_cert" {
  sensitive = true
  value     = alicloud_cs_managed_kubernetes.prod.certificate_authority["cluster_cert"]
}

# ────────────────────────────────────────────────────────────
# 2. Staging cluster (conditional — count = 0 or 1)
#
#    A common pattern: create the staging cluster only when the
#    variable is true, then guard every output reference with the
#    same ternary to avoid accessing [0] on an empty list.
# ────────────────────────────────────────────────────────────

resource "alicloud_vpc" "staging" {
  count      = var.create_staging ? 1 : 0
  vpc_name   = "${local.name_prefix}-staging-vpc"
  cidr_block = "172.16.0.0/12"
  tags       = local.common_tags
}

resource "alicloud_vswitch" "staging" {
  count      = var.create_staging ? 1 : 0
  vpc_id     = alicloud_vpc.staging[0].id
  cidr_block = "172.16.1.0/24"
  zone_id    = var.zones[0]
}

resource "alicloud_cs_managed_kubernetes" "staging" {
  count              = var.create_staging ? 1 : 0
  name               = "${local.name_prefix}-staging-ack"
  cluster_spec       = "ack.standard"
  vswitch_ids        = [alicloud_vswitch.staging[0].id]
  pod_cidr           = "10.65.0.0/16"
  service_cidr       = "10.97.0.0/16"
  slb_internet_enabled = true
  tags               = local.common_tags
}

# Conditional reference — only access [0] when staging exists.
output "staging_api_endpoint" {
  value = var.create_staging ? alicloud_cs_managed_kubernetes.staging[0].connections["api_server_internet"] : null
}

output "staging_ca_cert" {
  sensitive = true
  value     = var.create_staging ? alicloud_cs_managed_kubernetes.staging[0].certificate_authority["cluster_cert"] : null
}

# ────────────────────────────────────────────────────────────
# 3. Multi-AZ dedicated K8s clusters (count > 1)
#
#    Some customers deploy a dedicated cluster per AZ for workload
#    isolation. count.index is used inside another resource with
#    the same count to build per-cluster configurations.
# ────────────────────────────────────────────────────────────

resource "alicloud_cs_kubernetes" "az" {
  count                 = length(var.zones)
  name                  = "${local.name_prefix}-dedicated-${var.zones[count.index]}"
  master_instance_types = ["ecs.g6.xlarge", "ecs.g6.xlarge", "ecs.g6.xlarge"]
  master_vswitch_ids    = [for vsw in alicloud_vswitch.prod : vsw.id]
  worker_vswitch_ids    = [alicloud_vswitch.prod[var.zones[count.index]].id]

  # ARG: map-assign must become block syntax in v2
  runtime = {
    name    = "containerd"
    version = "1.6.28"
  }

  tags = local.common_tags
}

# Cross-resource with matching count — typical for tagging or config management.
resource "null_resource" "cluster_monitor" {
  count = length(var.zones)

  triggers = {
    endpoint = alicloud_cs_kubernetes.az[count.index].connections["api_server_internet"]
    cluster  = alicloud_cs_kubernetes.az[count.index].id
  }
}

# Splat — collect all cluster API endpoints for a load-balancer config.
output "all_dedicated_endpoints" {
  value = alicloud_cs_kubernetes.az[*].connections["api_server_internet"]
}

# Literal [0] — quick access to the first cluster's cert.
output "first_cluster_ca" {
  sensitive = true
  value     = alicloud_cs_kubernetes.az[0].certificate_authority["cluster_cert"]
}

# ────────────────────────────────────────────────────────────
# 4. Per-microservice container registries (for_each)
#
#    Teams create one ACR repo per service. Each repo's VPC
#    endpoint is then referenced by the service's deployment config.
# ────────────────────────────────────────────────────────────

resource "alicloud_cr_repo" "service" {
  for_each  = var.services
  namespace = "default"
  name      = each.value
  repo_type = "PRIVATE"
  summary   = "Container image repo for ${each.value}"
}

# Look up one specific service's VPC endpoint for a CI pipeline.
output "order_service_domain" {
  value = alicloud_cr_repo.service["order-service"].domain_list["vpc"]
}

output "payment_service_domain" {
  value = alicloud_cr_repo.service["payment-service"].domain_list["internet"]
}

# ────────────────────────────────────────────────────────────
# 5. Conditional data source lookup for GPU instance types
#
#    Customers often conditionally query data sources to discover
#    available instance types. The [0] index on the data source
#    plus the map-index on gpu["amount"] both need migration.
# ────────────────────────────────────────────────────────────

data "alicloud_instance_types" "gpu" {
  count                = var.need_gpu_nodes ? 1 : 0
  cpu_core_count       = 8
  memory_size          = 32
  gpu_amount           = 1
  instance_type_family = "ecs.gn6i"
}

output "gpu_card_count" {
  value = var.need_gpu_nodes ? data.alicloud_instance_types.gpu[0].instance_types[0].gpu["amount"] : 0
}

output "gpu_card_category" {
  value = var.need_gpu_nodes ? data.alicloud_instance_types.gpu[0].instance_types[0].gpu["category"] : ""
}
