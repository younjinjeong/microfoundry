# MicroFoundry Topology — Amazon MSK (Managed Kafka)

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
    var.plan_memory_mb <= 2048  ? "kafka.t3.small" :
    var.plan_memory_mb <= 8192  ? "kafka.m5.large" :
    "kafka.m5.2xlarge"
  )
  broker_count = var.plan_memory_mb >= 8192 ? 3 : 2
}

resource "aws_security_group" "this" {
  name        = "mf-msk-${var.instance_name}"
  description = "MicroFoundry MSK - ${var.instance_name}"
  vpc_id      = var.vpc_id

  ingress {
    description = "Kafka plaintext"
    from_port   = 9092
    to_port     = 9092
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }

  ingress {
    description = "Kafka TLS"
    from_port   = 9094
    to_port     = 9094
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
    Name                           = "mf-msk-${var.instance_name}"
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_msk_cluster" "this" {
  cluster_name           = "mf-${var.instance_name}"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = local.broker_count

  broker_node_group_info {
    instance_type  = local.instance_type
    client_subnets = slice(var.subnet_ids, 0, local.broker_count)
    security_groups = [aws_security_group.this.id]

    storage_info {
      ebs_storage_info {
        volume_size = var.plan_storage_gb
      }
    }
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS_PLAINTEXT"
      in_cluster    = true
    }
  }

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
    "microfoundry.io/instance"     = var.instance_name
    "microfoundry.io/namespace"    = var.namespace
  }
}
