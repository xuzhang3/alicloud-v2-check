# 基础网络 —— 均为不受影响资源

resource "alicloud_vpc" "main" {
  vpc_name   = "${local.name_prefix}-vpc"
  cidr_block = "172.16.0.0/12"
  tags       = local.common_tags
}

resource "alicloud_vswitch" "app" {
  for_each = toset(var.zones)

  vpc_id       = alicloud_vpc.main.id
  cidr_block   = cidrsubnet(alicloud_vpc.main.cidr_block, 8, index(var.zones, each.value))
  zone_id      = each.value
  vswitch_name = "${local.name_prefix}-${each.value}"
  tags         = local.common_tags
}

resource "alicloud_security_group" "cluster" {
  name   = "${local.name_prefix}-sg"
  vpc_id = alicloud_vpc.main.id
  tags   = local.common_tags
}

resource "alicloud_security_group_rule" "allow_https" {
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "443/443"
  security_group_id = alicloud_security_group.cluster.id
  cidr_ip           = "0.0.0.0/0"
}
