package aws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type S3 struct {
	Client *s3.Client
}

// NewS3 创建一个S3客户端（从S3配置），默认回退到 AWS 凭证链。
func NewS3(s3cfg *config.S3Config) (*S3, error) {
	if s3cfg == nil {
		return nil, fmt.Errorf("s3 config is nil")
	}
	return NewS3WithCredentials(s3cfg, GetAWSAccessKeyID(), GetAWSSecretAccessKey(), "")
}

// NewS3WithCredentials 创建一个S3客户端（从S3配置 + 显式凭证）。s3cfg 可为 nil。
func NewS3WithCredentials(s3cfg *config.S3Config, awsAccessKeyID, awsSecretAccessKey, awsSessionToken string) (*S3, error) {
	var region string
	var endpoint string
	if s3cfg != nil {
		region = s3cfg.Region
		endpoint = s3cfg.Endpoint
	}
    loadOpts := []func(*awsconfig.LoadOptions) error{}
    if region != "" {
        loadOpts = append(loadOpts, awsconfig.WithRegion(region))
    }
    // 有凭证就用静态凭证；否则如果 Region 也为空，则启用 IMDS 兜底解析
    if awsAccessKeyID != "" && awsSecretAccessKey != "" {
        loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessKeyID, awsSecretAccessKey, awsSessionToken)))
    } else if region == "" {
        // 当 AK/SK 缺省且 Region 也为空时，通过 IMDS 解析 Region
        loadOpts = append(loadOpts, awsconfig.WithEC2IMDSRegion())
    }
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		logger.Errorf("failed to load AWS config for S3, err:%v", err)
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return &S3{Client: client}, nil
}

// S3ObjectInfo 对象的基本信息
type S3ObjectInfo struct {
	Bucket          string
	Key             string
	ETag            string
	Size            int64
	ContentType     string
	ContentEncoding string
	LastModified    time.Time
	Metadata        map[string]string
}

// Exists 检查对象是否存在；存在则返回基本信息
func (c *S3) Exists(ctx context.Context, bucket, key string) (bool, *S3ObjectInfo, error) {
	if bucket == "" {
		return false, nil, fmt.Errorf("bucket is empty")
	}
	if key == "" {
		return false, nil, fmt.Errorf("key is empty")
	}
	out, err := c.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// 404/NotFound 视为不存在
		if isS3NotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	info := s3ObjectInfoFromHead(bucket, key, out)
	return true, info, nil
}

// Get 读取对象内容（一次性加载到内存）
func (c *S3) Get(ctx context.Context, bucket, key string) (data []byte, info *S3ObjectInfo, err error) {
	if bucket == "" {
		err = fmt.Errorf("bucket is empty")
		return
	}
	if key == "" {
		err = fmt.Errorf("key is empty")
		return
	}
	out, err := c.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return
	}
	defer out.Body.Close()
	data, err = io.ReadAll(out.Body)
	if err != nil {
		return
	}
	info = s3ObjectInfoFromGet(bucket, key, out)
	return data, info, nil
}

// GetStream 将对象内容直接写入提供的 Writer（流式输出，零拷贝至调用方 writer）
func (c *S3) GetStream(ctx context.Context, bucket, key string, w io.Writer) (written int64, info *S3ObjectInfo, err error) {
	if bucket == "" {
		err = fmt.Errorf("bucket is empty")
		return
	}
	if key == "" {
		err = fmt.Errorf("key is empty")
		return
	}
	if w == nil {
		err = fmt.Errorf("writer is nil")
		return
	}
	out, err := c.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return
	}
	defer out.Body.Close()
	written, err = io.Copy(w, out.Body)
	if err != nil {
		return
	}
	info = s3ObjectInfoFromGet(bucket, key, out)
	return
}

// GetReader 流式读取对象内容（返回 ReadCloser，调用方需负责关闭）
func (c *S3) GetReader(ctx context.Context, bucket, key string) (reader io.ReadCloser, info *S3ObjectInfo, err error) {
	if bucket == "" {
		err = fmt.Errorf("bucket is empty")
		return
	}
	if key == "" {
		err = fmt.Errorf("key is empty")
		return
	}
	out, err := c.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return
	}

	return out.Body, s3ObjectInfoFromGet(bucket, key, out), nil
}

// GetFile 将对象内容保存到本地文件（会自动创建父目录）
func (c *S3) GetFile(ctx context.Context, bucket, key, filePath string) (written int64, info *S3ObjectInfo, err error) {
	if filePath == "" {
		err = fmt.Errorf("file path is empty")
		return
	}
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return
		}
	}
	f, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("failed to close file, err:%v", closeErr)
		}
	}()
	return c.GetStream(ctx, bucket, key, f)
}

// Put 写入对象内容
func (c *S3) Put(ctx context.Context, bucket, key string, body []byte, contentType string, metadata map[string]string, contentEncoding string) (info *S3ObjectInfo, err error) {
	if bucket == "" {
		err = fmt.Errorf("bucket is empty")
		return
	}
	if key == "" {
		err = fmt.Errorf("key is empty")
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}
	if contentEncoding != "" {
		input.ContentEncoding = aws.String(contentEncoding)
	}
	out, err := c.Client.PutObject(ctx, input)
	if err != nil {
		return
	}
	return s3ObjectInfoFromPut(bucket, key, out), nil
}

// PutStream 流式写入对象内容（body 不会被缓存到内存）。
func (c *S3) PutStream(ctx context.Context, bucket, key string, body io.Reader, contentType string, contentLength int64, metadata map[string]string, contentEncoding string) (info *S3ObjectInfo, err error) {
	if bucket == "" {
		err = fmt.Errorf("bucket is empty")
		return
	}
	if key == "" {
		err = fmt.Errorf("key is empty")
		return
	}
	if body == nil {
		err = fmt.Errorf("body is nil")
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}
	input.ContentLength = aws.Int64(contentLength)
	if contentEncoding != "" {
		input.ContentEncoding = aws.String(contentEncoding)
	}
	out, err := c.Client.PutObject(ctx, input)
	if err != nil {
		return
	}
	return s3ObjectInfoFromPut(bucket, key, out), nil
}

// PutFile 将本地文件上传到 S3（自动读取文件大小）
func (c *S3) PutFile(ctx context.Context, bucket, key, filePath, contentType string, metadata map[string]string, contentEncoding string) (info *S3ObjectInfo, err error) {
	if filePath == "" {
		err = fmt.Errorf("file path is empty")
		return
	}
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}
	return c.PutStream(ctx, bucket, key, f, contentType, fi.Size(), metadata, contentEncoding)
}

// CopyObject 拷贝对象（跨Region复制：客户端应使用目标Bucket所在Region）
func (c *S3) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) (info *S3ObjectInfo, err error) {
	if srcBucket == "" || dstBucket == "" {
		err = fmt.Errorf("source or destination bucket is empty")
		return
	}
	if srcKey == "" || dstKey == "" {
		err = fmt.Errorf("source or destination key is empty")
		return
	}
	// AWS 要求 CopySource 形如 "bucket/key" 且 key 需要 URL-encode
	copySource := fmt.Sprintf("%s/%s", srcBucket, url.PathEscape(srcKey))
	out, err := c.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		logger.Errorf("failed to copy object, err:%v, out:%+v", err, out)
		return
	}
	info = s3ObjectInfoFromCopy(dstBucket, dstKey, out)
	return
}

// CopyObjectWithRegion 拷贝对象（支持跨Region，按目标Region发起请求）
func (c *S3) CopyObjectWithRegion(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey, dstRegion string) (info *S3ObjectInfo, err error) {
	if dstRegion == "" {
		err = fmt.Errorf("destination region is empty")
		return
	}
	if srcBucket == "" || dstBucket == "" {
		err = fmt.Errorf("source or destination bucket is empty")
		return
	}
	if srcKey == "" || dstKey == "" {
		err = fmt.Errorf("source or destination key is empty")
		return
	}
	copySource := fmt.Sprintf("%s/%s", srcBucket, url.PathEscape(srcKey))
	out, err := c.Client.CopyObject(ctx,
		&s3.CopyObjectInput{
			Bucket:     aws.String(dstBucket),
			Key:        aws.String(dstKey),
			CopySource: aws.String(copySource),
		},
		func(o *s3.Options) {
			o.Region = dstRegion
		})
	if err != nil {
		logger.Errorf("failed to copy object with region %s, copySource: %s, dstBucket: %s, dstKey: %s, err:%v, out:%+v", dstRegion, copySource, dstBucket, dstKey, err, out)
		return
	}
	info = s3ObjectInfoFromCopy(dstBucket, dstKey, out)
	return
}

func (c *S3) DeleteObject(ctx context.Context, bucket, key string) (err error) {
	if bucket == "" {
		err = fmt.Errorf("bucket is empty")
		return
	}
	if key == "" {
		err = fmt.Errorf("key is empty")
		return
	}
	_, err = c.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// 404/NotFound 视为不存在
		if isS3NotFound(err) {
			return nil
		}
		return
	}
	return nil
}

func normalizeETag(e *string) string {
	if e == nil {
		return ""
	}
	return strings.Trim(*e, "\"")
}

func s3ObjectInfoFromHead(bucket, key string, out *s3.HeadObjectOutput) *S3ObjectInfo {
	if out == nil {
		return nil
	}
	info := &S3ObjectInfo{
		Bucket:          bucket,
		Key:             key,
		ETag:            normalizeETag(out.ETag),
		Size:            aws.ToInt64(out.ContentLength),
		ContentType:     aws.ToString(out.ContentType),
		LastModified:    aws.ToTime(out.LastModified),
		Metadata:        out.Metadata,
		ContentEncoding: aws.ToString(out.ContentEncoding),
	}
	return info
}

func s3ObjectInfoFromGet(bucket, key string, out *s3.GetObjectOutput) *S3ObjectInfo {
	if out == nil {
		return nil
	}
	info := &S3ObjectInfo{
		Bucket:          bucket,
		Key:             key,
		ETag:            normalizeETag(out.ETag),
		Size:            aws.ToInt64(out.ContentLength),
		ContentType:     aws.ToString(out.ContentType),
		LastModified:    aws.ToTime(out.LastModified),
		Metadata:        out.Metadata,
		ContentEncoding: aws.ToString(out.ContentEncoding),
	}
	return info
}

func s3ObjectInfoFromPut(bucket, key string, out *s3.PutObjectOutput) *S3ObjectInfo {
	if out == nil {
		return nil
	}
	info := &S3ObjectInfo{
		Bucket: bucket,
		Key:    key,
		ETag:   normalizeETag(out.ETag),
		Size:   aws.ToInt64(out.Size),
	}
	return info
}

func s3ObjectInfoFromCopy(bucket, key string, out *s3.CopyObjectOutput) *S3ObjectInfo {
	if out == nil {
		return nil
	}

	copyObjectResult := out.CopyObjectResult
	if copyObjectResult == nil {
		return nil
	}

	info := &S3ObjectInfo{
		Bucket:       bucket,
		Key:          key,
		ETag:         normalizeETag(copyObjectResult.ETag),
		LastModified: aws.ToTime(copyObjectResult.LastModified),
	}
	return info
}

func isS3NotFound(err error) bool {
	// 优先匹配 S3 专用错误类型
	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	// 兼容 smithy 通用 API 错误码
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		if code == "NotFound" || code == "NoSuchKey" {
			return true
		}
	}
	return false
}
