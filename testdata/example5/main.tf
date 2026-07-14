terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.270"
    }
  }
}

provider "alicloud" {
  region = "cn-hangzhou"
}

# 一个完全不涉及 v2 breaking change 的项目：只用未受影响的资源。
# 升级 provider 到 v2 时，此项目无需任何改动。

resource "alicloud_vpc" "main" {
  vpc_name   = "acme-vpc"
  cidr_block = "10.0.0.0/8"
}

resource "alicloud_vswitch" "app" {
  vpc_id       = alicloud_vpc.main.id
  cidr_block   = "10.1.0.0/16"
  zone_id      = "cn-hangzhou-h"
  vswitch_name = "acme-app"
}

resource "alicloud_security_group" "web" {
  name   = "acme-web-sg"
  vpc_id = alicloud_vpc.main.id
}

resource "alicloud_oss_bucket" "static" {
  bucket = "acme-static-assets"
  tags = {
    env = "prod"
  }
}

output "vpc_id" {
  value = alicloud_vpc.main.id
}

output "bucket_env_tag" {
  value = alicloud_oss_bucket.static.tags["env"]
}
