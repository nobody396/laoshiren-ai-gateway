package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
)

var (
	ErrFinanceReceiptS3NotConfigured = errors.New("finance-receipt S3 storage is not configured")
	ErrFinanceReceiptTooLarge        = errors.New("finance receipt image exceeds size limit")
	ErrFinanceReceiptNotImage        = errors.New("finance receipt content type is not an image")
)

type FinanceReceiptStoredObject struct {
	Key         string
	SizeBytes   int64
	ContentType string
}

// FinanceReceiptS3Storage 把记账凭证图片私有存储到 S3，只回传 key，读取通过预签名 URL。
type FinanceReceiptS3Storage struct {
	cfg *config.Config

	mu     sync.Mutex
	client *s3.Client
	bucket string
}

func NewFinanceReceiptS3Storage(cfg *config.Config) *FinanceReceiptS3Storage {
	return &FinanceReceiptS3Storage{cfg: cfg}
}

func (s *FinanceReceiptS3Storage) Enabled() bool {
	if s == nil || s.cfg == nil {
		return false
	}
	return s.cfg.Gateway.FinanceReceiptS3.IsConfigured() || s.cfg.Gateway.GPTImageS3.IsConfigured()
}

// resolvedConfig 返回实际生效的 S3 配置：优先使用专门配置的 finance_receipt_s3；
// 未配置时退回复用已有的 gpt_image_s3 桶（同一套凭证/endpoint），仅前缀不同，
// 避免为凭证图片单独申请一个新桶。
func (s *FinanceReceiptS3Storage) resolvedConfig() config.GPTImageS3Config {
	cfg := s.cfg.Gateway.FinanceReceiptS3
	if cfg.IsConfigured() {
		return cfg
	}
	fallback := s.cfg.Gateway.GPTImageS3
	fallback.Prefix = cfg.Prefix
	if strings.TrimSpace(fallback.Prefix) == "" {
		fallback.Prefix = "finance-receipts/"
	}
	if cfg.MaxImageBytes > 0 {
		fallback.MaxImageBytes = cfg.MaxImageBytes
	}
	return fallback
}

// Upload 保存已经在调用方压缩过的凭证图片，返回其 S3 key。
func (s *FinanceReceiptS3Storage) Upload(ctx context.Context, createdBy int64, data []byte, contentType string) (*FinanceReceiptStoredObject, error) {
	if !s.Enabled() {
		return nil, ErrFinanceReceiptS3NotConfigured
	}

	contentType = strings.TrimSpace(contentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return nil, ErrFinanceReceiptNotImage
	}

	resolved := s.resolvedConfig()
	maxBytes := resolved.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = 8 * 1024 * 1024
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrFinanceReceiptTooLarge
	}

	key := s.objectKey(resolved, createdBy, extensionForImageContentType(contentType))
	client, bucket, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &key,
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   &contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload finance receipt to S3: %w", err)
	}
	return &FinanceReceiptStoredObject{Key: key, SizeBytes: int64(len(data)), ContentType: contentType}, nil
}

// PresignGetURL 生成一个限时可用的私有读取链接，供前端展示凭证图片。
func (s *FinanceReceiptS3Storage) PresignGetURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if !s.Enabled() {
		return "", ErrFinanceReceiptS3NotConfigured
	}
	client, bucket, err := s.getClient(ctx)
	if err != nil {
		return "", err
	}
	presignClient := s3.NewPresignClient(client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &objectKey,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign finance receipt url: %w", err)
	}
	return result.URL, nil
}

func (s *FinanceReceiptS3Storage) objectKey(resolved config.GPTImageS3Config, createdBy int64, ext string) string {
	prefix := "finance-receipts/"
	if strings.TrimSpace(resolved.Prefix) != "" {
		prefix = strings.TrimSpace(resolved.Prefix)
	}
	prefix = strings.Trim(prefix, "/")

	now := time.Now().UTC()
	actor := "0"
	if createdBy > 0 {
		actor = strconv.FormatInt(createdBy, 10)
	}
	return path.Join(
		prefix,
		now.Format("2006"),
		now.Format("01"),
		fmt.Sprintf("%s-%d%s", actor, now.UnixNano(), ext),
	)
}

func (s *FinanceReceiptS3Storage) getClient(ctx context.Context) (*s3.Client, string, error) {
	if !s.Enabled() {
		return nil, "", ErrFinanceReceiptS3NotConfigured
	}
	cfg := s.resolvedConfig()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && s.bucket == cfg.Bucket {
		return s.client, s.bucket, nil
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if strings.TrimSpace(cfg.AccessKeyID) != "" || strings.TrimSpace(cfg.SecretAccessKey) != "" {
		if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
			return nil, "", fmt.Errorf("both access_key_id and secret_access_key are required when static S3 credentials are configured")
		}
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, "", fmt.Errorf("load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if strings.TrimSpace(cfg.Endpoint) != "" {
			endpoint := strings.TrimSpace(cfg.Endpoint)
			o.BaseEndpoint = &endpoint
		}
		o.UsePathStyle = cfg.ForcePathStyle
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	s.client = client
	s.bucket = cfg.Bucket
	return client, cfg.Bucket, nil
}
