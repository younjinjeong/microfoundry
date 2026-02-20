# MicroFoundry Topology — Amazon ElastiCache Redis

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
  node_type = (
    var.plan_memory_mb <= 512  ? "cache.t3.micro" :
    var.plan_memory_mb <= 3072 ? "cache.t3.medium" :
    "cache.r6g.large"
  )
  num_replicas = var.plan_memory_mb >= 13312 ? 1 : 0
}

resource "aws_elasticache_subnet_group" "this" {
  name       = "mf-${var.instance_name}"
  subnet_ids = var.subnet_ids

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_security_group" "this" {
  name        = "mf-redis-${var.instance_name}"
  description = "MicroFoundry ElastiCache Redis - ${var.instance_name}"
  vpc_id      = var.vpc_id

  ingress {
    description = "Redis"
    from_port   = 6379
    to_port     = 6379
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
    Name                           = "mf-redis-${var.instance_name}"
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id = "mf-${var.instance_name}"
  description          = "MicroFoundry Redis - ${var.instance_name}"

  node_type            = local.node_type
  num_cache_clusters   = 1 + local.num_replicas
  port                 = 6379
  parameter_group_name = "default.redis7"
  engine_version       = "7.1"

  subnet_group_name  = aws_elasticache_subnet_group.this.name
  security_group_ids = [aws_security_group.this.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = var.password

  automatic_failover_enabled = local.num_replicas > 0

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
    "microfoundry.io/instance"     = var.instance_name
    "microfoundry.io/namespace"    = var.namespace
  }
}
