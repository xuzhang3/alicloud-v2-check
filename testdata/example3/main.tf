terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.235"
    }
  }
}

provider "alicloud" {
  region = "cn-beijing"
}

module "dedicated_cluster" {
  source     = "./modules/dedicated-k8s"
  name       = "edge-prod"
  vswitch_id = "vsw-2ze0000000000000000000"
}

# 直接消费本地 module 暴露出来的连接信息（旧的 map 下标写法）
output "cluster_api" {
  value = module.dedicated_cluster.api_server_intranet
}
