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

# 这个项目通过官方 registry 模块创建数据库，命中已知受影响模块清单。
# 注意: 这些模块内部使用了受影响的 alicloud_db_instance_classes 等资源，
#       升级到 provider v2 时需要同步升级模块版本。

module "rds" {
  source  = "terraform-alicloud-modules/rds/alicloud"
  version = "2.5.1"

  engine         = "MySQL"
  engine_version = "8.0"
}

module "rds_mysql" {
  source  = "terraform-alicloud-modules/rds-mysql/alicloud"
  version = "2.0.0"
}

module "rds_postgres" {
  source  = "terraform-alicloud-modules/rds-postgres/alicloud"
  version = "2.0.0"
}

module "ots" {
  source  = "terraform-alicloud-modules/multi-zone-infrastructure-with-ots/alicloud"
  version = "2.0.0"
}
