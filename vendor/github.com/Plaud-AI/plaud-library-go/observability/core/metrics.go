package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// MetricsConfig Metrics配置接口，避免循环导入
type MetricsConfig interface {
	IsEnableConsoleExport() bool
	IsEnableOTLPExport() bool
	GetOTLPEndpoint() string
	GetMetricsExportInterval() time.Duration
}

// InitMetrics 初始化OpenTelemetry Metrics
// 功能分层设计：
// 1. 基础功能：提供服务标识和基础API（不收集数据）
// 2. 数据收集：收集metrics数据并导出到监控系统（需要配置导出器）
func InitMetrics(config MetricsConfig, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	// 创建Readers
	readers := []sdkmetric.Reader{}

	// 控制台导出器
	if config.IsEnableConsoleExport() {
		consoleExporter, err := stdoutmetric.New(
			stdoutmetric.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("创建Metrics控制台导出器失败: %w", err)
		}

		reader := sdkmetric.NewPeriodicReader(
			consoleExporter,
			sdkmetric.WithInterval(config.GetMetricsExportInterval()),
		)
		readers = append(readers, reader)
	}

	// OTLP导出器
	if config.IsEnableOTLPExport() {
		otlpExporter, err := otlpmetricgrpc.New(
			context.Background(),
			otlpmetricgrpc.WithEndpoint(config.GetOTLPEndpoint()),
			otlpmetricgrpc.WithInsecure(), // 在生产环境中应该配置TLS
		)
		if err != nil {
			return nil, fmt.Errorf("创建OTLP Metrics导出器失败: %w", err)
		}

		reader := sdkmetric.NewPeriodicReader(
			otlpExporter,
			sdkmetric.WithInterval(config.GetMetricsExportInterval()),
		)
		readers = append(readers, reader)
	}

	// 创建MeterProvider选项
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	// 添加所有readers
	for _, reader := range readers {
		opts = append(opts, sdkmetric.WithReader(reader))
	}

	meterProvider := sdkmetric.NewMeterProvider(opts...)

	// 设置到全局懒加载Provider
	GetGlobalMeterProvider().SetProvider(meterProvider)

	// 设置全局MeterProvider
	otel.SetMeterProvider(meterProvider)

	hasExporters := len(readers) > 0
	if hasExporters {
		log.Printf("Metrics 完整功能已启用 - 包含数据推送导出功能")
	} else {
		log.Printf("Metrics 基础功能已启用 - 仅在内存中生成指标，等待外部拉取 (如 Prometheus)")
	}

	return meterProvider, nil
}

// ShutdownMetrics 关闭Metrics
func ShutdownMetrics() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return GetGlobalMeterProvider().Shutdown(ctx)
}

// ForceFlushMetrics 强制刷新Metrics
func ForceFlushMetrics() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return GetGlobalMeterProvider().ForceFlush(ctx)
}
