package service

import "github.com/younjinjeong/microfoundry/pkg/models"

// Catalog returns all available service types grouped by category.
// Categories: database, nosql, datawarehouse, cache, messaging, streaming,
//             storage, search, ai, media, gateway
func Catalog() []models.ServiceType {
	var all []models.ServiceType
	all = append(all, localServices()...)
	all = append(all, awsServices()...)
	all = append(all, gcpServices()...)
	all = append(all, azureServices()...)
	return all
}

// ── Local K8s-Native Services ─────────────────────────────────────────────────

func localServices() []models.ServiceType {
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

// ── AWS Managed Services ──────────────────────────────────────────────────────

func awsServices() []models.ServiceType {
	return []models.ServiceType{
		// ── Databases (Relational) ──
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
			ID: "aws-rds-mariadb", Name: "Amazon RDS MariaDB", Label: "aws-rds-mariadb",
			Description: "Managed MariaDB, drop-in MySQL replacement with enhanced features",
			Provider: "aws", Category: "database",
			Tags:  []string{"aws", "rds", "mariadb", "managed", "database"},
			Plans: awsDBPlans("RDS MariaDB"),
		},
		{
			ID: "aws-aurora-postgresql", Name: "Amazon Aurora PostgreSQL", Label: "aws-aurora-postgresql",
			Description: "PostgreSQL-compatible with up to 5x throughput, auto-scaling storage",
			Provider: "aws", Category: "database",
			Tags:  []string{"aws", "aurora", "postgresql", "managed", "database"},
			Plans: awsAuroraPlans("Aurora PostgreSQL"),
		},
		{
			ID: "aws-aurora-mysql", Name: "Amazon Aurora MySQL", Label: "aws-aurora-mysql",
			Description: "MySQL-compatible with up to 5x throughput, serverless v2 available",
			Provider: "aws", Category: "database",
			Tags:  []string{"aws", "aurora", "mysql", "managed", "database"},
			Plans: awsAuroraPlans("Aurora MySQL"),
		},

		// ── Databases (NoSQL) ──
		{
			ID: "aws-dynamodb", Name: "Amazon DynamoDB", Label: "aws-dynamodb",
			Description: "Serverless key-value and document database with single-digit ms latency",
			Provider: "aws", Category: "nosql",
			Tags:  []string{"aws", "dynamodb", "nosql", "key-value", "managed"},
			Plans: awsNoSQLPlans("DynamoDB"),
		},
		{
			ID: "aws-documentdb", Name: "Amazon DocumentDB", Label: "aws-documentdb",
			Description: "MongoDB-compatible managed document database with replication",
			Provider: "aws", Category: "nosql",
			Tags:  []string{"aws", "documentdb", "mongodb", "nosql", "managed"},
			Plans: awsDBPlans("DocumentDB"),
		},

		// ── Data Warehouses ──
		{
			ID: "aws-redshift", Name: "Amazon Redshift", Label: "aws-redshift",
			Description: "Petabyte-scale columnar data warehouse with Spectrum for S3 queries",
			Provider: "aws", Category: "datawarehouse",
			Tags:  []string{"aws", "redshift", "data-warehouse", "analytics", "managed"},
			Plans: awsDWPlans("Redshift"),
		},

		// ── Caches ──
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

		// ── Message Queues ──
		{
			ID: "aws-sqs", Name: "Amazon SQS", Label: "aws-sqs",
			Description: "Fully managed message queuing with standard and FIFO queues",
			Provider: "aws", Category: "messaging",
			Tags:  []string{"aws", "sqs", "queue", "managed", "messaging"},
			Plans: awsQueuePlans("SQS"),
		},
		{
			ID: "aws-sns", Name: "Amazon SNS", Label: "aws-sns",
			Description: "Managed pub/sub messaging and mobile push notifications",
			Provider: "aws", Category: "messaging",
			Tags:  []string{"aws", "sns", "pubsub", "managed", "messaging"},
			Plans: awsQueuePlans("SNS"),
		},
		{
			ID: "aws-mq-rabbitmq", Name: "Amazon MQ for RabbitMQ", Label: "aws-mq-rabbitmq",
			Description: "Managed RabbitMQ message broker with AMQP support",
			Provider: "aws", Category: "messaging",
			Tags:  []string{"aws", "mq", "rabbitmq", "amqp", "managed", "messaging"},
			Plans: awsBrokerPlans("MQ RabbitMQ"),
		},

		// ── Stream / Event Processing ──
		{
			ID: "aws-msk", Name: "Amazon MSK", Label: "aws-msk",
			Description: "Managed Apache Kafka with built-in monitoring and security",
			Provider: "aws", Category: "streaming",
			Tags:  []string{"aws", "msk", "kafka", "managed", "streaming"},
			Plans: awsMessagingPlans("MSK"),
		},
		{
			ID: "aws-kinesis", Name: "Amazon Kinesis Data Streams", Label: "aws-kinesis",
			Description: "Real-time data streaming at scale with per-shard throughput",
			Provider: "aws", Category: "streaming",
			Tags:  []string{"aws", "kinesis", "streaming", "real-time", "managed"},
			Plans: awsStreamPlans("Kinesis"),
		},

		// ── Object Storage ──
		{
			ID: "aws-s3", Name: "Amazon S3", Label: "aws-s3",
			Description: "Scalable object storage with lifecycle policies and versioning",
			Provider: "aws", Category: "storage",
			Tags:  []string{"aws", "s3", "object-storage", "managed", "storage"},
			Plans: awsStoragePlans("S3"),
		},

		// ── Search / Analytics ──
		{
			ID: "aws-opensearch", Name: "Amazon OpenSearch", Label: "aws-opensearch",
			Description: "Managed OpenSearch for log analytics and full-text search",
			Provider: "aws", Category: "search",
			Tags:  []string{"aws", "opensearch", "elasticsearch", "managed", "search"},
			Plans: awsSearchPlans("OpenSearch"),
		},

		// ── AI / ML Services ──
		{
			ID: "aws-bedrock", Name: "Amazon Bedrock", Label: "aws-bedrock",
			Description: "Managed foundation models: Claude, Titan, Llama, Mistral, and more",
			Provider: "aws", Category: "ai",
			Tags:  []string{"aws", "bedrock", "llm", "ai", "managed", "generative-ai"},
			Plans: awsAIPlans("Bedrock"),
		},
		{
			ID: "aws-sagemaker", Name: "Amazon SageMaker", Label: "aws-sagemaker",
			Description: "Build, train, and deploy ML models at scale with managed infrastructure",
			Provider: "aws", Category: "ai",
			Tags:  []string{"aws", "sagemaker", "ml", "ai", "managed", "training"},
			Plans: awsSageMakerPlans("SageMaker"),
		},

		// ── Media Services ──
		{
			ID: "aws-mediaconvert", Name: "AWS MediaConvert", Label: "aws-mediaconvert",
			Description: "File-based video transcoding with broadcast-grade features",
			Provider: "aws", Category: "media",
			Tags:  []string{"aws", "mediaconvert", "video", "transcoding", "managed", "media"},
			Plans: awsMediaPlans("MediaConvert"),
		},
		{
			ID: "aws-ivs", Name: "Amazon IVS", Label: "aws-ivs",
			Description: "Managed live streaming with ultra-low latency",
			Provider: "aws", Category: "media",
			Tags:  []string{"aws", "ivs", "live-streaming", "video", "managed", "media"},
			Plans: awsMediaPlans("IVS"),
		},
	}
}

// ── GCP Managed Services ──────────────────────────────────────────────────────

func gcpServices() []models.ServiceType {
	return []models.ServiceType{
		// ── Databases (Relational) ──
		{
			ID: "gcp-cloudsql-postgresql", Name: "Cloud SQL for PostgreSQL", Label: "gcp-cloudsql-postgresql",
			Description: "Fully managed PostgreSQL with HA, automated backups, and IAM auth",
			Provider: "gcp", Category: "database",
			Tags:  []string{"gcp", "cloudsql", "postgresql", "managed", "database"},
			Plans: gcpDBPlans("Cloud SQL PostgreSQL"),
		},
		{
			ID: "gcp-cloudsql-mysql", Name: "Cloud SQL for MySQL", Label: "gcp-cloudsql-mysql",
			Description: "Fully managed MySQL with HA and automatic storage increases",
			Provider: "gcp", Category: "database",
			Tags:  []string{"gcp", "cloudsql", "mysql", "managed", "database"},
			Plans: gcpDBPlans("Cloud SQL MySQL"),
		},
		{
			ID: "gcp-alloydb", Name: "AlloyDB for PostgreSQL", Label: "gcp-alloydb",
			Description: "PostgreSQL-compatible, 4x faster than standard PostgreSQL, columnar engine",
			Provider: "gcp", Category: "database",
			Tags:  []string{"gcp", "alloydb", "postgresql", "managed", "database"},
			Plans: gcpAlloyDBPlans("AlloyDB"),
		},
		{
			ID: "gcp-spanner", Name: "Cloud Spanner", Label: "gcp-spanner",
			Description: "Globally distributed, strongly consistent relational database",
			Provider: "gcp", Category: "database",
			Tags:  []string{"gcp", "spanner", "relational", "global", "managed", "database"},
			Plans: gcpSpannerPlans("Spanner"),
		},

		// ── Databases (NoSQL) ──
		{
			ID: "gcp-firestore", Name: "Cloud Firestore", Label: "gcp-firestore",
			Description: "Serverless NoSQL document database with real-time sync",
			Provider: "gcp", Category: "nosql",
			Tags:  []string{"gcp", "firestore", "nosql", "document", "managed"},
			Plans: gcpNoSQLPlans("Firestore"),
		},
		{
			ID: "gcp-bigtable", Name: "Cloud Bigtable", Label: "gcp-bigtable",
			Description: "Managed wide-column NoSQL for analytical and operational workloads",
			Provider: "gcp", Category: "nosql",
			Tags:  []string{"gcp", "bigtable", "nosql", "wide-column", "managed"},
			Plans: gcpBigtablePlans("Bigtable"),
		},

		// ── Data Warehouses ──
		{
			ID: "gcp-bigquery", Name: "BigQuery", Label: "gcp-bigquery",
			Description: "Serverless multi-cloud data warehouse with built-in ML and BI",
			Provider: "gcp", Category: "datawarehouse",
			Tags:  []string{"gcp", "bigquery", "data-warehouse", "analytics", "managed"},
			Plans: gcpDWPlans("BigQuery"),
		},

		// ── Caches ──
		{
			ID: "gcp-memorystore-redis", Name: "Memorystore for Redis", Label: "gcp-memorystore-redis",
			Description: "Fully managed Redis with sub-ms latency and 99.9% SLA",
			Provider: "gcp", Category: "cache",
			Tags:  []string{"gcp", "memorystore", "redis", "managed", "cache"},
			Plans: gcpCachePlans("Memorystore Redis"),
		},

		// ── Message Queues ──
		{
			ID: "gcp-pubsub", Name: "Cloud Pub/Sub", Label: "gcp-pubsub",
			Description: "Global real-time messaging with at-least-once delivery",
			Provider: "gcp", Category: "messaging",
			Tags:  []string{"gcp", "pubsub", "messaging", "managed", "event-driven"},
			Plans: gcpMessagingPlans("Pub/Sub"),
		},

		// ── Object Storage ──
		{
			ID: "gcp-gcs", Name: "Cloud Storage", Label: "gcp-gcs",
			Description: "Unified object storage with multi-region and auto-class tiering",
			Provider: "gcp", Category: "storage",
			Tags:  []string{"gcp", "gcs", "object-storage", "managed", "storage"},
			Plans: gcpStoragePlans("Cloud Storage"),
		},

		// ── AI / ML Services ──
		{
			ID: "gcp-vertex-ai", Name: "Vertex AI", Label: "gcp-vertex-ai",
			Description: "Unified ML platform with model garden, Gemini, and custom training",
			Provider: "gcp", Category: "ai",
			Tags:  []string{"gcp", "vertex-ai", "gemini", "ml", "ai", "managed"},
			Plans: gcpAIPlans("Vertex AI"),
		},

		// ── Media Services ──
		{
			ID: "gcp-transcoder", Name: "Transcoder API", Label: "gcp-transcoder",
			Description: "Serverless video transcoding with job templates",
			Provider: "gcp", Category: "media",
			Tags:  []string{"gcp", "transcoder", "video", "managed", "media"},
			Plans: gcpMediaPlans("Transcoder"),
		},
	}
}

// ── Azure Managed Services ────────────────────────────────────────────────────

func azureServices() []models.ServiceType {
	return []models.ServiceType{
		// ── Databases (Relational) ──
		{
			ID: "azure-db-postgresql", Name: "Azure Database for PostgreSQL", Label: "azure-db-postgresql",
			Description: "Managed PostgreSQL Flexible Server with zone-redundant HA",
			Provider: "azure", Category: "database",
			Tags:  []string{"azure", "postgresql", "flexible-server", "managed", "database"},
			Plans: azureDBPlans("PostgreSQL Flexible"),
		},
		{
			ID: "azure-db-mysql", Name: "Azure Database for MySQL", Label: "azure-db-mysql",
			Description: "Managed MySQL Flexible Server with zone-redundant HA and read replicas",
			Provider: "azure", Category: "database",
			Tags:  []string{"azure", "mysql", "flexible-server", "managed", "database"},
			Plans: azureDBPlans("MySQL Flexible"),
		},
		{
			ID: "azure-sql-database", Name: "Azure SQL Database", Label: "azure-sql-database",
			Description: "Managed SQL Server database with intelligent performance tuning",
			Provider: "azure", Category: "database",
			Tags:  []string{"azure", "sql-server", "managed", "database"},
			Plans: azureSQLPlans("SQL Database"),
		},

		// ── Databases (NoSQL) ──
		{
			ID: "azure-cosmosdb", Name: "Azure Cosmos DB", Label: "azure-cosmosdb",
			Description: "Globally distributed multi-model database with single-digit ms latency",
			Provider: "azure", Category: "nosql",
			Tags:  []string{"azure", "cosmosdb", "nosql", "multi-model", "managed"},
			Plans: azureNoSQLPlans("Cosmos DB"),
		},

		// ── Data Warehouses ──
		{
			ID: "azure-synapse", Name: "Azure Synapse Analytics", Label: "azure-synapse",
			Description: "Unified analytics with data warehouse, Spark, and data integration",
			Provider: "azure", Category: "datawarehouse",
			Tags:  []string{"azure", "synapse", "data-warehouse", "analytics", "managed"},
			Plans: azureDWPlans("Synapse"),
		},

		// ── Caches ──
		{
			ID: "azure-cache-redis", Name: "Azure Cache for Redis", Label: "azure-cache-redis",
			Description: "Managed Redis with clustering, geo-replication, and persistence",
			Provider: "azure", Category: "cache",
			Tags:  []string{"azure", "redis", "cache", "managed"},
			Plans: azureCachePlans("Azure Cache Redis"),
		},

		// ── Message Queues ──
		{
			ID: "azure-service-bus", Name: "Azure Service Bus", Label: "azure-service-bus",
			Description: "Enterprise messaging with queues, topics, and sessions",
			Provider: "azure", Category: "messaging",
			Tags:  []string{"azure", "service-bus", "messaging", "managed", "enterprise"},
			Plans: azureMessagingPlans("Service Bus"),
		},

		// ── Stream / Event Processing ──
		{
			ID: "azure-event-hubs", Name: "Azure Event Hubs", Label: "azure-event-hubs",
			Description: "Big data streaming platform, Apache Kafka compatible",
			Provider: "azure", Category: "streaming",
			Tags:  []string{"azure", "event-hubs", "kafka", "streaming", "managed"},
			Plans: azureStreamingPlans("Event Hubs"),
		},

		// ── Object Storage ──
		{
			ID: "azure-blob-storage", Name: "Azure Blob Storage", Label: "azure-blob-storage",
			Description: "Massively scalable object storage for unstructured data",
			Provider: "azure", Category: "storage",
			Tags:  []string{"azure", "blob", "object-storage", "managed", "storage"},
			Plans: azureStoragePlans("Blob Storage"),
		},

		// ── Search / Analytics ──
		{
			ID: "azure-ai-search", Name: "Azure AI Search", Label: "azure-ai-search",
			Description: "AI-powered cloud search with vector search and semantic ranking",
			Provider: "azure", Category: "search",
			Tags:  []string{"azure", "cognitive-search", "ai-search", "managed", "search"},
			Plans: azureSearchPlans("AI Search"),
		},

		// ── AI / ML Services ──
		{
			ID: "azure-openai", Name: "Azure OpenAI Service", Label: "azure-openai",
			Description: "Managed access to GPT-4o, GPT-4, DALL-E, and Whisper models",
			Provider: "azure", Category: "ai",
			Tags:  []string{"azure", "openai", "gpt", "ai", "managed", "generative-ai"},
			Plans: azureAIPlans("Azure OpenAI"),
		},
		{
			ID: "azure-ml", Name: "Azure Machine Learning", Label: "azure-ml",
			Description: "End-to-end ML lifecycle with training, deployment, and MLOps",
			Provider: "azure", Category: "ai",
			Tags:  []string{"azure", "machine-learning", "ml", "ai", "managed"},
			Plans: azureMLPlans("Azure ML"),
		},

		// ── Media Services ──
		{
			ID: "azure-media-services", Name: "Azure Media Services", Label: "azure-media-services",
			Description: "End-to-end media workflow: encode, package, stream, and protect",
			Provider: "azure", Category: "media",
			Tags:  []string{"azure", "media-services", "video", "streaming", "managed", "media"},
			Plans: azureMediaPlans("Media Services"),
		},
	}
}

// ── Local plan builders ───────────────────────────────────────────────────────

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

// ── AWS plan builders ─────────────────────────────────────────────────────────

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

func awsAuroraPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "db.t3.medium", Description: "2 vCPU, 4 GiB RAM, 20 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 4096, CPUMillis: 2000, StorageGB: 20}, CostEstimate: "~$60/month"},
		{ID: "medium", Name: "db.r6g.large", Description: "2 vCPU, 16 GiB RAM, 100 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, CPUMillis: 2000, StorageGB: 100}, CostEstimate: "~$250/month"},
		{ID: "large", Name: "db.r6g.xlarge", Description: "4 vCPU, 32 GiB RAM, 500 GB, Multi-AZ — " + name,
			Resources: models.PlanResources{MemoryMB: 32768, CPUMillis: 4000, StorageGB: 500}, CostEstimate: "~$600/month"},
	}
}

func awsNoSQLPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "On-Demand", Description: "Pay-per-request, 25 GB free tier — " + name,
			Resources: models.PlanResources{StorageGB: 25}, CostEstimate: "~$0/month (free tier)"},
		{ID: "medium", Name: "Provisioned", Description: "25 RCU / 25 WCU, 50 GB — " + name,
			Resources: models.PlanResources{StorageGB: 50}, CostEstimate: "~$15/month"},
		{ID: "large", Name: "Provisioned+DAX", Description: "100 RCU / 100 WCU, 250 GB, DAX cache — " + name,
			Resources: models.PlanResources{MemoryMB: 2048, StorageGB: 250}, CostEstimate: "~$75/month"},
	}
}

func awsDWPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "dc2.large", Description: "1 node, 2 vCPU, 15 GiB, 160 GB SSD — " + name,
			Resources: models.PlanResources{MemoryMB: 15360, CPUMillis: 2000, StorageGB: 160}, CostEstimate: "~$180/month"},
		{ID: "medium", Name: "ra3.xlplus", Description: "2 nodes, 4 vCPU, 32 GiB, 1 TB managed — " + name,
			Resources: models.PlanResources{MemoryMB: 32768, CPUMillis: 4000, StorageGB: 1000}, CostEstimate: "~$800/month"},
		{ID: "large", Name: "ra3.4xlarge", Description: "2 nodes, 12 vCPU, 96 GiB, 4 TB managed — " + name,
			Resources: models.PlanResources{MemoryMB: 98304, CPUMillis: 12000, StorageGB: 4000}, CostEstimate: "~$2400/month"},
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

func awsQueuePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard", Description: "Standard queue, 1M requests/mo free — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0/month (free tier)"},
		{ID: "medium", Name: "Standard", Description: "Unlimited throughput — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.40/1M requests"},
		{ID: "large", Name: "FIFO", Description: "Ordered, exactly-once delivery — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.50/1M requests"},
	}
}

func awsBrokerPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "mq.m5.large", Description: "Single broker, 2 vCPU, 8 GiB — " + name,
			Resources: models.PlanResources{MemoryMB: 8192, CPUMillis: 2000}, CostEstimate: "~$150/month"},
		{ID: "medium", Name: "mq.m5.large", Description: "Active/standby, 2 vCPU, 8 GiB — " + name,
			Resources: models.PlanResources{MemoryMB: 8192, CPUMillis: 2000}, CostEstimate: "~$300/month"},
		{ID: "large", Name: "mq.m5.2xlarge", Description: "Active/standby, 8 vCPU, 32 GiB — " + name,
			Resources: models.PlanResources{MemoryMB: 32768, CPUMillis: 8000}, CostEstimate: "~$600/month"},
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

func awsStreamPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "On-Demand (1 shard)", Description: "4 MB/s in, 8 MB/s out — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$36/month"},
		{ID: "medium", Name: "On-Demand (4 shards)", Description: "16 MB/s in, 32 MB/s out — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$144/month"},
		{ID: "large", Name: "On-Demand (16 shards)", Description: "64 MB/s in, 128 MB/s out — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$576/month"},
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

func awsAIPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "On-Demand", Description: "Pay-per-token, Haiku / Titan Lite — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~usage-based"},
		{ID: "medium", Name: "On-Demand", Description: "Pay-per-token, Sonnet / Titan Express — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~usage-based"},
		{ID: "large", Name: "Provisioned Throughput", Description: "Reserved model units for Opus / GPT — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$1980/month per PTU"},
	}
}

func awsSageMakerPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "ml.t3.medium", Description: "2 vCPU, 4 GiB endpoint — " + name,
			Resources: models.PlanResources{MemoryMB: 4096, CPUMillis: 2000}, CostEstimate: "~$50/month"},
		{ID: "medium", Name: "ml.m5.xlarge", Description: "4 vCPU, 16 GiB endpoint — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, CPUMillis: 4000}, CostEstimate: "~$200/month"},
		{ID: "large", Name: "ml.g5.xlarge", Description: "4 vCPU, 16 GiB, 1 GPU endpoint — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, CPUMillis: 4000}, CostEstimate: "~$1000/month"},
	}
}

func awsMediaPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "On-Demand", Description: "SD transcoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.015/min"},
		{ID: "medium", Name: "On-Demand", Description: "HD transcoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.030/min"},
		{ID: "large", Name: "Reserved", Description: "4K/UHD transcoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.060/min"},
	}
}

// ── GCP plan builders ─────────────────────────────────────────────────────────

func gcpDBPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "db-f1-micro", Description: "Shared vCPU, 0.6 GiB, 10 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 614, CPUMillis: 250, StorageGB: 10}, CostEstimate: "~$8/month"},
		{ID: "medium", Name: "db-custom-2-7680", Description: "2 vCPU, 7.5 GiB, 100 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 7680, CPUMillis: 2000, StorageGB: 100}, CostEstimate: "~$75/month"},
		{ID: "large", Name: "db-custom-4-15360", Description: "4 vCPU, 15 GiB, 500 GB, HA — " + name,
			Resources: models.PlanResources{MemoryMB: 15360, CPUMillis: 4000, StorageGB: 500}, CostEstimate: "~$350/month"},
	}
}

func gcpAlloyDBPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "2 vCPU", Description: "2 vCPU, 16 GiB, 100 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, CPUMillis: 2000, StorageGB: 100}, CostEstimate: "~$200/month"},
		{ID: "medium", Name: "4 vCPU", Description: "4 vCPU, 32 GiB, 500 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 32768, CPUMillis: 4000, StorageGB: 500}, CostEstimate: "~$500/month"},
		{ID: "large", Name: "8 vCPU", Description: "8 vCPU, 64 GiB, 1 TB, HA — " + name,
			Resources: models.PlanResources{MemoryMB: 65536, CPUMillis: 8000, StorageGB: 1000}, CostEstimate: "~$1200/month"},
	}
}

func gcpSpannerPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "100 PU", Description: "100 processing units, 10 GB — " + name,
			Resources: models.PlanResources{StorageGB: 10}, CostEstimate: "~$65/month"},
		{ID: "medium", Name: "1000 PU", Description: "1000 processing units, 100 GB — " + name,
			Resources: models.PlanResources{StorageGB: 100}, CostEstimate: "~$650/month"},
		{ID: "large", Name: "3 nodes", Description: "3 nodes (3000 PU), 1 TB, multi-region — " + name,
			Resources: models.PlanResources{StorageGB: 1000}, CostEstimate: "~$2700/month"},
	}
}

func gcpNoSQLPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Native", Description: "1 GiB storage, 50K reads/day — " + name,
			Resources: models.PlanResources{StorageGB: 1}, CostEstimate: "~$0/month (free tier)"},
		{ID: "medium", Name: "Native", Description: "10 GiB storage — " + name,
			Resources: models.PlanResources{StorageGB: 10}, CostEstimate: "~$18/month"},
		{ID: "large", Name: "Native", Description: "100 GiB storage — " + name,
			Resources: models.PlanResources{StorageGB: 100}, CostEstimate: "~$180/month"},
	}
}

func gcpBigtablePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "1 node", Description: "1 node, 1 TB SSD — " + name,
			Resources: models.PlanResources{StorageGB: 1000}, CostEstimate: "~$460/month"},
		{ID: "medium", Name: "3 nodes", Description: "3 nodes, 5 TB SSD — " + name,
			Resources: models.PlanResources{StorageGB: 5000}, CostEstimate: "~$1400/month"},
		{ID: "large", Name: "5 nodes", Description: "5 nodes, 10 TB SSD, replication — " + name,
			Resources: models.PlanResources{StorageGB: 10000}, CostEstimate: "~$2500/month"},
	}
}

func gcpDWPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "On-Demand", Description: "1 TB queries/mo free, 10 GB storage — " + name,
			Resources: models.PlanResources{StorageGB: 10}, CostEstimate: "~$0/month (free tier)"},
		{ID: "medium", Name: "On-Demand", Description: "10 TB queries/mo, 1 TB storage — " + name,
			Resources: models.PlanResources{StorageGB: 1000}, CostEstimate: "~$75/month"},
		{ID: "large", Name: "Flat-Rate Slots", Description: "100 slots, 10 TB storage — " + name,
			Resources: models.PlanResources{StorageGB: 10000}, CostEstimate: "~$2000/month"},
	}
}

func gcpCachePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "M1 Basic", Description: "1 GiB, no replication — " + name,
			Resources: models.PlanResources{MemoryMB: 1024}, CostEstimate: "~$35/month"},
		{ID: "medium", Name: "M2 Standard", Description: "5 GiB, with replication — " + name,
			Resources: models.PlanResources{MemoryMB: 5120}, CostEstimate: "~$175/month"},
		{ID: "large", Name: "M3 Standard", Description: "16 GiB, replication, HA — " + name,
			Resources: models.PlanResources{MemoryMB: 16384}, CostEstimate: "~$560/month"},
	}
}

func gcpMessagingPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard", Description: "10 GB/mo free tier — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0/month (free tier)"},
		{ID: "medium", Name: "Standard", Description: "100 GB/mo throughput — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$4/month"},
		{ID: "large", Name: "Standard", Description: "1 TB/mo throughput — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$40/month"},
	}
}

func gcpStoragePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard (single)", Description: "Single-region, 5 GB — " + name,
			Resources: models.PlanResources{StorageGB: 5}, CostEstimate: "~$0.10/month"},
		{ID: "medium", Name: "Standard (dual)", Description: "Dual-region, 100 GB — " + name,
			Resources: models.PlanResources{StorageGB: 100}, CostEstimate: "~$3.60/month"},
		{ID: "large", Name: "Standard (multi)", Description: "Multi-region, 1 TB — " + name,
			Resources: models.PlanResources{StorageGB: 1000}, CostEstimate: "~$26/month"},
	}
}

func gcpAIPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "n1-standard-4", Description: "4 vCPU, 15 GiB endpoint — " + name,
			Resources: models.PlanResources{MemoryMB: 15360, CPUMillis: 4000}, CostEstimate: "~$150/month"},
		{ID: "medium", Name: "n1-standard-8", Description: "8 vCPU, 30 GiB endpoint — " + name,
			Resources: models.PlanResources{MemoryMB: 30720, CPUMillis: 8000}, CostEstimate: "~$300/month"},
		{ID: "large", Name: "n1-standard-16+T4", Description: "16 vCPU, 60 GiB, 1 GPU — " + name,
			Resources: models.PlanResources{MemoryMB: 61440, CPUMillis: 16000}, CostEstimate: "~$900/month"},
	}
}

func gcpMediaPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard", Description: "SD transcoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.015/min"},
		{ID: "medium", Name: "Standard", Description: "HD transcoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.030/min"},
		{ID: "large", Name: "Standard", Description: "4K transcoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.060/min"},
	}
}

// ── Azure plan builders ───────────────────────────────────────────────────────

func azureDBPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "B_Standard_B1ms", Description: "1 vCPU, 2 GiB, 32 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 2048, CPUMillis: 1000, StorageGB: 32}, CostEstimate: "~$13/month"},
		{ID: "medium", Name: "GP_Standard_D2ds_v4", Description: "2 vCPU, 8 GiB, 128 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 8192, CPUMillis: 2000, StorageGB: 128}, CostEstimate: "~$100/month"},
		{ID: "large", Name: "GP_Standard_D4ds_v4", Description: "4 vCPU, 16 GiB, 512 GB, zone-redundant — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, CPUMillis: 4000, StorageGB: 512}, CostEstimate: "~$400/month"},
	}
}

func azureSQLPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Basic 5 DTU", Description: "5 DTU, 2 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 2048, StorageGB: 2}, CostEstimate: "~$5/month"},
		{ID: "medium", Name: "Standard S1", Description: "20 DTU, 250 GB — " + name,
			Resources: models.PlanResources{MemoryMB: 4096, StorageGB: 250}, CostEstimate: "~$30/month"},
		{ID: "large", Name: "Premium P1", Description: "125 DTU, 500 GB, zone-redundant — " + name,
			Resources: models.PlanResources{MemoryMB: 16384, StorageGB: 500}, CostEstimate: "~$465/month"},
	}
}

func azureNoSQLPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Serverless", Description: "Pay-per-RU, 1 GB — " + name,
			Resources: models.PlanResources{StorageGB: 1}, CostEstimate: "~$0/month (minimal)"},
		{ID: "medium", Name: "Provisioned 400 RU/s", Description: "400 RU/s, 50 GB, single region — " + name,
			Resources: models.PlanResources{StorageGB: 50}, CostEstimate: "~$25/month"},
		{ID: "large", Name: "Provisioned 4000 RU/s", Description: "4000 RU/s, 250 GB, multi-region — " + name,
			Resources: models.PlanResources{StorageGB: 250}, CostEstimate: "~$290/month"},
	}
}

func azureDWPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "DW100c", Description: "100 cDWU, serverless SQL available — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$1.20/hour"},
		{ID: "medium", Name: "DW500c", Description: "500 cDWU — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$6.00/hour"},
		{ID: "large", Name: "DW1000c", Description: "1000 cDWU — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$12.00/hour"},
	}
}

func azureCachePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "C0 Basic", Description: "250 MB, no SLA — " + name,
			Resources: models.PlanResources{MemoryMB: 256}, CostEstimate: "~$16/month"},
		{ID: "medium", Name: "C1 Standard", Description: "1 GiB, with replication — " + name,
			Resources: models.PlanResources{MemoryMB: 1024}, CostEstimate: "~$42/month"},
		{ID: "large", Name: "P1 Premium", Description: "6 GiB, clustering, persistence, VNet — " + name,
			Resources: models.PlanResources{MemoryMB: 6144}, CostEstimate: "~$172/month"},
	}
}

func azureMessagingPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Basic", Description: "Queues only, 256 KB messages — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.05/1M ops"},
		{ID: "medium", Name: "Standard", Description: "Queues + topics, 256 KB — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$10/month base"},
		{ID: "large", Name: "Premium (1 MU)", Description: "1 MB messages, VNet, geo-DR — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$668/month"},
	}
}

func azureStreamingPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Basic (1 TU)", Description: "1 MB/s in, 2 MB/s out — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$11/month"},
		{ID: "medium", Name: "Standard (4 TU)", Description: "4 MB/s in, 8 MB/s out, Kafka — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$90/month"},
		{ID: "large", Name: "Premium (1 PU)", Description: "Dedicated, 10 MB/s+ — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$750/month"},
	}
}

func azureStoragePlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Hot LRS", Description: "Locally redundant, 5 GB — " + name,
			Resources: models.PlanResources{StorageGB: 5}, CostEstimate: "~$0.10/month"},
		{ID: "medium", Name: "Hot GRS", Description: "Geo-redundant, 100 GB — " + name,
			Resources: models.PlanResources{StorageGB: 100}, CostEstimate: "~$4.60/month"},
		{ID: "large", Name: "Hot RA-GRS", Description: "Read-access geo-redundant, 1 TB — " + name,
			Resources: models.PlanResources{StorageGB: 1000}, CostEstimate: "~$42/month"},
	}
}

func azureSearchPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Free", Description: "3 indexes, 50 MB — " + name,
			Resources: models.PlanResources{StorageGB: 1}, CostEstimate: "~$0/month (free)"},
		{ID: "medium", Name: "Basic", Description: "15 indexes, 2 GB — " + name,
			Resources: models.PlanResources{StorageGB: 2}, CostEstimate: "~$75/month"},
		{ID: "large", Name: "Standard S1", Description: "50 indexes, 25 GB — " + name,
			Resources: models.PlanResources{StorageGB: 25}, CostEstimate: "~$250/month"},
	}
}

func azureAIPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard S0", Description: "Pay-per-token, GPT-4o-mini — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~usage-based"},
		{ID: "medium", Name: "Standard S0", Description: "Pay-per-token, GPT-4o — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~usage-based"},
		{ID: "large", Name: "Provisioned", Description: "Reserved PTU for GPT-4 — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$2000/month per PTU"},
	}
}

func azureMLPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "Standard_DS2_v2", Description: "2 vCPU, 7 GiB compute cluster — " + name,
			Resources: models.PlanResources{MemoryMB: 7168, CPUMillis: 2000}, CostEstimate: "~$70/month"},
		{ID: "medium", Name: "Standard_DS4_v2", Description: "8 vCPU, 28 GiB compute cluster — " + name,
			Resources: models.PlanResources{MemoryMB: 28672, CPUMillis: 8000}, CostEstimate: "~$280/month"},
		{ID: "large", Name: "Standard_NC6s_v3", Description: "6 vCPU, 112 GiB, 1 GPU — " + name,
			Resources: models.PlanResources{MemoryMB: 114688, CPUMillis: 6000}, CostEstimate: "~$900/month"},
	}
}

func azureMediaPlans(name string) []models.ServicePlan {
	return []models.ServicePlan{
		{ID: "small", Name: "S1 Reserved", Description: "SD encoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.015/min"},
		{ID: "medium", Name: "S2 Reserved", Description: "HD encoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.030/min"},
		{ID: "large", Name: "S3 Reserved", Description: "4K encoding — " + name,
			Resources: models.PlanResources{}, CostEstimate: "~$0.060/min"},
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
