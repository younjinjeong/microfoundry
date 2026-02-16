package service

import "fmt"

// ServiceTemplate defines how to deploy a service as K8s resources.
type ServiceTemplate struct {
	Image        string
	Command      []string
	Ports        []int32
	ServicePort  int32
	Env          map[string]string // ${PASSWORD} and ${INSTANCE_NAME} are substituted
	NeedsPVC     bool
	PVCMountPath string
	Probe        ProbeSpec
	BuildOutputs func(host string, port int, password, instanceName string) map[string]string
}

// ProbeSpec defines a readiness probe for a service container.
type ProbeSpec struct {
	Type string // "tcp" or "http"
	Port int32
	Path string // for http probes only
}

// GetTemplate returns the provisioning template for a service type.
func GetTemplate(serviceTypeID string) (ServiceTemplate, bool) {
	t, ok := serviceTemplates[serviceTypeID]
	return t, ok
}

var serviceTemplates = map[string]ServiceTemplate{
	"mariadb": {
		Image:       "mariadb:11",
		Ports:       []int32{3306},
		ServicePort: 3306,
		Env: map[string]string{
			"MARIADB_ROOT_PASSWORD": "${PASSWORD}",
			"MARIADB_DATABASE":      "${INSTANCE_NAME}",
			"MARIADB_USER":          "admin",
			"MARIADB_PASSWORD":      "${PASSWORD}",
		},
		NeedsPVC:     true,
		PVCMountPath: "/var/lib/mysql",
		Probe:        ProbeSpec{Type: "tcp", Port: 3306},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":     host,
				"port":     fmt.Sprintf("%d", port),
				"username": "admin",
				"password": password,
				"database": instanceName,
				"uri":      fmt.Sprintf("mysql://admin:%s@%s:%d/%s", password, host, port, instanceName),
			}
		},
	},
	"postgresql": {
		Image:       "postgres:16-alpine",
		Ports:       []int32{5432},
		ServicePort: 5432,
		Env: map[string]string{
			"POSTGRES_PASSWORD": "${PASSWORD}",
			"POSTGRES_DB":       "${INSTANCE_NAME}",
			"POSTGRES_USER":     "admin",
		},
		NeedsPVC:     true,
		PVCMountPath: "/var/lib/postgresql/data",
		Probe:        ProbeSpec{Type: "tcp", Port: 5432},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":     host,
				"port":     fmt.Sprintf("%d", port),
				"username": "admin",
				"password": password,
				"database": instanceName,
				"uri":      fmt.Sprintf("postgresql://admin:%s@%s:%d/%s", password, host, port, instanceName),
			}
		},
	},
	"clickhouse": {
		Image:       "clickhouse/clickhouse-server:24-alpine",
		Ports:       []int32{8123, 9000},
		ServicePort: 8123,
		Env: map[string]string{
			"CLICKHOUSE_PASSWORD": "${PASSWORD}",
			"CLICKHOUSE_DB":       "${INSTANCE_NAME}",
			"CLICKHOUSE_USER":     "admin",
		},
		NeedsPVC:     true,
		PVCMountPath: "/var/lib/clickhouse",
		Probe:        ProbeSpec{Type: "http", Port: 8123, Path: "/ping"},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":        host,
				"port":        fmt.Sprintf("%d", port),
				"native_port": "9000",
				"username":    "admin",
				"password":    password,
				"database":    instanceName,
				"uri":         fmt.Sprintf("clickhouse://admin:%s@%s:%d/%s", password, host, port, instanceName),
			}
		},
	},
	"redis": {
		Image:       "redis:7-alpine",
		Ports:       []int32{6379},
		ServicePort: 6379,
		Env: map[string]string{},
		Command:     []string{"redis-server", "--requirepass", "${PASSWORD}"},
		NeedsPVC:    false,
		Probe:       ProbeSpec{Type: "tcp", Port: 6379},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":     host,
				"port":     fmt.Sprintf("%d", port),
				"password": password,
				"uri":      fmt.Sprintf("redis://:%s@%s:%d/0", password, host, port),
			}
		},
	},
	"memcached": {
		Image:       "memcached:1-alpine",
		Ports:       []int32{11211},
		ServicePort: 11211,
		Env:         map[string]string{},
		NeedsPVC:    false,
		Probe:       ProbeSpec{Type: "tcp", Port: 11211},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host": host,
				"port": fmt.Sprintf("%d", port),
				"uri":  fmt.Sprintf("%s:%d", host, port),
			}
		},
	},
	"rabbitmq": {
		Image:       "rabbitmq:4-management-alpine",
		Ports:       []int32{5672, 15672},
		ServicePort: 5672,
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "admin",
			"RABBITMQ_DEFAULT_PASS": "${PASSWORD}",
		},
		NeedsPVC: false,
		Probe:    ProbeSpec{Type: "tcp", Port: 5672},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":           host,
				"port":           fmt.Sprintf("%d", port),
				"management_port": "15672",
				"username":       "admin",
				"password":       password,
				"vhost":          "/",
				"uri":            fmt.Sprintf("amqp://admin:%s@%s:%d/", password, host, port),
				"management_uri": fmt.Sprintf("http://%s:15672", host),
			}
		},
	},
	"activemq": {
		Image:       "apache/activemq-artemis:2",
		Ports:       []int32{61616, 8161},
		ServicePort: 61616,
		Env: map[string]string{
			"ARTEMIS_USER":     "admin",
			"ARTEMIS_PASSWORD": "${PASSWORD}",
		},
		NeedsPVC: false,
		Probe:    ProbeSpec{Type: "tcp", Port: 61616},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":           host,
				"port":           fmt.Sprintf("%d", port),
				"console_port":   "8161",
				"username":       "admin",
				"password":       password,
				"uri":            fmt.Sprintf("tcp://admin:%s@%s:%d", password, host, port),
				"console_uri":    fmt.Sprintf("http://%s:8161/console", host),
			}
		},
	},
	"minio": {
		Image:       "minio/minio:latest",
		Command:     []string{"server", "/data", "--console-address", ":9001"},
		Ports:       []int32{9000, 9001},
		ServicePort: 9000,
		Env: map[string]string{
			"MINIO_ROOT_USER":     "admin",
			"MINIO_ROOT_PASSWORD": "${PASSWORD}",
		},
		NeedsPVC:     true,
		PVCMountPath: "/data",
		Probe:        ProbeSpec{Type: "http", Port: 9000, Path: "/minio/health/live"},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":         host,
				"port":         fmt.Sprintf("%d", port),
				"console_port": "9001",
				"access_key":   "admin",
				"secret_key":   password,
				"endpoint":     fmt.Sprintf("http://%s:%d", host, port),
				"console_uri":  fmt.Sprintf("http://%s:9001", host),
				"uri":          fmt.Sprintf("s3://admin:%s@%s:%d", password, host, port),
			}
		},
	},
	"kong": {
		Image:       "kong:3",
		Ports:       []int32{8000, 8001},
		ServicePort: 8000,
		Env: map[string]string{
			"KONG_DATABASE":       "off",
			"KONG_PROXY_LISTEN":   "0.0.0.0:8000",
			"KONG_ADMIN_LISTEN":   "0.0.0.0:8001",
			"KONG_ADMIN_GUI_URL":  "http://localhost:8002",
		},
		NeedsPVC: false,
		Probe:    ProbeSpec{Type: "http", Port: 8001, Path: "/status"},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host":       host,
				"proxy_port": fmt.Sprintf("%d", port),
				"admin_port": "8001",
				"proxy_url":  fmt.Sprintf("http://%s:%d", host, port),
				"admin_url":  fmt.Sprintf("http://%s:8001", host),
			}
		},
	},
	"nginx": {
		Image:       "nginx:1-alpine",
		Ports:       []int32{80},
		ServicePort: 80,
		Env:         map[string]string{},
		NeedsPVC:    false,
		Probe:       ProbeSpec{Type: "http", Port: 80, Path: "/"},
		BuildOutputs: func(host string, port int, password, instanceName string) map[string]string {
			return map[string]string{
				"host": host,
				"port": fmt.Sprintf("%d", port),
				"url":  fmt.Sprintf("http://%s:%d", host, port),
			}
		},
	},
}
