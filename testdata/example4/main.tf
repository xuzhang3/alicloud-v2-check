terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.228"
    }
  }
}

provider "alicloud" {
  region = "cn-shenzhen"
}

resource "alicloud_api_gateway_instance" "gateway" {
  instance_name = "public-api-gw"
  instance_spec = "api.s1.small"
  https_policy  = "HTTPS2_TLS1_0"
  payment_type  = "PayAsYouGo"

  # VPC 集成入口（v1 的 map 写法）
  to_connect_vpc_ip_block = {
    cidr_block = "10.0.0.0/24"
    vswitch_id = "vsw-abc1234567890"
    zone_id    = "cn-shenzhen-e"
  }
}

# 查询现有集群，用于把 API 路由到集群内网入口
data "alicloud_cs_kubernetes_clusters" "backends" {
  name_regex = "backend-.*"
}

data "alicloud_cs_serverless_kubernetes_clusters" "jobs" {
  name_regex = "jobs-.*"
}

output "gateway_ingress_cidr" {
  value = alicloud_api_gateway_instance.gateway.to_connect_vpc_ip_block["cidr_block"]
}

output "backend_api_intranet" {
  value = data.alicloud_cs_kubernetes_clusters.backends.clusters[0].connections["api_server_intranet"]
}

output "jobs_api_internet" {
  value = data.alicloud_cs_serverless_kubernetes_clusters.jobs.clusters[0].connections["api_server_internet"]
}
