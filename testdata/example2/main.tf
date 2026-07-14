terraform {
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.240"
    }
  }
}

provider "alicloud" {
  region = "cn-shanghai"
}

# 挑选一款 GPU 机型跑推理服务
data "alicloud_instance_types" "gpu" {
  cpu_core_count       = 8
  memory_size          = 32
  gpu_amount           = 1
  instance_type_family = "ecs.gn6i"
}

# 挑选一款突发性能实例跑批处理
data "alicloud_instance_types" "burst" {
  cpu_core_count = 2
  memory_size    = 4
  instance_charge_type = "PostPaid"
}

resource "alicloud_instance" "inference" {
  instance_name   = "inference-01"
  instance_type   = data.alicloud_instance_types.gpu.instance_types[0].id
  image_id        = "aliyun_3_x64_20G_alibase_20240528.vhd"
  security_groups = [alicloud_security_group.default.id]
  vswitch_id      = alicloud_vswitch.default.id
}

resource "alicloud_security_group" "default" {
  name   = "inference-sg"
  vpc_id = alicloud_vpc.default.id
}

resource "alicloud_vpc" "default" {
  vpc_name   = "compute-vpc"
  cidr_block = "192.168.0.0/16"
}

resource "alicloud_vswitch" "default" {
  vpc_id     = alicloud_vpc.default.id
  cidr_block = "192.168.1.0/24"
  zone_id    = "cn-shanghai-l"
}

output "gpu_count" {
  value = data.alicloud_instance_types.gpu.instance_types[0].gpu["amount"]
}

output "gpu_category" {
  value = data.alicloud_instance_types.gpu.instance_types[0].gpu["category"]
}

output "burst_baseline_credit" {
  value = data.alicloud_instance_types.burst.instance_types[0].burstable_instance["baseline_credit"]
}

output "local_disk_capacity" {
  value = data.alicloud_instance_types.gpu.instance_types[0].local_storage["capacity"]
}
