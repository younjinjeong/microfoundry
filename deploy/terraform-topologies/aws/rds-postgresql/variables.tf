# MicroFoundry Topology — Amazon RDS PostgreSQL
# Standard variables injected by MicroFoundry Terraform executor

variable "instance_name" {
  description = "Service instance name"
  type        = string
}

variable "namespace" {
  description = "MicroFoundry namespace"
  type        = string
}

variable "password" {
  description = "Database master password"
  type        = string
  sensitive   = true
}

variable "k8s_name" {
  description = "Kubernetes secret name"
  type        = string
}

variable "plan_memory_mb" {
  description = "Plan memory in MB"
  type        = number
}

variable "plan_cpu_millis" {
  description = "Plan CPU in millicores"
  type        = number
}

variable "plan_storage_gb" {
  description = "Plan storage in GB"
  type        = number
}

# AWS-specific variables (injected by executor when running on AWS)
variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_id" {
  description = "VPC ID for the database"
  type        = string
  default     = ""
}

variable "subnet_ids" {
  description = "Private subnet IDs for the DB subnet group"
  type        = list(string)
  default     = []
}
