#!/bin/bash
set -euo pipefail

# -----------------------------------------------------------------------------
# MicroFoundry AWS ECS Fargate + EKS — Installation Script
# -----------------------------------------------------------------------------
# This script provisions the complete hybrid infrastructure:
#   - VPC with public/private subnets
#   - ECS Fargate cluster (MicroFoundry control plane, FARGATE_SPOT by default)
#   - EKS cluster (application workloads, 1x t3.small by default)
#   - ALB, ECR, EFS, IAM roles, CloudWatch
#
# Cost-optimized defaults (~$108/month):
#   MicroFoundry is a developer tool — if it goes down, running apps are
#   unaffected; only new deploys are temporarily blocked. Defaults use
#   FARGATE_SPOT (0.25 vCPU, 512 MiB) and a single t3.small EKS node.
#
# Usage:
#   ./install.sh                    # Interactive — prompts terraform apply
#   ./install.sh --auto-approve     # Non-interactive — auto-approves apply
# -----------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="${SCRIPT_DIR}/terraform"

AUTO_APPROVE=""
if [[ "${1:-}" == "--auto-approve" ]]; then
  AUTO_APPROVE="-auto-approve"
fi

# -- Color helpers (disabled if not a terminal) --------------------------------

if [[ -t 1 ]]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  NC='\033[0m'
else
  RED='' GREEN='' YELLOW='' BLUE='' NC=''
fi

info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# -- Prerequisite checks ------------------------------------------------------

check_command() {
  local cmd="$1"
  local install_hint="${2:-}"
  if ! command -v "$cmd" &>/dev/null; then
    error "'$cmd' is not installed or not in PATH."
    if [[ -n "$install_hint" ]]; then
      error "Install it: $install_hint"
    fi
    return 1
  fi
}

echo ""
echo "=== MicroFoundry AWS ECS Fargate + EKS Deployment ==="
echo ""

info "Checking prerequisites..."

MISSING=0
check_command "terraform" "https://developer.hashicorp.com/terraform/install" || MISSING=1
check_command "aws"       "https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html" || MISSING=1

if [[ "$MISSING" -ne 0 ]]; then
  error "Missing required tools. Please install them and try again."
  exit 1
fi

ok "terraform $(terraform version -json 2>/dev/null | grep -o '"terraform_version":"[^"]*"' | cut -d'"' -f4 || terraform version | head -1)"
ok "aws $(aws --version 2>&1 | awk '{print $1}' | cut -d/ -f2)"

# -- Verify AWS credentials ---------------------------------------------------

info "Verifying AWS credentials..."

if ! aws sts get-caller-identity &>/dev/null; then
  error "AWS credentials are not configured or have expired."
  error "Run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY."
  exit 1
fi

AWS_ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
AWS_REGION=$(aws configure get region 2>/dev/null || echo "us-east-1")
ok "Authenticated to AWS account ${AWS_ACCOUNT} (region: ${AWS_REGION})"

# -- Verify terraform.tfvars exists --------------------------------------------

if [[ ! -f "${TF_DIR}/terraform.tfvars" ]]; then
  warn "No terraform.tfvars found in ${TF_DIR}."
  warn "Create one with at least 'domain' and 'mf_version' variables, for example:"
  echo ""
  echo '  domain     = "apps.example.com"'
  echo '  mf_version = "0.1.0"'
  echo ""
  warn "Proceeding anyway — Terraform will prompt for required variables."
fi

# -- Terraform Init ------------------------------------------------------------

echo ""
info "[1/3] Running terraform init..."
terraform -chdir="$TF_DIR" init

# -- Terraform Plan ------------------------------------------------------------

echo ""
info "[2/3] Running terraform plan..."
terraform -chdir="$TF_DIR" plan -out=tfplan

# -- Terraform Apply -----------------------------------------------------------

echo ""
info "[3/3] Running terraform apply..."

if [[ -n "$AUTO_APPROVE" ]]; then
  terraform -chdir="$TF_DIR" apply $AUTO_APPROVE tfplan
else
  terraform -chdir="$TF_DIR" apply tfplan
fi

# -- Print summary -------------------------------------------------------------

echo ""
echo "==========================================="
echo "  MicroFoundry deployment complete"
echo "==========================================="
echo ""

ALB_URL=$(terraform -chdir="$TF_DIR" output -raw alb_url 2>/dev/null || echo "(pending)")
ECS_CLUSTER=$(terraform -chdir="$TF_DIR" output -raw ecs_cluster_name 2>/dev/null || echo "(pending)")
EKS_CLUSTER=$(terraform -chdir="$TF_DIR" output -raw eks_cluster_name 2>/dev/null || echo "(pending)")
ECR_URL=$(terraform -chdir="$TF_DIR" output -raw ecr_repository_url 2>/dev/null || echo "(pending)")
KUBECONFIG_CMD=$(terraform -chdir="$TF_DIR" output -raw kubeconfig_command 2>/dev/null || echo "(pending)")

echo "  Admin UI:        ${ALB_URL}"
echo "  ECS Cluster:     ${ECS_CLUSTER}"
echo "  EKS Cluster:     ${EKS_CLUSTER}"
echo "  ECR Repository:  ${ECR_URL}"
echo ""
echo "  Configure kubectl for the workload cluster:"
echo "    ${KUBECONFIG_CMD}"
echo ""
echo "  View Fargate logs:"
echo "    aws logs tail /ecs/microfoundry --follow"
echo ""
echo "  Estimated cost:  ~\$108/month (FARGATE_SPOT + 1x t3.small EKS node)"
echo ""
echo "  Next steps:"
echo "    1. Push the MicroFoundry image to ECR:"
echo "       aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_URL%%/*}"
echo "       docker tag mf:latest ${ECR_URL}:<version>"
echo "       docker push ${ECR_URL}:<version>"
echo "    2. Copy mf.example.yaml to EFS as mf.yaml and customize it"
echo "    3. Access the admin UI at: ${ALB_URL}"
echo ""
