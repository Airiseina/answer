package tracer

import (
	"github.com/kitex-contrib/obs-opentelemetry/provider"
)

// serviceName: 比如 "api_gateway" 或 "user_service"
// endpoint: OTel Collector 的 OTLP gRPC 地址，如 "otel-collector:4317"
func InitTracer(serviceName string, endpoint string) provider.OtelProvider {
	p := provider.NewOpenTelemetryProvider(
		provider.WithServiceName(serviceName),
		provider.WithExportEndpoint(endpoint),
		provider.WithInsecure(), // 本地开发不使用 TLS
	)

	// kitex-contrib provider 已自动配置了 OTLP Metric 导出器（默认 enableMetrics=true）
	// meter.InitMeter() 通过 otel.GetMeterProvider() 获取的 MeterProvider 即为 provider 创建的
	// 无需再手动创建 MeterProvider，否则会覆盖掉 provider 设置的（含 service.name 等资源属性）

	return p
}
