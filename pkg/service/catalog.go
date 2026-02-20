package service

import "github.com/younjinjeong/microfoundry/pkg/models"

// Catalog returns all available service types grouped by category.
func Catalog() []models.ServiceType {
	return []models.ServiceType{
		// Database
		{
			ID: "mariadb", Name: "MariaDB", Label: "mariadb",
			Description: "Open-source relational database, MySQL-compatible",
			Provider: "local", Category: "database",
			Tags:  []string{"mysql", "mariadb", "relational", "database"},
			Plans: dbPlans("MariaDB"),
		},
		{
			ID: "postgresql", Name: "PostgreSQL", Label: "postgresql",
			Description: "Advanced open-source relational database",
			Provider: "local", Category: "database",
			Tags:  []string{"postgresql", "postgres", "relational", "database"},
			Plans: dbPlans("PostgreSQL"),
		},
		{
			ID: "clickhouse", Name: "ClickHouse", Label: "clickhouse",
			Description: "Column-oriented OLAP database for real-time analytics",
			Provider: "local", Category: "database",
			Tags:  []string{"clickhouse", "analytics", "columnar", "database"},
			Plans: dbPlans("ClickHouse"),
		},
		// Cache
		{
			ID: "redis", Name: "Redis", Label: "redis",
			Description: "In-memory data store for caching, sessions, and pub/sub",
			Provider: "local", Category: "cache",
			Tags:  []string{"redis", "cache", "key-value", "in-memory"},
			Plans: cachePlans("Redis"),
		},
		{
			ID: "memcached", Name: "Memcached", Label: "memcached",
			Description: "High-performance distributed memory caching system",
			Provider: "local", Category: "cache",
			Tags:  []string{"memcached", "cache", "key-value", "in-memory"},
			Plans: cachePlans("Memcached"),
		},
		// Messaging
		{
			ID: "rabbitmq", Name: "RabbitMQ", Label: "rabbitmq",
			Description: "Open-source message broker with management console",
			Provider: "local", Category: "messaging",
			Tags:  []string{"rabbitmq", "amqp", "message-queue", "messaging"},
			Plans: standardPlans("RabbitMQ"),
		},
		{
			ID: "activemq", Name: "ActiveMQ Artemis", Label: "activemq",
			Description: "High-performance multi-protocol messaging broker",
			Provider: "local", Category: "messaging",
			Tags:  []string{"activemq", "artemis", "jms", "message-queue", "messaging"},
			Plans: standardPlans("ActiveMQ"),
		},
		// Storage
		{
			ID: "minio", Name: "MinIO", Label: "minio",
			Description: "S3-compatible object storage for local development",
			Provider: "local", Category: "storage",
			Tags:  []string{"minio", "s3", "object-storage", "storage"},
			Plans: storagePlans("MinIO"),
		},
		// API Gateway
		{
			ID: "kong", Name: "Kong Gateway", Label: "kong",
			Description: "Cloud-native API gateway and service mesh",
			Provider: "local", Category: "gateway",
			Tags:  []string{"kong", "api-gateway", "proxy", "gateway"},
			Plans: gatewayPlans("Kong"),
		},
		{
			ID: "nginx", Name: "Nginx", Label: "nginx",
			Description: "High-performance HTTP server and reverse proxy",
			Provider: "local", Category: "gateway",
			Tags:  []string{"nginx", "http", "reverse-proxy", "gateway"},
			Plans: gatewayPlans("Nginx"),
		},

		// ── AWS Managed Services (provisioned via Terraform topologies) ──
		{
			ID: "aws-rds-postgresql", Name: "Amazon RDS PostgreSQL", Label: "aws-rds-postgresql",
			Description: "Managed PostgreSQL with automated backups, Multi-AZ, and read replicas",
			Provider: "aws", Category: "database",
			Tags:  []string{"aws", "rds", "postgresql", "managed", "database"},
			Plans: awsDBPlans("RDS PostgreSQL"),
		},
		{
			ID: "aws-rds-mysql", Name: "Amazon RDS MySQL", Label: "aws-rds-mysql",
			Description: "Managed MySQL with automated backups and Multi-AZ failover",
			Provider: "aws", Category: "database",
			Tags:  []string{"aws", "rds", "mysql", "managed", "database"},
			Plans: awsDBPlans("RDS MySQL"),
		},
		{
			ID: "aws-elasticache-redis", Name: "Amazon ElastiCache Redis", Label: "aws-elasticache-redis",
			Description: "Managed Redis with replication, persistence, and cluster mode",
			Provider: "aws", Category: "cache",
			Tags:  []string{"aws", "elasticache", "redis", "managed", "cache"},
			Plans: awsCachePlans("ElastiCache Redis"),
		},
		{
			ID: "aws-elasticache-memcached", Name: "Amazon ElastiCache Memcached", Label: "aws-elasticache-memcached",
			Description: "Managed Memcached for simple distributed caching",
			Provider: "aws", Category: "cache",
			Tags:  []string{"aws", "elasticache", "memcached", "managed", "cache"},
			Plans: awsCachePlans("ElastiCache Memcached"),
		},
		{
			ID: "aws-msk", Name: "Amazon MSK", Label: "aws-msk",
			Description: "Managed Apache Kafka with built-in monitoring and security",
			Provider: "aws", Category: "messaging",
			Tags:  []string{"aws", "msk", "kafka", "managed", "messaging"},
			Plans: awsMessagingPlans("MSK"),
		},
		{
			ID: "aws-s3", Name: "Amazon S3", Label: "aws-s3",
			Description: "Scalable object storage with lifecycle policies",
			Provider: "aws", Category: "storage",
			Tags:  []string{"aws", "s3", "object-storage", "managed", "storage"},
			Plans: awsStoragePlans("S3"),
		},
		{
			ID: "aws-opensearch", Name: "Amazon OpenSearch", Label: "aws-opensearch",
			Description: "Managed OpenSearch for log analytics and full-text search",
			Provider: "aws", Category: "database",
			Tags:  []string{"aws", "opensearch", "elasticsearch", "managed", "search"},
			Plans: awsSearchPlans("OpenSearch"),
		},
	}
}

func dbPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Small (Dev)", Description: "Single instance for development — " + name,
			Resources: models.PlanResources{MemoryMB: 256, CPUMillis: 250, StorageGB: 1}, CostEstimate: "Free (local)"},
		{ID: "medium", Name: "Medium (Staging)", Description: "Moderate resources for staging — " + name,
			Resources: models.PlanResources{MemoryMB: 512, CPUMillis: 500, StorageGB: 5}, CostEstimate: "Free (local)"},
		{ID: "large", Name: "Large (Production)", Description: "Production-grade resources — " + name,
			Resources: models.PlanResources{MemoryMB: 1024, CPUMillis: 1000, StorageGB: 10}, CostEstimate: "Free (local)"},
	}
}

func cachePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Small (Dev)", Description: "256MB cache — " + name,
			Resources: models.PlanResources{MemoryMB: 256, CPUMillis: 250}, CostEstimate: "Free (local)"},
		{ID: "medium", Name: "Medium (Staging)", Description: "512MB cache — " + name,
			Resources: models.PlanResources{MemoryMB: 512, CPUMillis: 500}, CostEstimate: "Free (local)"},
		{ID: "large", Name: "Large (Production)", Description: "1GB cache — " + name,
			Resources: models.PlanResources{MemoryMB: 1024, CPUMillis: 1000}, CostEstimate: "Free (local)"},
	}
}

func storagePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Small (Dev)", Description: "1GB storage — " + name,
			Resources: models.PlanResources{MemoryMB: 256, CPUMillis: 250, StorageGB: 1}, CostEstimate: "Free (local)"},
		{ID: "medium", Name: "Medium (Staging)", Description: "10GB storage — " + name,
			Resources: models.PlanResources{MemoryMB: 512, CPUMillis: 500, StorageGB: 10}, CostEstimate: "Free (local)"},
		{ID: "large", Name: "Large (Production)", Description: "50GB storage — " + name,
			Resources: models.PlanResources{MemoryMB: 1024, CPUMillis: 1000, StorageGB: 50}, CostEstimate: "Free (local)"},
	}
}

func standardPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Small (Dev)", Description: "Minimal resources — " + name,
			Resources: models.PlanResources{MemoryMB: 256, CPUMillis: 250}, CostEstimate: "Free (local)"},
		{ID: "medium", Name: "Medium (Staging)", Description: "Moderate resources — " + name,
			Resources: models.PlanResources{MemoryMB: 512, CPUMillis: 500}, CostEstimate: "Free (local)"},
		{ID: "large", Name: "Large (Production)", Description: "Production resources — " + name,
			Resources: models.PlanResources{MemoryMB: 1024, CPUMillis: 1000}, CostEstimate: "Free (local)"},
	}
}

func gatewayPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Small (Dev)", Description: "Minimal resources — " + name,
			Resources: models.PlanResources{MemoryMB: 256, CPUMillis: 250}, CostEstimate: "Free (local)"},
		{ID: "medium", Name: "Medium (Staging)", Description: "Moderate resources — " + name,
			Resources: models.PlanResources{MemoryMB: 512, CPUMillis: 500}, CostEstimate: "Free (local)"},
		{ID: "large", Name: "Large (Production)", Description: "Production resources — " + name,
			Resources: models.PlanResources{MemoryMB: 1024, CPUMillis: 1000}, CostEstimate: "Free (local)"},
	}
}

// ── AWS plan builders ────────────────────────────────────────────────────────

func awsDBPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "db.t3.micro", Description: "1 vCPU, 1 GiB RAM, 20 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 1024, CPUMillis: 1000, StorageGB: 20}, CostEstimate: "~$15/month"},
		{ID: "medium", Name: "db.t3.medium", Description: "2 vCPU, 4 GiB RAM, 100 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 4096, CPUMillis: 2000, StorageGB: 100}, CostEstimate: "~$70/month"},
		{ID: "large", Name: "db.r6g.large", Description: "2 vCPU, 16 GiB RAM, 500 GB, Multi-AZ — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, CPUMillis: 2000, StorageGB: 500}, CostEstimate: "~$350/month"},
	}
}

func awsCachePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "cache.t3.micro", Description: "1 vCPU, 0.5 GiB — " + name,
			Resources: models.PlanResources{MemoryMB: 512, CPUMillis: 1000}, CostEstimate: "~$12/month"},
		{ID: "medium", Name: "cache.t3.medium", Description: "2 vCPU, 3 GiB — " + name,
			Resources: models.PlanResources{MemoryMB: 3072, CPUMillis: 2000}, CostEstimate: "~$50/month"},
		{ID: "large", Name: "cache.r6g.large", Description: "2 vCPU, 13 GiB, replication — " + name,
			Resources: models.PlanResources{MemoryMB: 13312, CPUMillis: 2000}, CostEstimate: "~$200/month"},
	}
}

func awsMessagingPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "kafka.t3.small", Description: "2 brokers, 100 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 2048, CPUMillis: 2000, StorageGB: 100}, CostEstimate: "~$150/month"},
		{ID: "medium", Name: "kafka.m5.large", Description: "3 brokers, 500 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 8192, CPUMillis: 2000, StorageGB: 500}, CostEstimate: "~$450/month"},
		{ID: "large", Name: "kafka.m5.2xlarge", Description: "3 brokers, 1 TB, tiered storage — " + name,
			Resources: models.PlanResources{MemoryMB: 32768, CPUMillis: 8000, StorageGB: 1000}, CostEstimate: "~$1200/month"},
	}
}

func awsStoragePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard", Description: "Standard storage, 5 GB — " + name,
			Resources: models.PlanResources{StorageGB: 5}, CostEstimate: "~$0.12/month"},
		{ID: "medium", Name: "Standard-IA", Description: "Infrequent Access, 100 GB — " + name,
			Resources: models.PlanResources{StorageGB: 100}, CostEstimate: "~$1.25/month"},
		{ID: "large", Name: "Intelligent-Tiering", Description: "Auto-tiered, 1 TB — " + name,
			Resources: models.PlanResources{StorageGB: 1000}, CostEstimate: "~$23/month"},
	}
}

func awsSearchPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "t3.small.search", Description: "1 node, 10 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 2048, CPUMillis: 1000, StorageGB: 10}, CostEstimate: "~$25/month"},
		{ID: "medium", Name: "m6g.large.search", Description: "2 nodes, 100 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 8192, CPUMillis: 2000, StorageGB: 100}, CostEstimate: "~$180/month"},
		{ID: "large", Name: "r6g.xlarge.search", Description: "3 nodes, 500 GB, Multi-AZ — " + name,
			Resources: models.PlanResources{MemoryMB: 32768, CPUMillis: 4000, StorageGB: 500}, CostEstimate: "~$650/month"},
	}
}

// FindServiceType returns a service type by ID.
func FindServiceType(id string) (models.ServiceType, bool) {
	for _, st := range Catalog() {
		if st.ID == id {
			return st, true
		}
	}
	return models.ServiceType{}, false
}

// FindPlan returns a plan within a service type.
func FindPlan(serviceTypeID, planID string) (models.ServicePlan, bool) {
	st, ok := FindServiceType(serviceTypeID)
	if !ok {
		return models.ServicePlan{}, false
	}
	for _, p := range st.Plans {
		if p.ID == planID {
			return p, true
		}
	}
	return models.ServicePlan{}, false
}
