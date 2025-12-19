package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	_ PublicServerAddressGetter  = (*AppConfig)(nil)
	_ GRPCServerAddressGetter    = (*AppConfig)(nil)
	_ PrivateServerAddressGetter = (*AppConfig)(nil)
	_ LoggerConfigGetter         = (*AppConfig)(nil)
	_ SnowflakeConfigGetter      = (*AppConfig)(nil)
	_ RedisConfigGetter          = (*AppConfig)(nil)
	_ DBConfigGetter             = (*AppConfig)(nil)
	_ JWTRedisConfigGetter       = (*AppConfig)(nil)
	_ ETCDConfigGetter           = (*AppConfig)(nil)
	_ AuthConfigGetter           = (*AppConfig)(nil)
	_ KeySecretsConfigGetter     = (*AppConfig)(nil)
	_ RegionConfigGetter         = (*AppConfig)(nil)
	_ S3ConfigGetter             = (*AppConfig)(nil)
	_ ServerAddrsGetter          = (*AppConfig)(nil)
)

// BaseAppConfig 基础的应用配置
type BaseAppConfig struct {
	//本地的App名称
	AppName string `yaml:"app_name"`
	//AWS AppConfig的App名称
	AWSAppName string `yaml:"aws_app_name"`
	//日志配置
	LogConfig *LogConfig `yaml:"log"`

	envName    string
	configPath string
	loadMode   LoadMode
}

func (p *BaseAppConfig) GetEnv() string {
	return p.envName
}

func (p *BaseAppConfig) SetEnv(envName string) {
	p.envName = envName
}

func (p *BaseAppConfig) GetConfigPath() string {
	return p.configPath
}

func (p *BaseAppConfig) SetConfigPath(configPath string) {
	p.configPath = configPath
}

func (p *BaseAppConfig) GetLoadMode() LoadMode {
	return p.loadMode
}

func (p *BaseAppConfig) SetLoadMode(loadMode LoadMode) {
	p.loadMode = loadMode
}

func (p *BaseAppConfig) GetAppName() string {
	return p.AppName
}

func (p *BaseAppConfig) SetAppName(appName string) {
	p.AppName = appName
}

func (p *BaseAppConfig) GetAwsAppName() string {
	return p.AWSAppName
}

func (p *BaseAppConfig) SetAwsAppName(awsAppName string) {
	p.AWSAppName = awsAppName
}

func (p *BaseAppConfig) GetLoggerConfig() *LogConfig {
	return p.LogConfig
}

// AppConfig 基础的应用配置
type AppConfig struct {
	BaseAppConfig  `yaml:",inline"`
	Public         ServerAddress        `yaml:"public"`
	GRPC           ServerAddress        `yaml:"grpc"`
	Private        ServerAddress        `yaml:"private"`
	DBConfig       *DBConfig            `yaml:"db"`
	RedisConfig    *RedisConfig         `yaml:"redis"`
	JWTRedisConfig *RedisConfig         `yaml:"jwt_redis"`
	ETCDConfig     *ETCDConfig          `yaml:"etcd"`
	AuthConfig     *AuthConfig          `yaml:"auth"`
	Snowflake      *SnowflakeConfig     `yaml:"snowflake"`
	S3             *S3Config            `yaml:"s3"`
	KeySecrets     *KeySecretsConfig    `yaml:"key_secrets"`
	RegionConfig   string               `yaml:"region_config"`
	Observability  *ObservabilityConfig `yaml:"observability"`
}

func (p *AppConfig) GetPublicServerAddress() *ServerAddress {
	return &p.Public
}

func (p *AppConfig) GetGRPCServerAddress() *ServerAddress {
	return &p.GRPC
}

func (p *AppConfig) GetPrivateServerAddress() *ServerAddress {
	return &p.Private
}

func (p *AppConfig) GetSnowflakeConfig() *SnowflakeConfig {
	return p.Snowflake
}

func (p *AppConfig) GetRedisConfig() *RedisConfig {
	return p.RedisConfig
}

func (p *AppConfig) GetDBConfig() *DBConfig {
	return p.DBConfig
}

func (p *AppConfig) GetJWTRedisConfig() *RedisConfig {
	return p.JWTRedisConfig
}

func (p *AppConfig) GetETCDConfig() *ETCDConfig {
	return p.ETCDConfig
}

func (p *AppConfig) GetAuthConfig() *AuthConfig {
	return p.AuthConfig
}

func (p *AppConfig) GetKeySecretsConfig() *KeySecretsConfig {
	return p.KeySecrets
}

func (p *AppConfig) GetRegionConfig() string {
	return p.RegionConfig
}

func (p *AppConfig) GetS3Config() *S3Config {
	return p.S3
}

func parseConfig(c interface{}) error {
	config := reflect.Indirect(reflect.ValueOf(c))
	fieldCount := config.NumField()

	for i := 0; i < fieldCount; i++ {
		val := reflect.Indirect(config.Field(i))
		if !val.IsValid() {
			continue
		}

		addr := val.Addr()
		if !addr.CanInterface() {
			continue
		}
		if configFieldValue, ok := addr.Interface().(Configurer); ok {
			if err := configFieldValue.Parse(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Parse 解析基础的应用配置
func (p *AppConfig) Parse() error {
	return parseConfig(p)
}

// GetConfig 获取配置
func (p *AppConfig) GetConfig() *AppConfig {
	return p
}

// GetServerAddrs 获取服务的地址列表, 多个地址用逗号分隔
func (p *AppConfig) GetServerAddrs() string {
	var addrs []string
	if p.Private.IP != "" && p.Private.IP != "0.0.0.0" && p.Private.Port > 0 {
		addrs = append(addrs, fmt.Sprintf("%s:%d", p.Private.IP, p.Private.Port))
	}
	if p.GRPC.IP != "" && p.GRPC.IP != "0.0.0.0" && p.GRPC.Port > 0 {
		addrs = append(addrs, fmt.Sprintf("%s:%d", p.GRPC.IP, p.GRPC.Port))
	}
	if p.Public.IP != "" && p.Public.IP != "0.0.0.0" && p.Public.Port > 0 {
		addrs = append(addrs, fmt.Sprintf("%s:%d", p.Public.IP, p.Public.Port))
	}
	if len(addrs) == 0 {
		return ""
	}
	return strings.Join(addrs, ",")
}

type ServerAddress struct {
	IP   string `yaml:"ip"`
	Port int    `yaml:"port"`
}

// Assign 设置IP和Port, 如果ip不为空，则设置; 如果port>0，则设置
func (p *ServerAddress) Assign(ip string, port int) {
	if ip != "" {
		p.IP = ip
	}
	if port > 0 {
		p.Port = port
	}
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
	TLS      bool   `yaml:"tls"`
}

// Parse 解析Redis配置
func (p *RedisConfig) Parse() error {
	return nil
}

// S3Config AWS S3配置
type S3Config struct {
	Region   string `yaml:"region"`
	Endpoint string `yaml:"endpoint"`
}

// Parse 解析S3配置
func (p *S3Config) Parse() error {
	if p == nil {
		return nil
	}
	if p.Region == "" {
		p.Region = os.Getenv("AWS_REGION")
	}
	return nil
}

type DBConfig struct {
	// Type: mysql | postgres（必填）
	Type     string `yaml:"type"`
	DSN      string `yaml:"dsn"`
	PoolSize int    `yaml:"pool_size"`
}

func (p *DBConfig) Parse() error {
	if p.Type == "" {
		return fmt.Errorf("db type is empty")
	}
	switch p.Type {
	case "mysql", "postgres":
		// ok
	default:
		return fmt.Errorf("unsupported db type: %s", p.Type)
	}
	if p.DSN == "" {
		return fmt.Errorf("db dsn is empty")
	}
	return nil
}

// SnowflakeConfig 雪花ID配置
type SnowflakeConfig struct {
	// 是否使用ETCD，如果使用ETCD，则NodeID为ETCD中的key，否则为本地配置的NodeID
	UseEtcd bool `yaml:"use_etcd"`
	// 如果使用ETCD，如果不为空则和ETCDConfig中的ServicePrefix拼接，构建指定服务的NodeID的区间Key
	EtcdSuffix string `yaml:"etcd_suffix"`
	// 如果使用ETCD，则NodeID为ETCD中的key，定义NodeID的可以用区间
	EtcdNodIDRange [2]int64 `yaml:"etcd_node_id_range"`
	// 节点ID，范围建议 [0, 1023]
	NodeID int64 `yaml:"node_id"`
	// 自定义纪元，RFC3339 格式（例如 2025-01-01T00:00:00Z）。留空使用 2025-01-01
	Epoch string `yaml:"epoch"`
}

func (p *SnowflakeConfig) Parse() error {
	if p.UseEtcd {
		start := p.EtcdNodIDRange[0]
		end := p.EtcdNodIDRange[1]
		if start < 0 || end < start {
			return fmt.Errorf("invalid etcd_node_id_range: [%d, %d]", start, end)
		}
	} else {
		if p.NodeID < 0 {
			return fmt.Errorf("snowflake node_id must be >= 0")
		}
	}
	if p.Epoch == "" {
		p.Epoch = "2025-01-01T00:00:00Z"
	}
	return nil
}

// ETCDConfig ETCD配置
type ETCDConfig struct {
	Enable        bool     `yaml:"enable"`
	Endpoints     []string `yaml:"endpoints"`
	Username      string   `yaml:"username"`
	Password      string   `yaml:"password"`
	TTL           int64    `yaml:"ttl"`
	ServicePrefix string   `yaml:"service_prefix"`
}

// Parse 解析ETCD配置
func (p *ETCDConfig) Parse() error {
	if len(p.Endpoints) == 0 {
		return fmt.Errorf("etcd endpoints is empty")
	}
	if p.TTL <= 0 {
		p.TTL = 30 // 默认30秒TTL
	}
	return nil
}

// LogConfig 日志配置
type LogConfig struct {
	Env      string `yaml:"env"`
	FileName string `yaml:"file_name"`
	// RotateBy: size | daily (default: daily)
	RotateBy   string `yaml:"rotate_by"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	NoCaller   bool   `yaml:"no_caller"`
	Level      string `yaml:"level"`
	// Format: console | json (default follows existing behavior)
	Format string `yaml:"format"`
	// TimeEncoding: iso8601 | rfc3339 | rfc3339nano | millis | nanos | epoch | layout
	TimeEncoding string `yaml:"time_encoding"`
	// TimeFormat: custom Go time layout used when TimeEncoding == "layout"
	TimeFormat string `yaml:"time_format"`
	// encoder key names are set in code; no config needed
	// MaxMessageLength: truncate log messages longer than this length (in bytes).
	MaxMessageLength int `yaml:"max_message_length"`
}

// Parse 解析日志配置
func (p *LogConfig) Parse() error {
	return nil
}

// GetOutputFile 返回日志的输出文件
func (p *LogConfig) GetOutputFile() string {
	if p == nil {
		return ""
	}

	name := os.Getenv("LOG_FILE")
	if name != "" {
		return name
	}
	return p.FileName
}

// LoadYAMLFromPath 将YAML文件中的配置加载到到结构体target中
func LoadYAMLFromPath(filename string, target interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return LoadYAMl(data, target)
}

// LoadYAMl 将data中的YAML配置加载到到结构体target中
func LoadYAMl(data []byte, target interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("can't load yaml config from empty data")
	}
	return yaml.Unmarshal([]byte(data), target)
}

// SecretKeyID 密钥ID
type SecretKeyID struct {
	AccountEncKeySecretID string `yaml:"account_enc_key_secret_id"`
	AccountEncKey         string `yaml:"account_enc_key"`
}

// KeySecrets holds secret ids used for hashing and encryption
type KeySecrets struct {
	// 是否使用本地配置
	UserLocal bool `yaml:"user_local"`
	// 账号哈希盐的密钥ID
	AccountHashSaltID string `yaml:"account_hash_salt_id"`
	// 账号哈希盐
	AccountHashSalt string `yaml:"account_hash_salt"`
	//当前密钥的版本号
	Version int32 `yaml:"version"`
	//各个版本的密钥, key为版本号, value为密钥ID
	Keys map[int32]SecretKeyID `yaml:"keys"`
}

// GetCurrentKey 获取当前密钥
func (p *KeySecrets) GetCurrentKey() (ret SecretKeyID, ver int32, ok bool) {
	ret, ok = p.Keys[p.Version]
	ver = p.Version
	return
}

// GetKey 获取指定版本的密钥
func (p *KeySecrets) GetKey(ver int32) (ret SecretKeyID, ok bool) {
	ret, ok = p.Keys[ver]
	if !ok {
		return
	}
	return
}

type KeySecretsConfig struct {
	AccountKeys *KeySecrets `yaml:"account_keys"`
}

// ObservabilityConfig 可观测性配置
type ObservabilityConfig struct {
	// 是否启用，默认true
	Enabled bool `yaml:"enabled"`
	// 是否启用控制台导出，默认在开发环境为true
	ConsoleExport bool `yaml:"console_export"`
	// Trace采样率，0.0-1.0，默认1.0
	TraceSamplingRate float64 `yaml:"trace_sampling_rate"`
	// OTLP配置
	OTLP *OTLPConfig `yaml:"otlp"`
	// 是否启用日志增强
	EnhanceLogging bool `yaml:"enhance_logging"`
	// 是否启用性能监控
	EnablePerformanceMonitoring bool `yaml:"enable_performance_monitoring"`
}

// OTLPConfig OTLP导出器配置
type OTLPConfig struct {
	// 是否启用OTLP导出
	Enabled bool `yaml:"enabled"`
	// OTLP端点
	Endpoint string `yaml:"endpoint"`
	// 是否使用非安全连接，默认true（开发环境）
	Insecure bool `yaml:"insecure"`
	// 超时时间（秒），默认30
	Timeout int `yaml:"timeout"`
}

// Parse 解析可观测性配置
func (p *ObservabilityConfig) Parse() error {
	if p == nil {
		return nil
	}

	// 设置默认值
	if p.TraceSamplingRate <= 0 {
		p.TraceSamplingRate = 1.0
	}
	if p.TraceSamplingRate > 1.0 {
		p.TraceSamplingRate = 1.0
	}

	// 解析OTLP配置
	if p.OTLP != nil {
		if p.OTLP.Timeout <= 0 {
			p.OTLP.Timeout = 30
		}
	}

	return nil
}
