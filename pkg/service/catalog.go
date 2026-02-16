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
