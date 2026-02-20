# MicroFoundry Topology — Amazon OpenSearch

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

locals {
  instance_type = (
    var.plan_memory_mb <= 2048  ? "t3.small.search" :
    var.plan_memory_mb <= 8192  ? "m6g.large.search" :
    "r6g.xlarge.search"
  )
  instance_count = (
    var.plan_memory_mb <= 2048 ? 1 :
    var.plan_memory_mb <= 8192 ? 2 :
    3
  )
  zone_awareness = local.instance_count > 1
}

resource "aws_security_group" "this" {
  name        = "mf-opensearch-${var.instance_name}"
  description = "MicroFoundry OpenSearch - ${var.instance_name}"
  vpc_id      = var.vpc_id

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name                           = "mf-opensearch-${var.instance_name}"
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_opensearch_domain" "this" {
  domain_name    = "mf-${var.instance_name}"
  engine_version = "OpenSearch_2.13"

  cluster_config {
    instance_type  = local.instance_type
    instance_count = local.instance_count

    zone_awareness_enabled = local.zone_awareness
    dynamic "zone_awareness_config" {
      for_each = local.zone_awareness ? [1] : []
      content {
        availability_zone_count = min(local.instance_count, 3)
      }
    }
  }

  ebs_options {
    ebs_enabled = true
    volume_type = "gp3"
    volume_size = var.plan_storage_gb
  }

  vpc_options {
    subnet_ids         = slice(var.subnet_ids, 0, local.zone_awareness ? min(local.instance_count, length(var.subnet_ids)) : 1)
    security_group_ids = [aws_security_group.this.id]
  }

  encrypt_at_rest {
    enabled = true
  }

  node_to_node_encryption {
    enabled = true
  }

  domain_endpoint_options {
    enforce_https       = true
    tls_security_policy = "Policy-Min-TLS-1-2-PFS-2023-10"
  }

  advanced_security_options {
    enabled                        = true
    internal_user_database_enabled = true

    master_user_options {
      master_user_name     = "mfadmin"
      master_user_password = var.password
    }
  }

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
    "microfoundry.io/instance"     = var.instance_name
    "microfoundry.io/namespace"    = var.namespace
  }
}
