terraform {
  required_version = ">= 1.3.0"

  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.230"
    }
  }
}

provider "alicloud" {
  region = var.region
}
