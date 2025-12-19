package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	awsaws "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
)

// SecretsManager wraps AWS Secrets Manager client
type SecretsManager struct {
	client *secretsmanager.Client
}

// NewSecretsManager creates a Secrets Manager client using default credential chain.
// If explicit region or credentials are provided via env, they will be honored; otherwise fallback to default chain.
func NewSecretsManager() (*SecretsManager, error) {
	return NewSecretsManagerWithCredentials(GetAWSRegion(), GetAWSAccessKeyID(), GetAWSSecretAccessKey())
}

// NewSecretsManagerWithCredentials creates a Secrets Manager client with optional explicit region/credentials.
func NewSecretsManagerWithCredentials(region, awsAccessKeyID, awsSecretAccessKey string) (*SecretsManager, error) {
    loadOpts := []func(*awsconfig.LoadOptions) error{}
    if region != "" {
        loadOpts = append(loadOpts, awsconfig.WithRegion(region))
    }
    // 有凭证就用静态凭证；否则如果 Region 也为空，则启用 IMDS 兜底解析
    if awsAccessKeyID != "" && awsSecretAccessKey != "" {
        loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessKeyID, awsSecretAccessKey, "")))
    } else if region == "" {
        // 当 AK/SK 缺省且 Region 也为空时，通过 IMDS 解析 Region
        loadOpts = append(loadOpts, awsconfig.WithEC2IMDSRegion())
    }
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		logger.Errorf("failed to load AWS config for SecretsManager, err:%v", err)
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)
	return &SecretsManager{client: client}, nil
}

// GetSecretString retrieves the secret value as string by secret ID or ARN
func (s *SecretsManager) GetSecretString(ctx context.Context, secretID string) (string, error) {
	if secretID == "" {
		return "", fmt.Errorf("secret id is empty")
	}
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: awsaws.String(secretID)})
	if err != nil {
		if isSecretsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if out.SecretString != nil {
		return awsaws.ToString(out.SecretString), nil
	}
	if out.SecretBinary != nil {
		return string(out.SecretBinary), nil
	}
	return "", nil
}

// GetSecretBytes retrieves the secret value as bytes
func (s *SecretsManager) GetSecretBytes(ctx context.Context, secretID string) ([]byte, error) {
	if secretID == "" {
		return nil, fmt.Errorf("secret id is empty")
	}
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: awsaws.String(secretID)})
	if err != nil {
		if isSecretsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if out.SecretBinary != nil {
		return out.SecretBinary, nil
	}
	if out.SecretString != nil {
		return []byte(awsaws.ToString(out.SecretString)), nil
	}
	return nil, nil
}

// GetSecretJSON unmarshals the secret string into target (typical JSON use-case)
func (s *SecretsManager) GetSecretJSON(ctx context.Context, secretID string, target interface{}) error {
	val, err := s.GetSecretString(ctx, secretID)
	if err != nil {
		return err
	}
	if val == "" {
		return nil
	}
	return json.Unmarshal([]byte(val), target)
}

func isSecretsNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		if code == "ResourceNotFoundException" || code == "NotFound" {
			return true
		}
	}
	return false
}
