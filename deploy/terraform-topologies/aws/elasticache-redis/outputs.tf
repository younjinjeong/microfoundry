# MicroFoundry Topology Outputs — Amazon ElastiCache Redis

output "host" {
  value = aws_elasticache_replication_group.this.primary_endpoint_address
}

output "port" {
  value = "6379"
}

output "password" {
  value     = var.password
  sensitive = true
}

output "uri" {
  value     = "rediss://:${var.password}@${aws_elasticache_replication_group.this.primary_endpoint_address}:6379"
  sensitive = true
}
