package observability

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// 全局实例ID，模块加载时自动生成
var GlobalInstanceID = generateInstanceID()

// ObservabilityConfig 可观测性配置结构
// 包含所有可观测性功能的配置选项，支持通过代码或环境变量进行配置
type ObservabilityConfig struct {
	// ========== 基础服务配置 ==========

	// ServiceName 服务名称，用于标识你的应用服务
	// 例如: "plaud-api", "plaud-gateway", "user-service"
	// 这个名称会出现在 tracing 和 metrics 中，方便区分不同服务
	ServiceName string `json:"service_name"`

	// ServiceVersion 服务版本号，用于版本追踪和问题定位
	// 例如: "1.0.0", "v2.1.3", "latest"
	// 当你部署新版本时，可以通过版本号快速定位问题
	ServiceVersion string `json:"service_version"`

	// Environment 运行环境，用于区分不同部署环境
	// 例如: "development", "staging", "production"
	// 帮助你在监控面板中区分不同环境的数据
	Environment string `json:"environment"`

	// ========== Trace 采样配置 ==========

	// TraceSamplingRate Trace 采样率
	// 一个介于 0.0 和 1.0 之间的浮点数
	// 1.0: 采集所有 trace (适合开发环境)
	// 0.1: 采集 10% 的 trace
	// 0.0: 关闭 trace 采集
	TraceSamplingRate float64 `json:"trace_sampling_rate"`

	// ========== 数据导出配置 ==========

	// EnableConsoleExport 是否启用控制台导出
	// true: 将 traces 和 metrics 输出到控制台，适合开发调试
	// false: 不输出到控制台，适合生产环境
	EnableConsoleExport bool `json:"enable_console_export"`

	// OTLPEndpoint OTLP 接收端点地址
	// nil: 禁用所有远程功能，调用打点时会显示警告
	// "http://otel-collector:4317": 启用完整可观测性功能
	// 这是控制整个系统行为的关键配置
	OTLPEndpoint *string `json:"otlp_endpoint"`

	// ========== 自动插桩配置 ==========

	// AutoInstrumentHTTPClient 是否自动为 HTTP 客户端请求添加 tracing
	// true: 自动记录所有出站 HTTP 请求的耗时、状态码、URL 等信息
	// false: 需要手动添加 HTTP tracing 代码
	// 推荐开启，可以自动追踪对下游服务的调用
	AutoInstrumentHTTPClient bool `json:"auto_instrument_http_client"`

	// AutoInstrumentKafka 是否自动为 Kafka 消息添加 tracing
	// true: 自动记录消息生产和消费的链路信息
	// false: Kafka 操作不会被自动追踪
	// 如果使用 Kafka，推荐开启
	AutoInstrumentKafka bool `json:"auto_instrument_kafka"`

	// ========== 日志增强配置 ==========

	// EnhanceLogging 是否启用日志增强功能
	// true: 自动在日志中注入 trace_id 和 span_id，方便关联日志和链路
	// false: 日志保持原样，不添加 tracing 信息
	// 强烈推荐开启，可以快速定位问题
	EnhanceLogging bool `json:"enhance_logging"`

	// ========== Metrics 配置 ==========

	// MetricsExportInterval Metrics 导出间隔
	// 控制多久向后端发送一次 metrics 数据
	// 例如: 30s, 60s, 5m
	// 间隔越短数据越实时，但网络开销越大
	MetricsExportInterval time.Duration `json:"metrics_export_interval"`

	// ========== 自定义属性 ==========

	// CustomAttributes 自定义属性，会添加到所有 trace 和 metric 中
	// 例如: map[string]string{"team": "backend", "owner": "alice"}
	// 这些属性可以用于过滤和分组监控数据
	CustomAttributes map[string]string `json:"custom_attributes"`

	// ========== 高级配置 ==========

	// InstanceID 实例ID，用于区分同一服务的不同实例
	// 默认使用 hostname-pid-random 格式自动生成
	InstanceID string `json:"instance_id"`

	// EnablePerformanceMonitoring 是否启用性能监控
	// 监控goroutine泄漏、内存使用、GC压力等指标
	// 建议在开发和测试环境中启用，生产环境根据需要启用
	EnablePerformanceMonitoring bool `json:"enable_performance_monitoring"`
}

// NewObservabilityConfig 创建默认配置
func NewObservabilityConfig(serviceName string) *ObservabilityConfig {
	return &ObservabilityConfig{
		ServiceName:              serviceName,
		ServiceVersion:           "1.0.0",
		Environment:              "production",
		TraceSamplingRate:        0.0,
		EnableConsoleExport:      false,
		OTLPEndpoint:             nil,
		AutoInstrumentHTTPClient: true,
		AutoInstrumentKafka:      true,
		EnhanceLogging:           true,
		MetricsExportInterval:    30 * time.Second,
		CustomAttributes:         make(map[string]string),
		InstanceID:               GlobalInstanceID,
	}
}

// EnableOTLPExport 启用OTLP导出
func (c *ObservabilityConfig) EnableOTLPExport() bool {
	return c.OTLPEndpoint != nil && *c.OTLPEndpoint != ""
}

// ValidateConfig 验证配置
func (c *ObservabilityConfig) ValidateConfig() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service_name 不能为空")
	}

	if c.TraceSamplingRate < 0.0 || c.TraceSamplingRate > 1.0 {
		return fmt.Errorf("trace_sampling_rate 必须在 0.0 到 1.0 之间")
	}

	if c.MetricsExportInterval < time.Second {
		return fmt.Errorf("metrics_export_interval 不能小于 1 秒")
	}

	return nil
}

// generateInstanceID 生成实例ID
func generateInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	pid := os.Getpid()
	randomSuffix := uuid.New().String()[:8]

	return fmt.Sprintf("%s-%d-%s", hostname, pid, randomSuffix)
}

// IsEnhanceLogging 实现LoggingConfig接口
func (c *ObservabilityConfig) IsEnhanceLogging() bool {
	return c.EnhanceLogging
}

// GetInstanceID 实现LoggingConfig接口
func (c *ObservabilityConfig) GetInstanceID() string {
	return c.InstanceID
}

// IsEnableConsoleExport 实现MetricsConfig接口
func (c *ObservabilityConfig) IsEnableConsoleExport() bool {
	return c.EnableConsoleExport
}

// IsEnableOTLPExport 实现MetricsConfig接口
func (c *ObservabilityConfig) IsEnableOTLPExport() bool {
	return c.EnableOTLPExport()
}

// GetOTLPEndpoint 实现MetricsConfig接口
func (c *ObservabilityConfig) GetOTLPEndpoint() string {
	if c.OTLPEndpoint != nil {
		return *c.OTLPEndpoint
	}
	return ""
}

// GetMetricsExportInterval 实现MetricsConfig接口
func (c *ObservabilityConfig) GetMetricsExportInterval() time.Duration {
	return c.MetricsExportInterval
}

// GetTraceSamplingRate 实现TracerConfig接口
func (c *ObservabilityConfig) GetTraceSamplingRate() float64 {
	return c.TraceSamplingRate
}
