# MicroFoundry Topology Outputs — Amazon RDS PostgreSQL
# These become VCAP_SERVICES credentials when bound to an app.

output "host" {
  value = aws_db_instance.this.address
}

output "port" {
  value = tostring(aws_db_instance.this.port)
}

output "username" {
  value = aws_db_instance.this.username
}

output "password" {
  value     = var.password
  sensitive = true
}

output "database" {
  value = aws_db_instance.this.db_name
}

output "uri" {
  value     = "postgresql://${aws_db_instance.this.username}:${var.password}@${aws_db_instance.this.address}:${aws_db_instance.this.port}/${aws_db_instance.this.db_name}"
  sensitive = true
}
