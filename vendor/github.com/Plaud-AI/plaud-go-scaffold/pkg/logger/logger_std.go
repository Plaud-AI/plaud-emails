package logger

import (
	"context"
	"fmt"
	"log"

	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// StdLogger 使用标准库封装的logger
type StdLogger struct {
	logEnable zap.AtomicLevel
	fields    []Field
}

// NewStdLogger new info level logger
func NewStdLogger() *StdLogger {
	logger := &StdLogger{
		logEnable: zap.NewAtomicLevelAt(zapcore.InfoLevel),
	}
	return logger
}

func (p *StdLogger) fieldsPrefix() string {
	if len(p.fields) == 0 {
		return ""
	}
	buf := "["
	for i, f := range p.fields {
		if i > 0 {
			buf += " "
		}
		buf += f.Name + "="
		buf += toStringAny(f.Value)
	}
	buf += "] "
	return buf
}

// toStringAny provides a basic string conversion for field values
func toStringAny(v any) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", v)
}

// Debugf debug
func (p *StdLogger) Debugf(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.DebugLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + TruncateFormatted(format, params, 0))
}

// DebugEnabled is debug enbale
func (p *StdLogger) DebugEnabled() bool {
	return p.logEnable.Enabled(zap.DebugLevel)
}

// Infof info
func (p *StdLogger) Infof(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.InfoLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + TruncateFormatted(format, params, 0))
}

// InfoEnabled is info enable
func (p *StdLogger) InfoEnabled() bool {
	return p.logEnable.Enabled(zap.InfoLevel)
}

// Warnf warn
func (p *StdLogger) Warnf(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.WarnLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + TruncateFormatted(format, params, 0))
}

// WarnEnabled is  warn enabled
func (p *StdLogger) WarnEnabled() bool {
	return p.logEnable.Enabled(zap.WarnLevel)
}

// Errorf error
func (p *StdLogger) Errorf(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.ErrorLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + TruncateFormatted(format, params, 0))
}

// ErrorEnabled error
func (p *StdLogger) ErrorEnabled() bool {
	return p.logEnable.Enabled(zap.ErrorLevel)
}

// SetLevel set the log level
func (p *StdLogger) SetLevel(level LogLevel) {
	zapl, ok := level.zapLevel()
	if ok {
		p.logEnable.SetLevel(zapl)
	}
}

// WithFields returns a new StdLogger with additional fields
func (p *StdLogger) WithFields(fields []Field) Logger {
	if len(fields) == 0 {
		return p
	}
	merged := make([]Field, 0, len(p.fields)+len(fields))
	merged = append(merged, p.fields...)
	merged = append(merged, fields...)
	return &StdLogger{logEnable: p.logEnable, fields: merged}
}

func (p *StdLogger) DebugfCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.DebugLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + withTracePrefix(ctx, TruncateFormatted(format, params, 0)))
}

func (p *StdLogger) InfofCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.InfoLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + withTracePrefix(ctx, TruncateFormatted(format, params, 0)))
}

func (p *StdLogger) WarnfCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.WarnLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + withTracePrefix(ctx, TruncateFormatted(format, params, 0)))
}

func (p *StdLogger) ErrorfCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.ErrorLevel) {
		return
	}
	log.Print(p.fieldsPrefix() + withTracePrefix(ctx, TruncateFormatted(format, params, 0)))
}

// WithFieldsCtx 为context添加fields（与 ZapLogger 行为一致）
func (p *StdLogger) WithFieldsCtx(ctx context.Context, fields []Field) context.Context {
	if len(fields) == 0 {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if existingFields, ok := ctx.Value(ctxLogFieldsKey).([]Field); ok && len(existingFields) > 0 {
		merged := make([]Field, 0, len(existingFields)+len(fields))
		merged = append(merged, existingFields...)
		merged = append(merged, fields...)
		return context.WithValue(ctx, ctxLogFieldsKey, merged)
	}
	copied := append([]Field(nil), fields...)
	return context.WithValue(ctx, ctxLogFieldsKey, copied)
}

func withTracePrefix(ctx context.Context, format string) string {
	if ctx == nil {
		return format
	}
	var parts string
	var has bool
	sc := oteltrace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		parts = "trace_id=" + sc.TraceID().String() + ", span_id=" + sc.SpanID().String()
		has = true
	}
	if existingFields, ok := ctx.Value(ctxLogFieldsKey).([]Field); ok && len(existingFields) > 0 {
		for _, f := range existingFields {
			if has {
				parts += " "
			}
			parts += f.Name + "=" + toStringAny(f.Value)
			has = true
		}
	}
	if has {
		return "[" + parts + "] " + format
	}
	return format
}
