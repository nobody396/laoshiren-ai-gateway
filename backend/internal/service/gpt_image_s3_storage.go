package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/urlvalidator"
)

var (
	ErrGPTImageS3NotConfigured = errors.New("gpt-image S3 storage is not configured")
	ErrGPTImageS3ImageTooLarge = errors.New("gpt-image upstream image exceeds size limit")
)

type GPTImageS3Storage struct {
	cfg        *config.Config
	httpClient *http.Client

	mu     sync.Mutex
	client *s3.Client
	bucket string
}

type GPTImageStoredObject struct {
	Key         string
	SizeBytes   int64
	ContentType string
}

func NewGPTImageS3Storage(cfg *config.Config) *GPTImageS3Storage {
	return &GPTImageS3Storage{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return validateGPTImageDownloadURL(req.URL.String())
			},
		},
	}
}

func (s *GPTImageS3Storage) Enabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.GPTImageS3.IsConfigured()
}

func (s *GPTImageS3Storage) UploadFromURL(ctx context.Context, userID int64, taskID string, index int, sourceURL string) (*GPTImageStoredObject, error) {
	if !s.Enabled() {
		return nil, ErrGPTImageS3NotConfigured
	}
	if err := validateGPTImageDownloadURL(sourceURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sub2api-gpt-image-storage/1.0")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download upstream image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download upstream image returned status %d", resp.StatusCode)
	}

	maxBytes := s.cfg.Gateway.GPTImageS3.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = 20 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upstream image: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrGPTImageS3ImageTooLarge
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return nil, fmt.Errorf("upstream content type is not image: %s", contentType)
	}

	key := s.objectKey(userID, taskID, index, extensionForImageContentType(contentType))
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
		return nil, fmt.Errorf("upload image to S3: %w", err)
	}
	return &GPTImageStoredObject{Key: key, SizeBytes: int64(len(data)), ContentType: contentType}, nil
}

func (s *GPTImageS3Storage) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	if !s.Enabled() {
		return nil, "", 0, ErrGPTImageS3NotConfigured
	}
	client, bucket, err := s.getClient(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &objectKey,
	})
	if err != nil {
		return nil, "", 0, fmt.Errorf("get S3 image object: %w", err)
	}
	contentType := "application/octet-stream"
	if out.ContentType != nil && strings.TrimSpace(*out.ContentType) != "" {
		contentType = strings.TrimSpace(*out.ContentType)
	}
	size := int64(0)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, contentType, size, nil
}

func (s *GPTImageS3Storage) objectKey(userID int64, taskID string, index int, ext string) string {
	prefix := "gpt-image/"
	if s.cfg != nil && strings.TrimSpace(s.cfg.Gateway.GPTImageS3.Prefix) != "" {
		prefix = strings.TrimSpace(s.cfg.Gateway.GPTImageS3.Prefix)
	}
	prefix = strings.Trim(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	now := time.Now().UTC()
	cleanTaskID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, taskID)
	return path.Join(
		strings.TrimSuffix(prefix, "/"),
		strconv.FormatInt(userID, 10),
		now.Format("2006"),
		now.Format("01"),
		cleanTaskID,
		fmt.Sprintf("%d%s", index, ext),
	)
}

func (s *GPTImageS3Storage) getClient(ctx context.Context) (*s3.Client, string, error) {
	if !s.Enabled() {
		return nil, "", ErrGPTImageS3NotConfigured
	}
	cfg := s.cfg.Gateway.GPTImageS3
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

func validateGPTImageDownloadURL(raw string) error {
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{AllowPrivate: false})
	if err != nil {
		return fmt.Errorf("invalid upstream image url: %w", err)
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return fmt.Errorf("invalid upstream image url: %w", err)
	}
	if err := urlvalidator.ValidateResolvedIP(parsed.Hostname()); err != nil {
		return fmt.Errorf("upstream image url is not allowed: %w", err)
	}
	return nil
}

func extensionForImageContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/png":
		return ".png"
	default:
		return ".png"
	}
}
