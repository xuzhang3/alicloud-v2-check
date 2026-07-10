# Fixture: all 5 affected resources in v1 (old) syntax.
# Expect: ARG (runtime x2, to_connect_vpc_ip_block x1) + REF (several) + PRESENT.

resource "alicloud_api_gateway_instance" "gw" {
  to_connect_vpc_ip_block = {
    cidr_block = "172.16.0.0/12"
  }
}

output "gw_cidr" {
  value = alicloud_api_gateway_instance.gw.to_connect_vpc_ip_block["cidr_block"]
}

resource "alicloud_cr_repo" "repo" {
  namespace = "ns"
  name      = "r"
  summary   = "s"
}

output "repo_public" {
  value = alicloud_cr_repo.repo.domain_list["public"]
}

resource "alicloud_cs_kubernetes" "k8s" {
  name = "k"
  runtime = {
    name = "containerd"
  }
}

output "k8s_cert" {
  value = alicloud_cs_kubernetes.k8s.certificate_authority["cluster_cert"]
}

resource "alicloud_cs_edge_kubernetes" "edge" {
  name = "e"
  runtime = {
    name = "containerd"
  }
}

output "edge_conn" {
  value = alicloud_cs_edge_kubernetes.edge.connections["master_public_ip"]
}

resource "alicloud_cs_managed_kubernetes" "m" {
  name = "m"
}

output "m_conn" {
  value = alicloud_cs_managed_kubernetes.m.connections["service_domain"]
}
