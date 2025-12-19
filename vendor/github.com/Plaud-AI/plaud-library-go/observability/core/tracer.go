package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// TracerConfig Tracer配置接口，避免循环导入
type TracerConfig interface {
	GetTraceSamplingRate() float64
	IsEnableConsoleExport() bool
	IsEnableOTLPExport() bool
	GetOTLPEndpoint() string
}

// InitTracing 初始化OpenTelemetry Tracing
// 功能分层设计：
// 1. 基础功能：生成trace ID和span ID，用于日志记录和请求链路传播
// 2. 打点上报：提供手动打点上报的能力（需要配置导出器）
// 3. 自动收集：自动收集接口的trace链和性能指标（需要配置导出器）
func InitTracing(config TracerConfig, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	// 创建采样器
	sampler := createSampler(config.GetTraceSamplingRate())

	// 创建TracerProvider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// 配置导出器
	processors := []sdktrace.SpanProcessor{}

	// 控制台导出器
	if config.IsEnableConsoleExport() {
		consoleExporter, err := stdouttrace.New(
			stdouttrace.WithWriter(log.Writer()),
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("创建控制台导出器失败: %w", err)
		}
		processors = append(processors, createOptimizedBatchProcessor(consoleExporter))
	}

	// OTLP导出器
	if config.IsEnableOTLPExport() {
		otlpExporter, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(config.GetOTLPEndpoint()),
			otlptracegrpc.WithInsecure(), // 在生产环境中应该配置TLS
		)
		if err != nil {
			return nil, fmt.Errorf("创建OTLP导出器失败: %w", err)
		}
		processors = append(processors, createOptimizedBatchProcessor(otlpExporter))
	}

	// 添加处理器
	for _, processor := range processors {
		tracerProvider.RegisterSpanProcessor(processor)
	}

	// 配置传播器
	setupPropagators()

	// 设置到全局懒加载Provider
	GetGlobalTracerProvider().SetProvider(tracerProvider)

	// 设置全局TracerProvider
	otel.SetTracerProvider(tracerProvider)

	hasExporters := len(processors) > 0
	if hasExporters {
		log.Printf("Tracing 完整功能已启用 - 包含数据导出功能")
	} else {
		log.Printf("Tracing 基础功能已启用 - 仅生成trace ID和span ID用于日志记录和请求传播")
	}

	return tracerProvider, nil
}

// createSampler 创建采样器
func createSampler(rate float64) sdktrace.Sampler {
	// 使用ParentBased采样器，遵循OpenTelemetry最佳实践
	// - 尊重上游决策：如果请求已经被上游服务决定采样，本服务一定会跟进采样
	// - 根节点决策：如果本服务是链路的第一个节点，则根据采样率决定是否采样
	rootSampler := sdktrace.TraceIDRatioBased(rate)
	return sdktrace.ParentBased(rootSampler)
}

// createOptimizedBatchProcessor 创建针对高并发优化的BatchSpanProcessor
// 优化配置以避免CPU和内存问题
func createOptimizedBatchProcessor(exporter sdktrace.SpanExporter) sdktrace.SpanProcessor {
	return sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(256), // 进一步减小队列大小，避免内存积压
		sdktrace.WithBatchTimeout(30*time.Second),  // 更频繁的导出，避免数据积压
		sdktrace.WithMaxExportBatchSize(32),        // 更小的批次，减少单次内存占用
		sdktrace.WithExportTimeout(10*time.Second), // 稍微增加超时，但仍保持合理
	)
}

// setupPropagators 设置传播器
func setupPropagators() {
	// 设置复合传播器，支持多种传播格式
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C Trace Context
		propagation.Baggage{},      // W3C Baggage
		b3.New(),                   // B3 格式（Zipkin）
	))
}

// GetCurrentTraceInfo 获取当前trace信息
func GetCurrentTraceInfo() map[string]interface{} {
	span := trace.SpanFromContext(context.Background())

	// 快速检查：如果是NoOp span，直接返回空值
	if !span.IsRecording() {
		return map[string]interface{}{
			"trace_id":    nil,
			"span_id":     nil,
			"trace_flags": nil,
		}
	}

	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return map[string]interface{}{
			"trace_id":    nil,
			"span_id":     nil,
			"trace_flags": nil,
		}
	}

	return map[string]interface{}{
		"trace_id":    spanContext.TraceID().String(),
		"span_id":     spanContext.SpanID().String(),
		"trace_flags": spanContext.TraceFlags(),
	}
}

// CreateCustomSpan 创建自定义span
func CreateCustomSpan(ctx context.Context, name string, kind trace.SpanKind, attributes map[string]interface{}) (context.Context, trace.Span) {
	tracer := GetTracer("github.com/Plaud-AI/plaud-library-go/observability")

	// 创建span选项
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(kind),
	}

	// 添加属性
	if attributes != nil {
		attrs := make([]attribute.KeyValue, 0, len(attributes))
		for key, value := range attributes {
			switch v := value.(type) {
			case string:
				attrs = append(attrs, attribute.String(key, v))
			case int:
				attrs = append(attrs, attribute.Int(key, v))
			case int64:
				attrs = append(attrs, attribute.Int64(key, v))
			case float64:
				attrs = append(attrs, attribute.Float64(key, v))
			case bool:
				attrs = append(attrs, attribute.Bool(key, v))
			}
		}
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	return tracer.Start(ctx, name, opts...)
}

// GetTraceID 获取当前的trace ID
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return ""
	}

	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ""
	}

	return spanContext.TraceID().String()
}

// GetSpanID 获取当前的span ID
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return ""
	}

	spanContext := span.SpanContext()
	if !spanContext.IsValid() {
		return ""
	}

	return spanContext.SpanID().String()
}

// ShutdownTracing 关闭Tracing
func ShutdownTracing() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return GetGlobalTracerProvider().Shutdown(ctx)
}

// ForceFlushTracing 强制刷新Tracing
func ForceFlushTracing() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return GetGlobalTracerProvider().ForceFlush(ctx)
}
