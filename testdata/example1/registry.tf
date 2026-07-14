# 容器镜像仓库

resource "alicloud_cr_repo" "app" {
  namespace = "acme-${var.env}"
  name      = "api-server"
  summary   = "ACME API server image"
  repo_type = "PRIVATE"
}

data "alicloud_cr_repos" "all" {
  namespace = "acme-${var.env}"
}

output "app_repo_vpc_domain" {
  value = alicloud_cr_repo.app.domain_list["vpc"]
}

output "first_repo_public_domain" {
  value = data.alicloud_cr_repos.all.repos[0].domain_list["public"]
}
