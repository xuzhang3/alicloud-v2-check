variable "region" {
  type    = string
  default = "cn-hangzhou"
}

variable "env" {
  type    = string
  default = "prod"
}

variable "zones" {
  type    = list(string)
  default = ["cn-hangzhou-h", "cn-hangzhou-i", "cn-hangzhou-j"]
}

variable "k8s_version" {
  type    = string
  default = "1.28.9-aliyun.1"
}

locals {
  name_prefix = "acme-${var.env}"
  common_tags = {
    Project     = "acme"
    Environment = var.env
    ManagedBy   = "terraform"
  }
}
