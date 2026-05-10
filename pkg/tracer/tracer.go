package tracer

import (
	"github.com/kitex-contrib/obs-opentelemetry/provider"
)

// serviceName: 比如 "api_gateway" 或 "user_service"
// endpoint: jaeger 容器的 OTLP 地址，因为同在一个 Network 里，可用 "jaeger:4317"
func InitTracer(serviceName string, endpoint string) provider.OtelProvider {
	p := provider.NewOpenTelemetryProvider(
		provider.WithServiceName(serviceName),
		provider.WithExportEndpoint(endpoint),
		provider.WithInsecure(), // 本地开发不使用 TLS
	)
	return p
}
