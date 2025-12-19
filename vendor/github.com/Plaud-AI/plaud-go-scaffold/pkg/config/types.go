package config

import "context"

// ConfigConstraint 配置约束
type ConfigConstraint interface {
	Configurer
	comparable
	EnvSetter
	ConfigPathSetter
	LoadModeGetter
	AppNameGetter
	AWSAppNameGetter
	LoadModeSetter
	AppNameSetter
	AWSAppNameSetter
	ServerAddrsGetter
	LoggerConfigGetter
}

type BaseConfigConstraint interface {
	SnowflakeConfigGetter
	RedisConfigGetter
	DBConfigGetter
	JWTRedisConfigGetter
	ETCDConfigGetter
	AuthConfigGetter
	KeySecretsConfigGetter
	RegionConfigGetter
	S3ConfigGetter
}

// Configurer 配置器
type Configurer interface {
	//解析配置
	Parse() error
}

// 配置加载的模式
type LoadMode string

const (
	//从本地加载
	LoadModeLocal LoadMode = "local"
	//从AWS AppConfig加载
	LoadModeAWS LoadMode = "aws"
)

// AppConfigGetter 获取AppConfig
type AppConfigGetter[T ConfigConstraint] interface {
	GetConfig() T
	GetEnv() string
	GetConfigPath() string
	GetLoadMode() LoadMode
	GetAppName() string
}

// EnvSetter 设置环境
type EnvSetter interface {
	GetEnv() string
	SetEnv(envName string)
}

// ConfigPathSetter 设置配置路径
type ConfigPathSetter interface {
	GetConfigPath() string
	SetConfigPath(configPath string)
}

type LoadModeGetter interface {
	GetLoadMode() LoadMode
}

// LoadModeSetter 设置加载模式
type LoadModeSetter interface {
	SetLoadMode(loadMode LoadMode)
}

type AppNameGetter interface {
	GetAppName() string
}

type AppNameSetter interface {
	SetAppName(appName string)
}

type AWSAppNameGetter interface {
	GetAwsAppName() string
}

type AWSAppNameSetter interface {
	SetAwsAppName(awsAppName string)
}

type PublicServerAddressGetter interface {
	GetPublicServerAddress() *ServerAddress
}

type GRPCServerAddressGetter interface {
	GetGRPCServerAddress() *ServerAddress
}

type PrivateServerAddressGetter interface {
	GetPrivateServerAddress() *ServerAddress
}

type LoggerConfigGetter interface {
	GetLoggerConfig() *LogConfig
}

type SnowflakeConfigGetter interface {
	GetSnowflakeConfig() *SnowflakeConfig
}

type RedisConfigGetter interface {
	GetRedisConfig() *RedisConfig
}

type DBConfigGetter interface {
	GetDBConfig() *DBConfig
}

type JWTRedisConfigGetter interface {
	GetJWTRedisConfig() *RedisConfig
}

type ETCDConfigGetter interface {
	GetETCDConfig() *ETCDConfig
}

type AuthConfigGetter interface {
	GetAuthConfig() *AuthConfig
}

type KeySecretsConfigGetter interface {
	GetKeySecretsConfig() *KeySecretsConfig
}

type RegionConfigGetter interface {
	GetRegionConfig() string
}

type S3ConfigGetter interface {
	GetS3Config() *S3Config
}

type ServerAddrsGetter interface {
	GetServerAddrs() string
}

// ConfigVarResolver 配置变量解析器
type ConfigVarResolver interface {
	ResolveConfigVars(ctx context.Context, config string, mode LoadMode, appName string, envName string) (replaced string, err error)
}
