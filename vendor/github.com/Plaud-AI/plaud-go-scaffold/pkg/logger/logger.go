package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"

	"github.com/Plaud-AI/plaud-library-go/env"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel log level
type LogLevel int8

// log levels
const (
	Debug LogLevel = LogLevel(zapcore.DebugLevel)
	Info  LogLevel = LogLevel(zapcore.InfoLevel)
	Warn  LogLevel = LogLevel(zapcore.WarnLevel)
	Error LogLevel = LogLevel(zapcore.ErrorLevel)
)

// Name returns the name of the log level
func (p LogLevel) Name() string {
	switch p {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	}
	return ""
}

// Field represents an extra structured field for logging
type Field struct {
	Name  string
	Value any
}

// LogLevelName LogLevel
func LogLevelName(name string) LogLevel {
	switch name {
	case "debug":
		return Debug
	case "info":
		return Info
	case "warn":
		return Warn
	case "error":
		return Error
	}
	return Info
}

func (p LogLevel) zapLevel() (level zapcore.Level, ok bool) {
	switch p {
	case Debug:
		level = zapcore.DebugLevel
		ok = true
	case Info:
		level = zapcore.InfoLevel
		ok = true
	case Warn:
		level = zapcore.WarnLevel
		ok = true
	case Error:
		level = zapcore.ErrorLevel
		ok = true
	}
	return
}

// Logger 日志记录接口
type Logger interface {
	SetLevel(level LogLevel)
	DebugEnabled() bool
	InfoEnabled() bool
	WarnEnabled() bool
	ErrorEnabled() bool
	Debugf(format string, params ...interface{})
	Infof(format string, params ...interface{})
	Warnf(format string, params ...interface{})
	Errorf(format string, params ...interface{})
	DebugfCtx(ctx context.Context, format string, params ...interface{})
	InfofCtx(ctx context.Context, format string, params ...interface{})
	WarnfCtx(ctx context.Context, format string, params ...interface{})
	ErrorfCtx(ctx context.Context, format string, params ...interface{})
	WithFields(fields []Field) Logger
	WithFieldsCtx(ctx context.Context, fields []Field) context.Context
}

// Debugf debug级别记录日志
func Debugf(format string, params ...interface{}) {
	logger.Debugf(format, params...)
}

func DebugfCtx(ctx context.Context, format string, params ...interface{}) {
	logger.DebugfCtx(ctx, format, params...)
}

// DebugEnabled debug
func DebugEnabled() bool {
	return logger.DebugEnabled()
}

// Infof info级别记录日志
func Infof(format string, params ...interface{}) {
	logger.Infof(format, params...)
}

func InfofCtx(ctx context.Context, format string, params ...interface{}) {
	logger.InfofCtx(ctx, format, params...)
}

// InfoEnabled info
func InfoEnabled() bool {
	return logger.InfoEnabled()
}

// Warnf warn级别记录日志
func Warnf(format string, params ...interface{}) {
	logger.Warnf(format, params...)
}

func WarnfCtx(ctx context.Context, format string, params ...interface{}) {
	logger.WarnfCtx(ctx, format, params...)
}

// WarnEnabled warn
func WarnEnabled() bool {
	return logger.WarnEnabled()
}

// Errorf error级别记录日志
func Errorf(format string, params ...interface{}) {
	logger.Errorf(format, params...)
}

func ErrorfCtx(ctx context.Context, format string, params ...interface{}) {
	logger.ErrorfCtx(ctx, format, params...)
}

// FatalAndExit error级别记录日志并退出程序, 用于严重错误记录日志后退出程序
func FatalAndExit(format string, args ...interface{}) {
	logger.Errorf(format, args...)
	os.Exit(1)
}

// ErrorEnabled error
func ErrorEnabled() bool {
	return logger.ErrorEnabled()
}

// SetLogLevel set the log level
func SetLogLevel(level LogLevel) {
	logger.SetLevel(level)
}

// WithFields returns a Logger enriched with provided fields
func WithFields(fields []Field) Logger {
	// 处理敏感信息
	for i := range fields {
		fields[i].Value = sensitiveLogField(fields[i].Value)
	}
	return logger.WithFields(fields)
}

// Wf (WithFields) returns a Logger enriched with provided fields
func Wf(fields ...Field) Logger {
	// 处理敏感信息
	for i := range fields {
		fields[i].Value = sensitiveLogField(fields[i].Value)
	}
	return logger.WithFields(fields)
}

// WithFieldsCtx returns a context enriched with provided fields
func WithFieldsCtx(ctx context.Context, fields []Field) context.Context {
	// 处理敏感信息
	for i := range fields {
		fields[i].Value = sensitiveLogField(fields[i].Value)
	}
	return logger.WithFieldsCtx(ctx, fields)
}

// WfCtx (WithFieldsCtx) returns a context enriched with provided fields
func WfCtx(ctx context.Context, fields ...Field) context.Context {
	// 处理敏感信息
	for i := range fields {
		fields[i].Value = sensitiveLogField(fields[i].Value)
	}
	return logger.WithFieldsCtx(ctx, fields)
}

// Logf log
func Logf(level LogLevel, foramt string, params ...interface{}) {
	switch level {
	case Debug:
		logger.Debugf(foramt, params...)
	case Info:
		logger.Infof(foramt, params...)
	case Warn:
		logger.Warnf(foramt, params...)
	case Error:
		logger.Errorf(foramt, params...)
	default:
		logger.Debugf(foramt, params...)
	}
}

var (
	logger       Logger = NewStdLogger()
	loggerWriter io.Writer
)

// GetLoggerWriter 获取logger的writer
func GetLoggerWriter() io.Writer {
	return loggerWriter
}

var m sync.Mutex
var loggerInitd bool

// InitLogger 初始化logger
func InitLogger(logConfig *config.LogConfig) (err error) {
	m.Lock()
	defer m.Unlock()

	if loggerInitd {
		Errorf("Logger has been already inited.")
		return
	}

	zapLogger, writeSyncer, err := NewZapLogger(logConfig)
	if err != nil {
		return err
	}
	loggerWriter = writeSyncer
	logger = zapLogger
	loggerInitd = true
	return
}

// ZapLogger 使用zap封装的logger
type ZapLogger struct {
	logEnable zap.AtomicLevel
	logger    *zap.SugaredLogger
	maxLen    int
}

// Debugf debug
func (p *ZapLogger) Debugf(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.DebugLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.logger.Debug(msg)
}

// DebugEnabled is debug enbale
func (p *ZapLogger) DebugEnabled() bool {
	return p.logEnable.Enabled(zap.DebugLevel)
}

// Infof info
func (p *ZapLogger) Infof(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.InfoLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.logger.Info(msg)
}

// InfoEnabled is info enbale
func (p *ZapLogger) InfoEnabled() bool {
	return p.logEnable.Enabled(zap.InfoLevel)
}

// Warnf warn
func (p *ZapLogger) Warnf(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.WarnLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.logger.Warn(msg)
}

// WarnEnabled is info enbale
func (p *ZapLogger) WarnEnabled() bool {
	return p.logEnable.Enabled(zap.WarnLevel)
}

// Errorf error
func (p *ZapLogger) Errorf(format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.ErrorLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.logger.Error(msg)
}

// Context-aware variants that append trace_id/span_id if found
func (p *ZapLogger) DebugfCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.DebugLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.withTrace(ctx).Debug(msg)
}
func (p *ZapLogger) InfofCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.InfoLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.withTrace(ctx).Info(msg)
}
func (p *ZapLogger) WarnfCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.WarnLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.withTrace(ctx).Warn(msg)
}
func (p *ZapLogger) ErrorfCtx(ctx context.Context, format string, params ...interface{}) {
	if !p.logEnable.Enabled(zap.ErrorLevel) {
		return
	}
	msg := TruncateFormatted(format, params, p.maxLen)
	p.withTrace(ctx).Error(msg)
}

// WithFields returns a Logger enriched with provided fields
func (p *ZapLogger) WithFields(fields []Field) Logger {
	if len(fields) == 0 {
		return p
	}
	args := make([]interface{}, 0, len(fields)*2)
	for _, f := range fields {
		args = append(args, f.Name, f.Value)
	}
	newLogger := p.logger.With(args...).WithOptions(zap.AddCallerSkip(-1))
	return &ZapLogger{logger: newLogger, logEnable: p.logEnable, maxLen: p.maxLen}
}

type ctxLogFields string

const (
	ctxLogFieldsKey ctxLogFields = "fields"
)

// WithFieldsCtx 为context添加fields
func (p *ZapLogger) WithFieldsCtx(ctx context.Context, fields []Field) context.Context {
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

func (p *ZapLogger) withTrace(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		return p.logger
	}
	var args []any
	sc := oteltrace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		args = append(args, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}

	if existingFields, ok := ctx.Value(ctxLogFieldsKey).([]Field); ok {
		for _, field := range existingFields {
			args = append(args, field.Name, field.Value)
		}
	}
	if len(args) > 0 {
		return p.logger.With(args...)
	}
	return p.logger
}

// ErrorEnabled is info enbale
func (p *ZapLogger) ErrorEnabled() bool {
	return p.logEnable.Enabled(zap.ErrorLevel)
}

// SetLevel set the log level
func (p *ZapLogger) SetLevel(level LogLevel) {
	zapl, ok := level.zapLevel()
	if ok {
		p.logEnable.SetLevel(zapl)
	}
}

// NewZapLogger new zap logger
func NewZapLogger(logConfig *config.LogConfig) (ret *ZapLogger, writer zapcore.WriteSyncer, err error) {
	var encoder zapcore.Encoder
	var logEnable zap.AtomicLevel

	// default level is info
	if zapl, ok := LogLevelName(logConfig.Level).zapLevel(); ok {
		logEnable = zap.NewAtomicLevelAt(zapl)
	} else {
		logEnable = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// base encoder config by env
	var encCfg zapcore.EncoderConfig
	if logConfig.Env == env.ProductEnv {
		encCfg = zap.NewProductionEncoderConfig()
	} else {
		encCfg = zap.NewDevelopmentEncoderConfig()
	}

	// time encoding override
	if te := strings.ToLower(strings.TrimSpace(logConfig.TimeEncoding)); te != "" {
		if tenc := chooseTimeEncoder(te, logConfig.TimeFormat); tenc != nil {
			encCfg.EncodeTime = tenc
		}
	}

	if encCfg.EncodeTime == nil {
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// encoder format
	if strings.ToLower(strings.TrimSpace(logConfig.Format)) == "json" {
		// set standard encoder keys (no config required)
		encCfg.LevelKey = "level"
		encCfg.TimeKey = "time"
		encCfg.CallerKey = "caller"
		encCfg.MessageKey = "msg"
		encCfg.NameKey = "logger"
		encCfg.StacktraceKey = "stack"
		encoder = zapcore.NewJSONEncoder(encCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}

	var syncers []zapcore.WriteSyncer
	outputFile := logConfig.GetOutputFile()
	if outputFile != "" {
		dir := filepath.Dir(outputFile)
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, nil, err
			}
		}
		pattern := outputFile + ".%Y-%m-%d"
		options := []rotatelogs.Option{
			rotatelogs.WithRotationTime(24 * time.Hour),
		}
		if logConfig.RotateBy == "size" {
			maxSize := int64(logConfig.MaxSize)
			if maxSize < 0 {
				maxSize = 1024 * 1024 * 1024 // 1GB
			}
			options = append(options, rotatelogs.WithRotationSize(maxSize))
		} else {
			options = append(options, rotatelogs.WithRotationTime(24*time.Hour))
		}
		if logConfig.MaxBackups > 0 {
			options = append(options, rotatelogs.WithRotationCount(uint(logConfig.MaxBackups)))
		}
		w, err := rotatelogs.New(pattern, options...)
		if err != nil {
			return nil, nil, err
		}
		syncers = append(syncers, zapcore.AddSync(w))
	}

	//如果环境是开发环境，同时输出到控制台
	if logConfig.Env == env.DevelopEnv {
		writerSync := zapcore.AddSync(os.Stderr)
		syncers = append(syncers, writerSync)
	}

	writerSync := zapcore.NewMultiWriteSyncer(syncers...)

	core := zapcore.NewCore(encoder, writerSync, logEnable)
	logger := zap.New(core)
	if !logConfig.NoCaller {
		logger = logger.WithOptions(zap.AddCaller(), zap.AddCallerSkip(2))
	}

	// decide max message length
	ml := logConfig.MaxMessageLength
	if ml <= 0 {
		ml = 2048
	}

	sugarLogger := logger.Sugar()
	return &ZapLogger{logger: sugarLogger, logEnable: logEnable, maxLen: ml}, writerSync, nil
}

// chooseTimeEncoder returns a zapcore.TimeEncoder based on name/layout.
func chooseTimeEncoder(name string, layout string) zapcore.TimeEncoder {
	switch strings.ToLower(name) {
	case "iso8601":
		return zapcore.ISO8601TimeEncoder
	case "rfc3339":
		return zapcore.RFC3339TimeEncoder
	case "rfc3339nano":
		return zapcore.RFC3339NanoTimeEncoder
	case "millis":
		return zapcore.EpochMillisTimeEncoder
	case "nanos":
		return zapcore.EpochNanosTimeEncoder
	case "epoch", "seconds":
		return zapcore.EpochTimeEncoder
	case "layout":
		if layout != "" {
			return zapcore.TimeEncoderOfLayout(layout)
		}
	}
	return nil
}

// TruncateFormatted formats the message, then truncates to maxLen bytes (safe for UTF-8),
// appending a concise suffix indicating how many bytes were truncated.
func TruncateFormatted(format string, params []interface{}, maxLen int) string {
	msg := fmt.Sprintf(format, params...)
	if maxLen <= 0 || len(msg) <= maxLen {
		return msg
	}
	cut := maxLen
	for cut > 0 && !utf8.RuneStart(msg[cut]) {
		cut--
	}
	truncated := msg[:cut]
	return fmt.Sprintf("%s...(truncated %d bytes)", truncated, len(msg)-cut)
}

// Sensitiver 敏感信息接口
type Sensitiver interface {
	SensitiveLogField() any
}

// sensitiveLogField 敏感信息处理
func sensitiveLogField(v any) any {
	if sv, ok := v.(Sensitiver); ok {
		return sv.SensitiveLogField()
	}
	return v
}
