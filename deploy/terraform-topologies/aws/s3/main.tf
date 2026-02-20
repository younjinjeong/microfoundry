# MicroFoundry Topology — Amazon S3

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

resource "aws_s3_bucket" "this" {
  bucket        = "mf-${var.instance_name}-${var.namespace}"
  force_destroy = true

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
    "microfoundry.io/instance"     = var.instance_name
    "microfoundry.io/namespace"    = var.namespace
  }
}

resource "aws_s3_bucket_versioning" "this" {
  bucket = aws_s3_bucket.this.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  bucket = aws_s3_bucket.this.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "this" {
  bucket = aws_s3_bucket.this.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# IAM user for app access (generates access key credentials)
resource "aws_iam_user" "this" {
  name = "mf-s3-${var.instance_name}"

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_iam_user_policy" "this" {
  name = "mf-s3-${var.instance_name}"
  user = aws_iam_user.this.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket",
        ]
        Resource = [
          aws_s3_bucket.this.arn,
          "${aws_s3_bucket.this.arn}/*",
        ]
      }
    ]
  })
}

resource "aws_iam_access_key" "this" {
  user = aws_iam_user.this.name
}
