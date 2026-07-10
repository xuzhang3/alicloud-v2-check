# Fixture: registry-module usage. Expect MODULE x4 (vpc not reported).

module "rds" {
  source = "terraform-alicloud-modules/rds/alicloud"
}

module "rds_mysql" {
  source = "terraform-alicloud-modules/rds-mysql/alicloud"
}

module "rds_pg" {
  source = "terraform-alicloud-modules/rds-postgres/alicloud"
}

module "ots" {
  source = "terraform-alicloud-modules/multi-zone-infrastructure-with-ots/alicloud"
}

module "vpc" {
  source = "terraform-alicloud-modules/vpc/alicloud"
}
