package observability

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/Plaud-AI/plaud-library-go/observability/core"
)

// 版本信息
const Version = "1.0.0"

// 全局状态
var (
	initialized  bool
	globalConfig *ObservabilityConfig
	initMutex    sync.Once
)

// InitObservability 初始化可观测性系统
// 这是与本库交互的主要、且向后兼容的入口点
// 此函数可以被安全地多次调用，但只有第一次调用会生效
func InitObservability(serviceName string, options ...InitOption) error {
	var initErr error

	initMutex.Do(func() {
		// 创建默认配置
		globalConfig = NewObservabilityConfig(serviceName)

		// 应用选项
		for _, option := range options {
			option(globalConfig)
		}

		// 验证配置
		if err := globalConfig.ValidateConfig(); err != nil {
			initErr = fmt.Errorf("配置验证失败: %w", err)
			return
		}

		// 创建资源
		res, err := createResource(globalConfig)
		if err != nil {
			initErr = fmt.Errorf("创建资源失败: %w", err)
			return
		}

		// 初始化Tracing
		if _, err := core.InitTracing(globalConfig, res); err != nil {
			initErr = fmt.Errorf("初始化Tracing失败: %w", err)
			return
		}

		// 初始化Metrics
		if _, err := core.InitMetrics(globalConfig, res); err != nil {
			initErr = fmt.Errorf("初始化Metrics失败: %w", err)
			return
		}

		// 初始化日志增强
		core.InitLogging(globalConfig)

		// 启用性能监控（如果配置了）
		if globalConfig.EnablePerformanceMonitoring {
			core.GetPerformanceMonitor().Start()
		}

		// 标记已初始化
		initialized = true

		log.Printf("可观测性系统已初始化 - 服务: %s, 版本: %s", globalConfig.ServiceName, globalConfig.ServiceVersion)
	})

	if initErr != nil {
		return initErr
	}

	if initialized {
		log.Printf("可观测性系统已经初始化，跳过重复操作")
	}

	return nil
}

// InitOption 初始化选项
type InitOption func(*ObservabilityConfig)

// WithServiceVersion 设置服务版本
func WithServiceVersion(version string) InitOption {
	return func(c *ObservabilityConfig) {
		c.ServiceVersion = version
	}
}

// WithEnvironment 设置环境
func WithEnvironment(env string) InitOption {
	return func(c *ObservabilityConfig) {
		c.Environment = env
	}
}

// WithConsoleExport 启用控制台导出
func WithConsoleExport(enabled bool) InitOption {
	return func(c *ObservabilityConfig) {
		c.EnableConsoleExport = enabled
	}
}

// WithOTLPEndpoint 设置OTLP端点
func WithOTLPEndpoint(endpoint string) InitOption {
	return func(c *ObservabilityConfig) {
		c.OTLPEndpoint = &endpoint
	}
}

// WithTraceSamplingRate 设置trace采样率
func WithTraceSamplingRate(rate float64) InitOption {
	return func(c *ObservabilityConfig) {
		c.TraceSamplingRate = rate
	}
}

// WithMetricsExportInterval 设置metrics导出间隔
func WithMetricsExportInterval(interval time.Duration) InitOption {
	return func(c *ObservabilityConfig) {
		c.MetricsExportInterval = interval
	}
}

// WithAutoInstrumentHTTPClient 设置HTTP客户端自动插桩
func WithAutoInstrumentHTTPClient(enabled bool) InitOption {
	return func(c *ObservabilityConfig) {
		c.AutoInstrumentHTTPClient = enabled
	}
}

// WithAutoInstrumentKafka 设置Kafka自动插桩
func WithAutoInstrumentKafka(enabled bool) InitOption {
	return func(c *ObservabilityConfig) {
		c.AutoInstrumentKafka = enabled
	}
}

// WithEnhanceLogging 设置日志增强
func WithEnhanceLogging(enabled bool) InitOption {
	return func(c *ObservabilityConfig) {
		c.EnhanceLogging = enabled
	}
}

// WithCustomAttributes 设置自定义属性
func WithCustomAttributes(attrs map[string]string) InitOption {
	return func(c *ObservabilityConfig) {
		if c.CustomAttributes == nil {
			c.CustomAttributes = make(map[string]string)
		}
		for k, v := range attrs {
			c.CustomAttributes[k] = v
		}
	}
}

// WithEnablePerformanceMonitoring 设置性能监控
func WithEnablePerformanceMonitoring(enabled bool) InitOption {
	return func(c *ObservabilityConfig) {
		c.EnablePerformanceMonitoring = enabled
	}
}

// WithConfig 使用完整的配置对象
func WithConfig(cfg *ObservabilityConfig) InitOption {
	return func(c *ObservabilityConfig) {
		*c = *cfg
	}
}

// createResource 创建OpenTelemetry资源
func createResource(config *ObservabilityConfig) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(config.ServiceVersion),
		semconv.DeploymentEnvironment(config.Environment),
		semconv.ServiceInstanceID(config.InstanceID),
	}

	// 添加自定义属性
	if len(config.CustomAttributes) > 0 {
		for key, value := range config.CustomAttributes {
			attrs = append(attrs, attribute.String(key, value))
		}
	}

	// 直接创建资源，避免Schema URL冲突
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attrs...,
	), nil
}

// ShutdownObservability 优雅关闭可观测性系统
func ShutdownObservability() error {
	if !initialized {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止性能监控
	if globalConfig != nil && globalConfig.EnablePerformanceMonitoring {
		core.GetPerformanceMonitor().Stop()
	}

	// 关闭Tracing
	if err := core.GetGlobalTracerProvider().Shutdown(ctx); err != nil {
		log.Printf("关闭Tracing失败: %v", err)
	}

	// 关闭Metrics
	if err := core.GetGlobalMeterProvider().Shutdown(ctx); err != nil {
		log.Printf("关闭Metrics失败: %v", err)
	}

	log.Println("可观测性系统已关闭")
	return nil
}

// ForceFlushObservability 强制刷新可观测性系统
func ForceFlushObservability() error {
	if !initialized {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 刷新Tracing
	if err := core.GetGlobalTracerProvider().ForceFlush(ctx); err != nil {
		log.Printf("刷新Tracing失败: %v", err)
	}

	// 刷新Metrics
	if err := core.GetGlobalMeterProvider().ForceFlush(ctx); err != nil {
		log.Printf("刷新Metrics失败: %v", err)
	}

	return nil
}

// IsInitialized 检查是否已初始化
func IsInitialized() bool {
	return initialized
}

// GetConfig 获取当前配置
func GetConfig() *ObservabilityConfig {
	return globalConfig
}

// GetVersion 获取版本信息
func GetVersion() string {
	return Version
}
