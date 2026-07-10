# Fixture: no breaking changes. Expect 0 findings (no PRESENT either).

resource "alicloud_vpc" "v" {
  cidr_block = "10.0.0.0/8"
}

resource "alicloud_oss_bucket" "b" {
  bucket = "x"
  tags = {
    env = "prod"
  }
}

output "tag" {
  value = alicloud_oss_bucket.b.tags["env"]
}
