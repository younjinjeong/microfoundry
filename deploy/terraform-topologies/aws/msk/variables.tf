# MicroFoundry Topology — Amazon MSK (Managed Kafka)

variable "instance_name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}

variable "k8s_name" {
  type = string
}

variable "plan_memory_mb" {
  type = number
}

variable "plan_cpu_millis" {
  type = number
}

variable "plan_storage_gb" {
  type = number
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "vpc_id" {
  type    = string
  default = ""
}

variable "subnet_ids" {
  type    = list(string)
  default = []
}
