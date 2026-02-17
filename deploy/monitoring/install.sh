#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== MicroFoundry Monitoring Stack ==="
echo ""

# Create namespace
echo "[1/6] Creating monitoring namespace..."
kubectl apply -f namespace.yaml

# Add Helm repos
echo "[2/6] Adding Helm repositories..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo add grafana https://grafana.github.io/helm-charts 2>/dev/null || true
helm repo update

# Install kube-prometheus-stack
echo "[3/6] Installing kube-prometheus-stack (Prometheus + Grafana + Alertmanager)..."
helm upgrade --install kube-prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values kube-prometheus-values.yaml \
  --wait --timeout 10m

# Deploy Grafana Beyla (eBPF auto-instrumentation)
echo "[4/6] Deploying Grafana Beyla (eBPF auto-instrumentation)..."
kubectl apply -f beyla-config.yaml
kubectl apply -f beyla-networkpolicy.yaml
kubectl apply -f prometheus-recording-rules.yaml
echo "  Beyla DaemonSet + NetworkPolicy + recording rules applied"

# Install Loki + Promtail
echo "[5/6] Installing Loki + Promtail..."
helm upgrade --install loki grafana/loki-stack \
  --namespace monitoring \
  --values loki-values.yaml \
  --wait --timeout 5m

# Apply dashboards and alert rules
echo "[6/6] Applying dashboards and alert rules..."
kubectl apply -f dashboards/
kubectl apply -f alerts/

echo ""
echo "=== Monitoring stack installed successfully ==="
echo ""
echo "Grafana:      https://grafana.cf-local.dev (admin/microfoundry)"
echo "Prometheus:   kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-prometheus 9090:9090"
echo "Alertmanager: kubectl port-forward -n monitoring svc/kube-prometheus-kube-prome-alertmanager 9093:9093"
echo "Beyla:        kubectl get ds beyla -n monitoring"
echo ""
echo "Make sure grafana.cf-local.dev resolves to 127.0.0.1 in your /etc/hosts file."
echo "Run 'mf setup tls' to generate TLS certificates for HTTPS access."
