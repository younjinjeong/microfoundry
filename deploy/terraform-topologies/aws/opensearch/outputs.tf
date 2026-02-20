# MicroFoundry Topology Outputs — Amazon OpenSearch

output "endpoint" {
  value = "https://${aws_opensearch_domain.this.endpoint}"
}

output "dashboard_endpoint" {
  value = "https://${aws_opensearch_domain.this.dashboard_endpoint}"
}

output "username" {
  value = "mfadmin"
}

output "password" {
  value     = var.password
  sensitive = true
}

output "uri" {
  value     = "https://mfadmin:${var.password}@${aws_opensearch_domain.this.endpoint}"
  sensitive = true
}
