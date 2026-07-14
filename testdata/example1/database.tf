# RDS —— 用数据源挑选实例规格，再用原生资源创建实例

data "alicloud_db_instance_classes" "mysql" {
  engine               = "MySQL"
  engine_version       = "8.0"
  category             = "HighAvailability"
  storage_type         = "cloud_essd"
  instance_charge_type = "PostPaid"
}

resource "alicloud_db_instance" "mysql" {
  engine           = "MySQL"
  engine_version   = "8.0"
  instance_type    = data.alicloud_db_instance_classes.mysql.instance_classes[0].instance_class
  instance_storage = 20
  instance_name    = "${local.name_prefix}-mysql"
  vswitch_id       = alicloud_vswitch.app[var.zones[0]].id
  security_ips     = ["10.0.0.0/8"]
  tags             = local.common_tags
}

output "mysql_storage_min" {
  value = data.alicloud_db_instance_classes.mysql.instance_classes[0].storage_range["min"]
}

output "mysql_storage_max" {
  value = data.alicloud_db_instance_classes.mysql.instance_classes[0].storage_range["max"]
}
