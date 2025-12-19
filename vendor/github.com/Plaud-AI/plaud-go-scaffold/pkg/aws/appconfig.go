package aws

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/appconfigdata"
)

// AppConfigLoader AWS AppConfig服务
type AppConfigLoader[T config.Configurer] struct {
	awsApplicationName string
	envName            string
	configName         string
	appName            string

	client                     *appconfigdata.Client
	nextPollConfigurationToken *string
	nextPollIntervalInSeconds  int32
	cfgp                       atomic.Pointer[T]

	awsRegion          string
	awsAccessKeyID     string
	awsSecretAccessKey string

	listeners         []ConfigListener[T]
	configVarResolver config.ConfigVarResolver
}

// NewAppConfigLoader 创建新的AppConfig服务
func NewAppConfigLoader[T config.Configurer](awsApplicationName, appName, envName, configName string) *AppConfigLoader[T] {
	service := &AppConfigLoader[T]{
		awsApplicationName:        awsApplicationName,
		appName:                   appName,
		envName:                   envName,
		configName:                configName,
		nextPollIntervalInSeconds: 60,
		awsRegion:                 GetAWSRegion(),
		awsAccessKeyID:            GetAWSAccessKeyID(),
		awsSecretAccessKey:        GetAWSSecretAccessKey(),
	}
	return service
}

// NewAppConfigLoaderWithCredentials 创建新的AppConfig服务（使用显式凭证）
func NewAppConfigLoaderWithCredentials[T config.Configurer](awsApplicationName, appName, envName, configName, region, awsAccessKeyID, awsSecretAccessKey string) *AppConfigLoader[T] {
	service := &AppConfigLoader[T]{
		awsApplicationName:        awsApplicationName,
		appName:                   appName,
		envName:                   envName,
		configName:                configName,
		nextPollIntervalInSeconds: 60,
		awsRegion:                 region,
		awsAccessKeyID:            awsAccessKeyID,
		awsSecretAccessKey:        awsSecretAccessKey,
	}
	return service
}

// Init 初始化AppConfig服务
func (p *AppConfigLoader[T]) Init() (err error) {
    // 优先使用显式提供的 Region；否则让 SDK 通过默认链解析（环境变量/配置文件/IMDS等）
    loadOpts := []func(*awsconfig.LoadOptions) error{}
    if p.awsRegion != "" {
        loadOpts = append(loadOpts, awsconfig.WithRegion(p.awsRegion))
    }
    // 有凭证就用静态凭证；否则如果 Region 也为空，则启用 IMDS 兜底解析
    if p.awsAccessKeyID != "" && p.awsSecretAccessKey != "" {
        loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.awsAccessKeyID, p.awsSecretAccessKey, "")))
    } else if p.awsRegion == "" {
        // 当 AK/SK 缺省且 Region 也为空时，通过 IMDS 解析 Region
        loadOpts = append(loadOpts, awsconfig.WithEC2IMDSRegion())
    }

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), loadOpts...)
	if err != nil {
		logger.Errorf("failed to load AWS config, err:%v", err)
		return
	}

	startConfigurationSessionInput := &appconfigdata.StartConfigurationSessionInput{
		ApplicationIdentifier:          aws.String(p.awsApplicationName),
		EnvironmentIdentifier:          aws.String(p.envName),
		ConfigurationProfileIdentifier: aws.String(p.configName),
	}

	p.client = appconfigdata.NewFromConfig(cfg)
	sessionOutput, err := p.client.StartConfigurationSession(context.TODO(), startConfigurationSessionInput)
	if err != nil {
		logger.Errorf("failed to start configuration session, appconfig applicationName:%s, envName:%s, configName:%s, err:%v", p.awsApplicationName, p.envName, p.configName, err)
		return
	}
	p.nextPollConfigurationToken = sessionOutput.InitialConfigurationToken
	if err = p.readConfig(); err != nil {
		logger.Errorf("failed to read config, err:%v", err)
		return
	}

	return nil
}

func (p *AppConfigLoader[T]) readConfig() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取配置
	configInput := &appconfigdata.GetLatestConfigurationInput{
		ConfigurationToken: p.nextPollConfigurationToken,
	}

	configOutput, err := p.client.GetLatestConfiguration(ctx, configInput)
	if err != nil {
		logger.Errorf("failed to get latest configuration, err:%v", err)
		return err
	}

	configData := configOutput.Configuration
	p.nextPollConfigurationToken = configOutput.NextPollConfigurationToken
	p.nextPollIntervalInSeconds = configOutput.NextPollIntervalInSeconds
	logger.Debugf("%s nextPollIntervalInSeconds:%d", p.configName, p.nextPollIntervalInSeconds)
	if p.nextPollIntervalInSeconds <= 0 {
		p.nextPollIntervalInSeconds = 60
	}

	defer func() {
		go p.periodicLoadConfig(p.nextPollIntervalInSeconds)
	}()

	if len(configData) == 0 {
		if p.cfgp.Load() == nil {
			//第一次加载
			err = fmt.Errorf("config %s is empty %+v", p.configName, configOutput)
		}
		return
	}

	// 解析配置中的变量, 如果解析失败了，则返回错误
	if p.configVarResolver != nil {
		replaced, err := p.configVarResolver.ResolveConfigVars(ctx, string(configData), config.LoadModeAWS, p.appName, p.envName)
		if err != nil {
			logger.Errorf("failed to resolve config vars, err:%v", err)
			return err
		}
		configData = []byte(replaced)
	}

	var newConfig T
	if err = yaml.Unmarshal(configData, &newConfig); err != nil {
		logger.Errorf("failed to unmarshal config, err:%v", err)
		return err
	}
	if err = newConfig.Parse(); err != nil {
		logger.Errorf("failed to parse config, err:%v", err)
		return err
	}

	if envSetter, ok := any(newConfig).(config.EnvSetter); ok {
		envSetter.SetEnv(p.envName)
	}
	if configPathSetter, ok := any(newConfig).(config.ConfigPathSetter); ok {
		configPathSetter.SetConfigPath(p.configName)
	}
	if loadModeSetter, ok := any(newConfig).(config.LoadModeSetter); ok {
		loadModeSetter.SetLoadMode(config.LoadModeAWS)
	}
	if awsAppNameSetter, ok := any(newConfig).(config.AWSAppNameSetter); ok {
		awsAppNameSetter.SetAwsAppName(p.awsApplicationName)
	}

	for _, listener := range p.listeners {
		listener.OnConfigChanged(newConfig)
	}
	p.cfgp.Store(&newConfig)
	return
}

func (p *AppConfigLoader[T]) GetConfig() (ret T) {
	cfg := p.cfgp.Load()
	if cfg == nil {
		return
	}
	ret = *cfg
	return
}

// periodicLoadConfig 定期加载配置
func (p *AppConfigLoader[T]) periodicLoadConfig(intervalSeconds int32) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()
	<-ticker.C
	if err := p.readConfig(); err != nil {
		logger.Errorf("failed to read config, err:%v", err)
	}
}

func (p *AppConfigLoader[T]) GetEnv() string {
	return p.envName
}

func (p *AppConfigLoader[T]) GetConfigPath() string {
	return p.configName
}

func (p *AppConfigLoader[T]) GetLoadMode() config.LoadMode {
	return config.LoadModeAWS
}

func (p *AppConfigLoader[T]) GetAppName() string {
	return p.appName
}

func (p *AppConfigLoader[T]) AddConfigListener(listener ConfigListener[T]) {
	if listener == nil {
		return
	}
	p.listeners = append(p.listeners, listener)
}

// SetConfigVarResolver 设置配置变量解析器
func (p *AppConfigLoader[T]) SetConfigVarResolver(configVarResolver config.ConfigVarResolver) {
	p.configVarResolver = configVarResolver
}

// ConfigListener 配置变更监听器
type ConfigListener[T config.Configurer] interface {
	OnConfigChanged(config T)
}
