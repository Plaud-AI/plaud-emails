package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

// LoggingConfig 日志配置接口，避免循环导入
type LoggingConfig interface {
	IsEnhanceLogging() bool
	GetInstanceID() string
}

// TraceHook logrus Hook，用于注入trace信息
type TraceHook struct{}

// Levels 返回该hook需要处理的日志级别
func (h *TraceHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 在日志记录时触发，注入trace信息
func (h *TraceHook) Fire(entry *logrus.Entry) error {
	// 从context中获取trace信息
	if ctx := entry.Context; ctx != nil {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			spanContext := span.SpanContext()
			if spanContext.IsValid() {
				entry.Data["trace_id"] = spanContext.TraceID().String()
				entry.Data["span_id"] = spanContext.SpanID().String()
			}
		}
	}

	// 添加实例ID
	entry.Data["instance_id"] = "go-observability-instance"

	return nil
}

// LogrusFormatter 自定义格式化器，包含trace信息
type LogrusFormatter struct {
	*logrus.TextFormatter
}

// Format 格式化日志输出
func (f *LogrusFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 获取trace信息
	traceID := ""
	spanID := ""

	if tid, ok := entry.Data["trace_id"]; ok {
		traceID = tid.(string)
	}
	if sid, ok := entry.Data["span_id"]; ok {
		spanID = sid.(string)
	}

	// 构建日志前缀
	prefix := ""
	if traceID != "" && spanID != "" {
		prefix = fmt.Sprintf("[trace_id=%s span_id=%s] ", traceID, spanID)
	}

	// 使用原始格式化器
	data, err := f.TextFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	// 添加trace信息前缀
	if prefix != "" {
		prefixedData := make([]byte, len(prefix)+len(data))
		copy(prefixedData, prefix)
		copy(prefixedData[len(prefix):], data)
		return prefixedData, nil
	}

	return data, nil
}

// InitLogging 初始化日志增强功能
func InitLogging(config LoggingConfig) {
	if !config.IsEnhanceLogging() {
		log.Println("日志增强功能已禁用")
		return
	}

	// 设置logrus
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	// 设置格式化器
	logrus.SetFormatter(&LogrusFormatter{
		TextFormatter: &logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		},
	})

	// 添加trace hook
	logrus.AddHook(&TraceHook{})

	// 替换标准log
	log.SetOutput(logrus.StandardLogger().Writer())

	log.Println("日志增强功能已启用 - 自动注入trace_id和span_id")
}

// ContextWithTraceInfo 为context添加trace信息
func ContextWithTraceInfo(ctx context.Context) context.Context {
	// 如果context已经有trace信息，直接返回
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		return ctx
	}

	// 创建一个新的span来携带trace信息
	tracer := GetTracer("github.com/Plaud-AI/plaud-library-go/observability/logging")
	ctx, span := tracer.Start(ctx, "logging_context")

	// 立即结束span，我们只需要trace信息
	defer span.End()

	return ctx
}

// LogWithTrace 带trace信息的日志记录
func LogWithTrace(ctx context.Context, level logrus.Level, message string, fields logrus.Fields) {
	entry := logrus.WithContext(ctx)

	// 添加额外字段
	if fields != nil {
		entry = entry.WithFields(fields)
	}

	entry.Log(level, message)
}

// InfoWithTrace 带trace信息的Info日志
func InfoWithTrace(ctx context.Context, message string, fields logrus.Fields) {
	LogWithTrace(ctx, logrus.InfoLevel, message, fields)
}

// ErrorWithTrace 带trace信息的Error日志
func ErrorWithTrace(ctx context.Context, message string, fields logrus.Fields) {
	LogWithTrace(ctx, logrus.ErrorLevel, message, fields)
}

// WarnWithTrace 带trace信息的Warn日志
func WarnWithTrace(ctx context.Context, message string, fields logrus.Fields) {
	LogWithTrace(ctx, logrus.WarnLevel, message, fields)
}

// DebugWithTrace 带trace信息的Debug日志
func DebugWithTrace(ctx context.Context, message string, fields logrus.Fields) {
	LogWithTrace(ctx, logrus.DebugLevel, message, fields)
}
