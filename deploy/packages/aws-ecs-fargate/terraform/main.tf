# -----------------------------------------------------------------------------
# MicroFoundry AWS ECS Fargate Deployment — Main Infrastructure
# -----------------------------------------------------------------------------
# Hybrid architecture:
#   - ECS Fargate: runs the MicroFoundry admin control plane
#   - EKS:         runs application workloads managed by MicroFoundry
#
# Traffic flow: Internet -> ALB -> ECS Fargate (mf admin:8080) -> EKS (workloads)
# -----------------------------------------------------------------------------

locals {
  name   = var.project_name
  region = var.region

  azs = slice(data.aws_availability_zones.available.names, 0, 3)

  # Container image — pulled from the ECR repository created below
  mf_image = "${aws_ecr_repository.mf.repository_url}:${var.mf_version}"

  # EFS mount path inside the Fargate container
  mf_config_mount = "/etc/microfoundry"
}

# -----------------------------------------------------------------------------
# Data Sources
# -----------------------------------------------------------------------------

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

# -----------------------------------------------------------------------------
# VPC
# -----------------------------------------------------------------------------

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${local.name}-vpc"
  cidr = "10.0.0.0/16"

  azs             = local.azs
  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  private_subnets = ["10.0.11.0/24", "10.0.12.0/24", "10.0.13.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true
  enable_dns_support   = true

  # Tags required for EKS subnet auto-discovery
  public_subnet_tags = {
    "kubernetes.io/role/elb"                    = "1"
    "kubernetes.io/cluster/${local.name}-eks"   = "shared"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"           = "1"
    "kubernetes.io/cluster/${local.name}-eks"   = "shared"
  }

  tags = {
    Component = "networking"
  }
}

# -----------------------------------------------------------------------------
# ECR Repository
# -----------------------------------------------------------------------------

resource "aws_ecr_repository" "mf" {
  name                 = "${local.name}/mf"
  image_tag_mutability = "MUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = {
    Component = "registry"
  }
}

resource "aws_ecr_lifecycle_policy" "mf" {
  repository = aws_ecr_repository.mf.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 10 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 10
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

# -----------------------------------------------------------------------------
# CloudWatch Log Group
# -----------------------------------------------------------------------------

resource "aws_cloudwatch_log_group" "mf" {
  name              = "/ecs/${local.name}"
  retention_in_days = 30

  tags = {
    Component = "logging"
  }
}

# -----------------------------------------------------------------------------
# EFS — Persistent Config Storage for MicroFoundry
# -----------------------------------------------------------------------------

resource "aws_efs_file_system" "mf_config" {
  creation_token = "${local.name}-config"
  encrypted      = true

  performance_mode = "generalPurpose"
  throughput_mode  = "bursting"

  tags = {
    Name      = "${local.name}-config"
    Component = "storage"
  }
}

resource "aws_efs_mount_target" "mf_config" {
  count = length(module.vpc.private_subnets)

  file_system_id  = aws_efs_file_system.mf_config.id
  subnet_id       = module.vpc.private_subnets[count.index]
  security_groups = [aws_security_group.efs.id]
}

resource "aws_efs_access_point" "mf_config" {
  file_system_id = aws_efs_file_system.mf_config.id

  posix_user {
    uid = 1000
    gid = 1000
  }

  root_directory {
    path = "/microfoundry"
    creation_info {
      owner_uid   = 1000
      owner_gid   = 1000
      permissions = "0755"
    }
  }

  tags = {
    Name = "${local.name}-config-ap"
  }
}

# -----------------------------------------------------------------------------
# Security Groups
# -----------------------------------------------------------------------------

# ALB Security Group — allows inbound HTTP/HTTPS from the internet
resource "aws_security_group" "alb" {
  name        = "${local.name}-alb-sg"
  description = "Allow inbound HTTP/HTTPS to the ALB"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "HTTP from internet"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS from internet"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name      = "${local.name}-alb-sg"
    Component = "networking"
  }
}

# Fargate Security Group — allows traffic only from the ALB
resource "aws_security_group" "fargate" {
  name        = "${local.name}-fargate-sg"
  description = "Allow traffic from ALB to Fargate tasks"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Traffic from ALB"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    description = "Allow all outbound (ECR, EKS API, etc.)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name      = "${local.name}-fargate-sg"
    Component = "compute"
  }
}

# EFS Security Group — allows NFS from Fargate tasks
resource "aws_security_group" "efs" {
  name        = "${local.name}-efs-sg"
  description = "Allow NFS from Fargate tasks to EFS"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "NFS from Fargate"
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = [aws_security_group.fargate.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name      = "${local.name}-efs-sg"
    Component = "storage"
  }
}

# EKS Security Group — additional rules for the workload cluster
resource "aws_security_group" "eks_additional" {
  name        = "${local.name}-eks-additional-sg"
  description = "Additional security group rules for the EKS cluster"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Allow Fargate tasks to reach EKS API"
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    security_groups = [aws_security_group.fargate.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name      = "${local.name}-eks-additional-sg"
    Component = "compute"
  }
}

# -----------------------------------------------------------------------------
# IAM — ECS Task Execution Role (pulling images, writing logs)
# -----------------------------------------------------------------------------

data "aws_iam_policy_document" "ecs_task_execution_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task_execution" {
  name               = "${local.name}-ecs-task-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_execution_assume.json

  tags = {
    Component = "iam"
  }
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Allow ECS to pull from ECR and write to CloudWatch
resource "aws_iam_role_policy" "ecs_task_execution_extra" {
  name = "${local.name}-ecs-exec-extra"
  role = aws_iam_role.ecs_task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.mf.arn}:*"
      },
    ]
  })
}

# -----------------------------------------------------------------------------
# IAM — ECS Task Role (application-level permissions, including EKS access)
# -----------------------------------------------------------------------------

data "aws_iam_policy_document" "ecs_task_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task" {
  name               = "${local.name}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume.json

  tags = {
    Component = "iam"
  }
}

# Grant the MicroFoundry task permissions to manage EKS workloads
resource "aws_iam_role_policy" "ecs_task_eks_access" {
  name = "${local.name}-eks-access"
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "eks:DescribeCluster",
          "eks:ListClusters",
          "eks:AccessKubernetesApi",
        ]
        Resource = module.eks.cluster_arn
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:PutImage",
          "ecr:InitiateLayerUpload",
          "ecr:UploadLayerPart",
          "ecr:CompleteLayerUpload",
          "ecr:CreateRepository",
          "ecr:DescribeRepositories",
          "ecr:ListImages",
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "elasticfilesystem:ClientMount",
          "elasticfilesystem:ClientWrite",
          "elasticfilesystem:DescribeFileSystems",
        ]
        Resource = aws_efs_file_system.mf_config.arn
      },
    ]
  })
}

# Grant MicroFoundry permissions to provision CSP-native services via Terraform
resource "aws_iam_role_policy" "ecs_task_csp_provisioning" {
  name = "${local.name}-csp-provisioning"
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "RDSProvisioning"
        Effect = "Allow"
        Action = [
          "rds:CreateDBInstance", "rds:DescribeDBInstances",
          "rds:ModifyDBInstance", "rds:DeleteDBInstance",
          "rds:CreateDBSubnetGroup", "rds:DeleteDBSubnetGroup",
          "rds:DescribeDBSubnetGroups",
          "rds:AddTagsToResource", "rds:ListTagsForResource",
        ]
        Resource = "arn:aws:rds:${var.region}:${data.aws_caller_identity.current.account_id}:*"
      },
      {
        Sid    = "ElastiCacheProvisioning"
        Effect = "Allow"
        Action = [
          "elasticache:CreateCacheCluster", "elasticache:DescribeCacheClusters",
          "elasticache:DeleteCacheCluster", "elasticache:CreateReplicationGroup",
          "elasticache:DeleteReplicationGroup", "elasticache:DescribeReplicationGroups",
          "elasticache:CreateCacheSubnetGroup", "elasticache:DeleteCacheSubnetGroup",
          "elasticache:DescribeCacheSubnetGroups",
          "elasticache:AddTagsToResource",
        ]
        Resource = "*"
      },
      {
        Sid    = "MSKProvisioning"
        Effect = "Allow"
        Action = [
          "kafka:CreateCluster", "kafka:DescribeCluster", "kafka:DescribeClusterV2",
          "kafka:DeleteCluster", "kafka:GetBootstrapBrokers",
          "kafka:CreateConfiguration", "kafka:TagResource",
        ]
        Resource = "*"
      },
      {
        Sid    = "S3Provisioning"
        Effect = "Allow"
        Action = [
          "s3:CreateBucket", "s3:DeleteBucket",
          "s3:PutBucketPolicy", "s3:PutBucketTagging",
          "s3:PutBucketVersioning", "s3:PutEncryptionConfiguration",
          "s3:PutBucketPublicAccessBlock",
          "s3:GetBucketLocation", "s3:GetBucketTagging",
        ]
        Resource = "arn:aws:s3:::mf-*"
      },
      {
        Sid    = "S3IAMAccess"
        Effect = "Allow"
        Action = [
          "iam:CreateUser", "iam:DeleteUser",
          "iam:CreateAccessKey", "iam:DeleteAccessKey",
          "iam:PutUserPolicy", "iam:DeleteUserPolicy",
          "iam:ListAccessKeys",
        ]
        Resource = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:user/mf-s3-*"
      },
      {
        Sid    = "OpenSearchProvisioning"
        Effect = "Allow"
        Action = [
          "es:CreateDomain", "es:DescribeDomain", "es:DescribeDomains",
          "es:DeleteDomain", "es:AddTags", "es:UpdateDomainConfig",
        ]
        Resource = "arn:aws:es:${var.region}:${data.aws_caller_identity.current.account_id}:domain/mf-*"
      },
      {
        Sid    = "SecurityGroupManagement"
        Effect = "Allow"
        Action = [
          "ec2:CreateSecurityGroup", "ec2:DeleteSecurityGroup",
          "ec2:AuthorizeSecurityGroupIngress", "ec2:RevokeSecurityGroupIngress",
          "ec2:AuthorizeSecurityGroupEgress", "ec2:RevokeSecurityGroupEgress",
          "ec2:DescribeSecurityGroups", "ec2:DescribeSubnets", "ec2:DescribeVpcs",
          "ec2:CreateTags",
        ]
        Resource = "*"
      },
      {
        Sid    = "STSForTokenGeneration"
        Effect = "Allow"
        Action = [
          "sts:GetCallerIdentity",
        ]
        Resource = "*"
      },
    ]
  })
}

# -----------------------------------------------------------------------------
# EKS Cluster — Workload Target
# -----------------------------------------------------------------------------

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = "${local.name}-eks"
  cluster_version = var.eks_cluster_version

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # Allow the Fargate task role to manage workloads on this cluster
  cluster_endpoint_public_access = true

  cluster_additional_security_group_ids = [aws_security_group.eks_additional.id]

  # Map the ECS task role into the EKS cluster as a system:masters principal
  # so MicroFoundry can create namespaces, deployments, services, etc.
  access_entries = {
    mf_admin = {
      kubernetes_groups = []
      principal_arn     = aws_iam_role.ecs_task.arn
      policy_associations = {
        cluster_admin = {
          policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
          access_scope = {
            type = "cluster"
          }
        }
      }
    }
  }

  eks_managed_node_groups = {
    default = {
      name           = "${local.name}-workers"
      instance_types = [var.eks_node_instance_type]

      min_size     = 1
      max_size     = var.eks_node_count * 2
      desired_size = var.eks_node_count

      iam_role_additional_policies = {
        AmazonSSMManagedInstanceCore = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
      }
    }
  }

  # Enable IRSA (IAM Roles for Service Accounts)
  enable_irsa = true

  tags = {
    Component = "compute"
  }
}

# Grant the ECS task role access to additional EKS clusters (multi-cluster)
resource "aws_iam_role_policy" "ecs_task_additional_eks" {
  count = length(var.additional_eks_clusters) > 0 ? 1 : 0
  name  = "${local.name}-additional-eks-access"
  role  = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "eks:DescribeCluster",
          "eks:ListClusters",
          "eks:AccessKubernetesApi",
        ]
        Resource = [for c in var.additional_eks_clusters : c.cluster_arn]
      }
    ]
  })
}

# -----------------------------------------------------------------------------
# ECS Cluster — Control Plane (Fargate)
# -----------------------------------------------------------------------------

resource "aws_ecs_cluster" "mf" {
  name = "${local.name}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = {
    Component = "compute"
  }
}

resource "aws_ecs_cluster_capacity_providers" "mf" {
  cluster_name = aws_ecs_cluster.mf.name

  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
    base              = 1
  }
}

# -----------------------------------------------------------------------------
# ECS Task Definition
# -----------------------------------------------------------------------------

resource "aws_ecs_task_definition" "mf_admin" {
  family                   = "${local.name}-admin"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.fargate_cpu
  memory                   = var.fargate_memory
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  # EFS volume for persistent MicroFoundry configuration
  volume {
    name = "mf-config"
    efs_volume_configuration {
      file_system_id     = aws_efs_file_system.mf_config.id
      transit_encryption = "ENABLED"
      authorization_configuration {
        access_point_id = aws_efs_access_point.mf_config.id
        iam             = "ENABLED"
      }
    }
  }

  container_definitions = jsonencode([
    {
      name      = "mf-admin"
      image     = local.mf_image
      essential = true

      portMappings = [
        {
          containerPort = 8080
          hostPort      = 8080
          protocol      = "tcp"
        }
      ]

      mountPoints = [
        {
          sourceVolume  = "mf-config"
          containerPath = local.mf_config_mount
          readOnly      = false
        }
      ]

      environment = [
        { name = "MF_ADMIN_PORT", value = "8080" },
        { name = "MF_ADMIN_HOST", value = "0.0.0.0" },
        { name = "MF_EKS_CLUSTER_NAME", value = "${local.name}-eks" },
        { name = "MF_EKS_REGION", value = var.region },
        { name = "MF_CONFIG_DIR", value = local.mf_config_mount },
        { name = "AWS_DEFAULT_REGION", value = var.region },
        { name = "MF_VPC_ID", value = module.vpc.vpc_id },
        { name = "MF_SUBNET_IDS", value = jsonencode(module.vpc.private_subnets) },
        { name = "MF_AWS_ACCOUNT", value = data.aws_caller_identity.current.account_id },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.mf.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = "mf-admin"
        }
      }

      healthCheck = {
        command     = ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }
    }
  ])

  tags = {
    Component = "compute"
  }
}

# -----------------------------------------------------------------------------
# Application Load Balancer
# -----------------------------------------------------------------------------

resource "aws_lb" "mf" {
  name               = "${local.name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = module.vpc.public_subnets

  enable_deletion_protection = false

  tags = {
    Name      = "${local.name}-alb"
    Component = "networking"
  }
}

resource "aws_lb_target_group" "mf_admin" {
  name        = "${local.name}-admin-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = module.vpc.vpc_id
  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 3
    unhealthy_threshold = 3
    interval            = 30
    timeout             = 5
    path                = "/health"
    protocol            = "HTTP"
    matcher             = "200"
  }

  tags = {
    Component = "networking"
  }
}

# ACM certificate for HTTPS (created only when domain is provided)
resource "aws_acm_certificate" "mf" {
  count             = var.domain != "" ? 1 : 0
  domain_name       = var.domain
  subject_alternative_names = ["*.${var.domain}"]
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = {
    Name      = "${local.name}-cert"
    Component = "networking"
  }
}

# HTTPS listener (when domain + ACM cert are available)
resource "aws_lb_listener" "https" {
  count             = var.domain != "" ? 1 : 0
  load_balancer_arn = aws_lb.mf.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate.mf[0].arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.mf_admin.arn
  }

  tags = {
    Component = "networking"
  }
}

# HTTP listener — redirects to HTTPS when domain is set, otherwise forwards directly
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.mf.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = var.domain != "" ? "redirect" : "forward"

    # Forward (no domain — HTTP only)
    target_group_arn = var.domain == "" ? aws_lb_target_group.mf_admin.arn : null

    # Redirect HTTP → HTTPS (domain set)
    dynamic "redirect" {
      for_each = var.domain != "" ? [1] : []
      content {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
  }

  tags = {
    Component = "networking"
  }
}

# -----------------------------------------------------------------------------
# ECS Service
# -----------------------------------------------------------------------------

resource "aws_ecs_service" "mf_admin" {
  name            = "${local.name}-admin"
  cluster         = aws_ecs_cluster.mf.id
  task_definition = aws_ecs_task_definition.mf_admin.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = module.vpc.private_subnets
    security_groups  = [aws_security_group.fargate.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.mf_admin.arn
    container_name   = "mf-admin"
    container_port   = 8080
  }

  # Wait for ALB listeners to be ready before placing tasks
  depends_on = [
    aws_lb_listener.http,
    aws_efs_mount_target.mf_config,
  ]

  # Allow ECS to manage task placement during deployments
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  deployment_maximum_percent         = 200
  deployment_minimum_healthy_percent = 100

  tags = {
    Component = "compute"
  }
}
