package vars

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/aws"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"gopkg.in/yaml.v3"
)

// ConfigVarResolver 配置变量解析器
type ConfigVarResolver struct {
	// 用于指定加载secrets的key
	secretsName string
}

// NewConfigVarResolver 创建新的配置变量解析器
func NewConfigVarResolver() *ConfigVarResolver {
	return &ConfigVarResolver{}
}

// NewConfigVarResolverWithName 创建新的配置变量解析器，并指定加载secrets的key
func NewConfigVarResolverWithName(secretsName string) *ConfigVarResolver {
	return &ConfigVarResolver{
		secretsName: secretsName,
	}
}

// ResolveConfigVars 对文本中的占位符进行替换并返回替换后的文本。
// 占位符规则：
//   - ${env:KEY}: 使用 os.Getenv(KEY)
//   - ${ref:SECRET_ID}: 使用 AWS Secrets Manager 中 SECRET_ID 对应的 SecretString
//   - 其它 ${KEY}:
//   - local 模式: 使用 os.Getenv(KEY)
//   - aws 模式: 先从 SecretsManager 加载的顶层键查找，再回退到环境变量
//
// secres的加载规则:
//   - 如果环境变量APP_SECRETS_NAME 不为空，则使用 APP_SECRETS_NAME 加载 secrets
//   - 如果环境变量APP_SECRETS_NAME 为空，则使用 "<env>/<appName>-secrets" 加载 secrets
func (p ConfigVarResolver) ResolveConfigVars(ctx context.Context, text string, mode config.LoadMode, appName string, envName string) (replaced string, err error) {
	var (
		valueGetter       valueGetterFunc
		secretsMap        map[string]string
		getVarFromSecrets = func(key string) (val string, err error) {
			if secretsMap == nil {
				secretsMap, err = p.loadSecretsMap(ctx, appName, envName)
				if err != nil {
					logger.Errorf("load secrets map failed: %v", err)
					return
				}
			}
			if v, ok := secretsMap[key]; ok {
				return v, nil
			}
			return "", fmt.Errorf("key %s not found in secrets map", key)
		}
	)

	switch mode {
	case config.LoadModeLocal:
		valueGetter = func(key string) (val string, err error) {
			if isRef(key) {
				refID, err := parseRef(key)
				if err != nil {
					logger.Errorf("parse ref %s failed: %v", key, err)
					return "", err
				}
				return getVarFromSecrets(refID)
			}
			return parseEnvVal(key)
		}
	case config.LoadModeAWS:
		valueGetter = func(key string) (val string, err error) {
			if isEnv(key) {
				return parseEnvVal(key)
			}
			if isRef(key) {
				refID, err := parseRef(key)
				if err != nil {
					logger.Errorf("parse ref %s failed: %v", key, err)
					return "", err
				}
				key = refID
			}
			return getVarFromSecrets(key)
		}
	default:
		return text, nil
	}

	// 使用正则替换所有 ${...} 占位符
	var firstErr error
	replaced = varPlaceHolderRe.ReplaceAllStringFunc(
		text,
		func(m string) string {
			key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(m, "${"), "}"))
			if key == "" {
				return m
			}
			newVal, err := valueGetter(key)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				// 出错时保留原占位符，便于排查
				return m
			}
			return newVal
		})

	if firstErr != nil {
		return replaced, firstErr
	}
	return replaced, nil
}

type valueGetterFunc func(key string) (val string, err error)

var (
	varPlaceHolderRe = regexp.MustCompile(`\$\{([^}]+)\}`)
)

// loadSecretsMap loads a YAML map from AWS Secrets Manager.
// Secret id precedence: p.secretsName -> APP_SECRETS_NAME env -> "<env>/<appName>-secrets".
func (p *ConfigVarResolver) loadSecretsMap(ctx context.Context, appName string, appEnv string) (secrets map[string]string, err error) {
	secretName := p.secretsName
	if secretName == "" {
		secretName = strings.TrimSpace(os.Getenv("APP_SECRETS_NAME"))
		if secretName == "" {
			if strings.TrimSpace(appName) == "" {
				return nil, fmt.Errorf("app name is empty")
			}
			if strings.TrimSpace(appEnv) == "" {
				return nil, fmt.Errorf("env name is empty")
			}
			secretName = fmt.Sprintf("%s/%s-secrets", appEnv, appName)
		}
	}
	logger.Infof("load secrets from aws secrets manager name: %s", secretName)

	secrets = map[string]string{}
	val, err := getSecretString(ctx, secretName)
	if err != nil {
		logger.Errorf("get secret '%s' failed: %v", secretName, err)
		return
	}
	if strings.TrimSpace(val) == "" {
		return secrets, nil
	}
	if err = yaml.Unmarshal([]byte(val), &secrets); err != nil {
		logger.Errorf("unmarshal secrets '%s' failed: %v", secretName, err)
		return
	}
	return
}

const (
	refPrefix = "ref:"
	envPrefix = "env:"
)

func isRef(input string) bool { return strings.HasPrefix(input, refPrefix) }
func isEnv(input string) bool { return strings.HasPrefix(input, envPrefix) }

func parseRef(input string) (string, error) {
	if !strings.HasPrefix(input, refPrefix) {
		return "", fmt.Errorf("not a ref value")
	}
	v := strings.TrimSpace(strings.TrimPrefix(input, refPrefix))
	if v == "" {
		return "", fmt.Errorf("empty ref value")
	}
	return v, nil
}

func parseEnvVal(input string) (string, error) {
	envName := strings.TrimSpace(strings.TrimPrefix(input, envPrefix))
	v := os.Getenv(envName)
	if v == "" {
		return "", fmt.Errorf("`%s` is not set in environment", envName)
	}
	return v, nil
}

// secretFetcher allows injecting a custom fetcher from outside to avoid import cycles
var secretFetcher func(ctx context.Context, secretID string) (string, error)

// SetSecretFetcher sets a custom secret fetcher implementation
func SetSecretFetcher(f func(ctx context.Context, secretID string) (string, error)) {
	secretFetcher = f
}

func getSecretString(ctx context.Context, secretID string) (string, error) {
	if secretFetcher != nil {
		return secretFetcher(ctx, secretID)
	}
	sm, err := aws.NewSecretsManager()
	if err != nil {
		return "", err
	}
	if sm == nil {
		return "", fmt.Errorf("secrets manager is nil")
	}
	return sm.GetSecretString(ctx, secretID)
}
