# MicroFoundry Topology Outputs — Amazon S3

output "bucket_name" {
  value = aws_s3_bucket.this.id
}

output "region" {
  value = var.aws_region
}

output "access_key_id" {
  value     = aws_iam_access_key.this.id
  sensitive = true
}

output "secret_access_key" {
  value     = aws_iam_access_key.this.secret
  sensitive = true
}

output "endpoint" {
  value = "https://s3.${var.aws_region}.amazonaws.com"
}

output "uri" {
  value = "s3://${aws_s3_bucket.this.id}"
}
