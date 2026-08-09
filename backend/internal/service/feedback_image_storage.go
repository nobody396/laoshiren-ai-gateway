package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
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

var ErrFeedbackImageTokenInvalid = errors.New("invalid feedback image token")
var ErrFeedbackImageNotFound = errors.New("feedback image not found")

// S3FeedbackImageStorage stores private images in configured S3 or the durable
// application data volume, and exposes only HMAC-signed proxy URLs.
type S3FeedbackImageStorage struct {
	cfg    *config.Config
	mu     sync.Mutex
	client *s3.Client
	bucket string
}

func NewFeedbackImageStorage(cfg *config.Config) *S3FeedbackImageStorage {
	return &S3FeedbackImageStorage{cfg: cfg}
}

func (s *S3FeedbackImageStorage) Enabled(context.Context) bool {
	return s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.JWT.Secret) != ""
}

func (s *S3FeedbackImageStorage) resolvedConfig() config.GPTImageS3Config {
	cfg := s.cfg.Gateway.FeedbackS3
	if cfg.IsConfigured() {
		return cfg
	}
	fallback := s.cfg.Gateway.GPTImageS3
	fallback.Prefix = cfg.Prefix
	if strings.TrimSpace(fallback.Prefix) == "" {
		fallback.Prefix = "feedback-images/"
	}
	if cfg.MaxImageBytes > 0 {
		fallback.MaxImageBytes = cfg.MaxImageBytes
	}
	return fallback
}

func (s *S3FeedbackImageStorage) UploadObject(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) error {
	if !s.Enabled(ctx) {
		return ErrFeedbackUploadUnavailable
	}
	cfg := s.resolvedConfig()
	maxBytes := cfg.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = 5 << 20
	}
	if size <= 0 || size > maxBytes {
		return fmt.Errorf("feedback image exceeds size limit")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/") {
		return fmt.Errorf("feedback upload is not an image")
	}
	if !cfg.IsConfigured() {
		return writeLocalFeedbackImage(cfg.Prefix, objectKey, body, size)
	}
	client, bucket, err := s.getClient(ctx)
	if err != nil {
		return err
	}
	key := path.Join(strings.Trim(cfg.Prefix, "/"), strings.TrimLeft(objectKey, "/"))
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &key, Body: body, ContentLength: aws.Int64(size), ContentType: &contentType,
	})
	if err != nil {
		return fmt.Errorf("upload feedback image to S3: %w", err)
	}
	return nil
}

func (s *S3FeedbackImageStorage) GetAccessURL(_ context.Context, objectKey string) (string, error) {
	if strings.TrimSpace(objectKey) == "" {
		return "", ErrFeedbackImageTokenInvalid
	}
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.JWT.Secret) == "" {
		return "", ErrFeedbackUploadUnavailable
	}
	key := path.Join(strings.Trim(s.resolvedConfig().Prefix, "/"), strings.TrimLeft(objectKey, "/"))
	payload := base64.RawURLEncoding.EncodeToString([]byte(key))
	token := payload + "." + signFeedbackImagePayload(payload, s.cfg.JWT.Secret)
	return "/api/v1/feedback-images/" + token, nil
}

func (s *S3FeedbackImageStorage) ResolveToken(ctx context.Context, token string, expiry time.Duration) (*FeedbackImageAccess, error) {
	if !s.Enabled(ctx) {
		return nil, ErrFeedbackUploadUnavailable
	}
	objectKey, err := decodeFeedbackImageToken(token, s.cfg.JWT.Secret)
	if err != nil {
		return nil, err
	}
	if local, localErr := openLocalFeedbackImage(objectKey); localErr == nil {
		return local, nil
	} else if !errors.Is(localErr, ErrFeedbackImageNotFound) {
		return nil, localErr
	}
	if !s.resolvedConfig().IsConfigured() {
		return nil, ErrFeedbackImageNotFound
	}
	client, bucket, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &objectKey}, s3.WithPresignExpires(expiry))
	if err != nil {
		return nil, fmt.Errorf("presign feedback image: %w", err)
	}
	return &FeedbackImageAccess{RedirectURL: result.URL}, nil
}

func writeLocalFeedbackImage(prefix, objectKey string, body io.Reader, size int64) error {
	storageKey := path.Join(strings.Trim(prefix, "/"), strings.TrimLeft(objectKey, "/"))
	filename, err := localFeedbackImagePath(storageKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return fmt.Errorf("create feedback image directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".feedback-upload-*")
	if err != nil {
		return fmt.Errorf("create feedback image temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure feedback image file: %w", err)
	}
	written, copyErr := io.Copy(temporary, io.LimitReader(body, size+1))
	if copyErr == nil && written != size {
		copyErr = fmt.Errorf("feedback image size changed during upload")
	}
	if syncErr := temporary.Sync(); copyErr == nil && syncErr != nil {
		copyErr = syncErr
	}
	if closeErr := temporary.Close(); copyErr == nil && closeErr != nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return fmt.Errorf("write feedback image: %w", copyErr)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("publish feedback image: %w", err)
	}
	return nil
}

func openLocalFeedbackImage(storageKey string) (*FeedbackImageAccess, error) {
	filename, err := localFeedbackImagePath(storageKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrFeedbackImageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open feedback image: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat feedback image: %w", err)
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &FeedbackImageAccess{Reader: file, ContentType: contentType, ContentLength: info.Size()}, nil
}

func localFeedbackImagePath(storageKey string) (string, error) {
	clean := path.Clean(strings.TrimSpace(storageKey))
	if !isValidFeedbackStorageKey(clean) {
		return "", ErrFeedbackImageTokenInvalid
	}
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		if info, err := os.Stat("/app/data"); err == nil && info.IsDir() {
			dataDir = "/app/data"
		} else {
			dataDir = "data"
		}
	}
	base, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve feedback data directory: %w", err)
	}
	filename := filepath.Join(base, filepath.FromSlash(clean))
	relative, err := filepath.Rel(base, filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrFeedbackImageTokenInvalid
	}
	return filename, nil
}

func decodeFeedbackImageToken(token, secret string) (string, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || strings.TrimSpace(secret) == "" {
		return "", ErrFeedbackImageTokenInvalid
	}
	expected, err := hex.DecodeString(signFeedbackImagePayload(parts[0], secret))
	if err != nil {
		return "", ErrFeedbackImageTokenInvalid
	}
	actual, err := hex.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actual, expected) {
		return "", ErrFeedbackImageTokenInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(raw) == 0 {
		return "", ErrFeedbackImageTokenInvalid
	}
	key := path.Clean(string(raw))
	if !isValidFeedbackStorageKey(key) {
		return "", ErrFeedbackImageTokenInvalid
	}
	return key, nil
}

func isValidFeedbackStorageKey(key string) bool {
	return key != "." && !strings.HasPrefix(key, "../") && !strings.HasPrefix(key, "/") &&
		(strings.Contains(key, "/feedbacks/") || strings.HasPrefix(key, "feedbacks/"))
}

func signFeedbackImagePayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *S3FeedbackImageStorage) getClient(ctx context.Context) (*s3.Client, string, error) {
	if !s.Enabled(ctx) {
		return nil, "", ErrFeedbackUploadUnavailable
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
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if strings.TrimSpace(cfg.AccessKeyID) != "" || strings.TrimSpace(cfg.SecretAccessKey) != "" {
		if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
			return nil, "", fmt.Errorf("both S3 credentials are required")
		}
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, "", fmt.Errorf("load feedback S3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
		o.UsePathStyle = cfg.ForcePathStyle
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	s.client, s.bucket = client, cfg.Bucket
	return client, cfg.Bucket, nil
}
