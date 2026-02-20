# -----------------------------------------------------------------------------
# MicroFoundry AWS ECS Fargate Deployment — Input Variables
# -----------------------------------------------------------------------------

variable "project_name" {
  description = "Name prefix for all resources. Used in naming conventions and tags."
  type        = string
  default     = "microfoundry"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,28}[a-z0-9]$", var.project_name))
    error_message = "project_name must be 3-30 lowercase alphanumeric characters or hyphens, starting with a letter."
  }
}

variable "region" {
  description = "AWS region for all resources."
  type        = string
  default     = "us-east-1"
}

variable "domain" {
  description = "Base domain for applications deployed through MicroFoundry (e.g. apps.example.com)."
  type        = string
}

variable "mf_version" {
  description = "MicroFoundry container image version tag to deploy."
  type        = string
}

# -- ECS Fargate (Control Plane) -----------------------------------------------

variable "fargate_cpu" {
  description = "CPU units for the Fargate task (256, 512, 1024, 2048, 4096)."
  type        = number
  default     = 256

  validation {
    condition     = contains([256, 512, 1024, 2048, 4096], var.fargate_cpu)
    error_message = "fargate_cpu must be one of: 256, 512, 1024, 2048, 4096."
  }
}

variable "fargate_memory" {
  description = "Memory (MiB) for the Fargate task. Must be compatible with the chosen CPU value."
  type        = number
  default     = 512

  validation {
    condition     = var.fargate_memory >= 512 && var.fargate_memory <= 30720
    error_message = "fargate_memory must be between 512 and 30720 MiB."
  }
}

variable "use_fargate_spot" {
  description = "Use FARGATE_SPOT capacity provider for ~70% cost savings. MicroFoundry is a developer tool — Spot interruptions only delay new deploys, not running applications."
  type        = bool
  default     = true
}

# -- EKS (Workload Cluster) ---------------------------------------------------

variable "eks_node_instance_type" {
  description = "EC2 instance type for EKS worker nodes."
  type        = string
  default     = "t3.small"
}

variable "eks_node_count" {
  description = "Desired number of EKS worker nodes in the managed node group."
  type        = number
  default     = 1

  validation {
    condition     = var.eks_node_count >= 1 && var.eks_node_count <= 20
    error_message = "eks_node_count must be between 1 and 20."
  }
}

variable "enable_nat_gateway" {
  description = "Enable NAT gateway for private subnet internet access. Disable to save ~$32/month if the EKS cluster uses public subnets only."
  type        = bool
  default     = true
}

variable "eks_cluster_version" {
  description = "Kubernetes version for the EKS cluster."
  type        = string
  default     = "1.29"
}

# -- Multi-Cluster Support -----------------------------------------------------

variable "additional_eks_clusters" {
  description = "Additional EKS cluster ARNs that MicroFoundry should manage. The ECS task role will be granted EKS admin access to these clusters."
  type = list(object({
    cluster_arn  = string
    cluster_name = string
  }))
  default = []
}

# -- Tags ----------------------------------------------------------------------

variable "tags" {
  description = "Additional tags applied to all resources."
  type        = map(string)
  default     = {}
}
