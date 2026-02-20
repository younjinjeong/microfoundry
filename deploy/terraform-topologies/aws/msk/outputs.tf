# MicroFoundry Topology Outputs — Amazon MSK

output "bootstrap_brokers" {
  value = aws_msk_cluster.this.bootstrap_brokers
}

output "bootstrap_brokers_tls" {
  value = aws_msk_cluster.this.bootstrap_brokers_tls
}

output "zookeeper_connect" {
  value = aws_msk_cluster.this.zookeeper_connect_string
}

output "uri" {
  value = aws_msk_cluster.this.bootstrap_brokers
}
