package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultCodexImagePreviewTTL      = 24 * time.Hour
	defaultCodexImagePreviewMaxBytes = int64(20 * 1024 * 1024)
	codexImagePreviewTokenBytes      = 32
)

var (
	ErrCodexImagePreviewDisabled = errors.New("codex image preview storage is disabled")
	ErrCodexImagePreviewNotFound = errors.New("codex image preview not found")
	ErrCodexImagePreviewTooLarge = errors.New("codex image preview exceeds size limit")

	codexImagePreviewTokenPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	codexImagePreviewCleanupState = struct {
		sync.Mutex
		lastRun time.Time
	}{}
)

type CodexImagePreview struct {
	Token       string
	ContentType string
	SizeBytes   int64
}

// StoreCodexImagePreviewsBase64 stores a complete image set atomically from
// the caller's perspective. If any member is invalid or cannot be persisted,
// already-written members are removed so a multi-image response is never
// delivered partially.
func (s *OpenAIGatewayService) StoreCodexImagePreviewsBase64(values []string) ([]*CodexImagePreview, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one Codex image preview is required")
	}
	previews := make([]*CodexImagePreview, 0, len(values))
	tokens := make([]string, 0, len(values))
	for _, value := range values {
		preview, err := s.StoreCodexImagePreviewBase64(value)
		if err != nil {
			s.DeleteCodexImagePreviews(tokens)
			return nil, err
		}
		previews = append(previews, preview)
		tokens = append(tokens, preview.Token)
	}
	return previews, nil
}

// StoreCodexImagePreviewBase64 persists one validated generated image under a
// cryptographically random token. The token contains no user, account, request
// or prompt data and is safe to embed in a short-lived public URL.
func (s *OpenAIGatewayService) StoreCodexImagePreviewBase64(value string) (*CodexImagePreview, error) {
	dir, ttl, maxBytes, err := s.codexImagePreviewConfig()
	if err != nil {
		return nil, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("codex image preview is empty")
	}
	if int64(base64.StdEncoding.DecodedLen(len(value))) > maxBytes+2 {
		return nil, ErrCodexImagePreviewTooLarge
	}
	data, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode Codex image preview: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrCodexImagePreviewTooLarge
	}
	contentType, err := codexImagePreviewContentType(data)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create Codex image preview directory: %w", err)
	}

	tokenBytes := make([]byte, codexImagePreviewTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate Codex image preview token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	finalPath := filepath.Join(dir, token)
	tempPath := finalPath + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create Codex image preview: %w", codexImagePreviewFileError(err))
	}
	removeTemp := true
	defer func() {
		_ = file.Close()
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return nil, fmt.Errorf("write Codex image preview: %w", codexImagePreviewFileError(err))
	}
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("sync Codex image preview: %w", codexImagePreviewFileError(err))
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close Codex image preview: %w", codexImagePreviewFileError(err))
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, fmt.Errorf("publish Codex image preview: %w", codexImagePreviewFileError(err))
	}
	removeTemp = false
	s.cleanupExpiredCodexImagePreviews(dir, ttl)
	return &CodexImagePreview{Token: token, ContentType: contentType, SizeBytes: int64(len(data))}, nil
}

// OpenCodexImagePreview opens a non-expired preview. Remote path traversal is
// rejected before any filesystem lookup by the exact 256-bit token grammar.
func (s *OpenAIGatewayService) OpenCodexImagePreview(token string) (io.ReadCloser, string, int64, time.Duration, error) {
	dir, ttl, maxBytes, err := s.codexImagePreviewConfig()
	if err != nil {
		return nil, "", 0, 0, err
	}
	// The route is retained read-only for links emitted by the short-lived
	// preview release. New Codex responses no longer create these files.
	s.cleanupExpiredCodexImagePreviews(dir, ttl)
	token = strings.TrimSpace(token)
	if !codexImagePreviewTokenPattern.MatchString(token) {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	filePath := filepath.Join(dir, token)
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	if time.Since(info.ModTime()) > ttl {
		_ = os.Remove(filePath)
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	if info.Size() <= 0 || info.Size() > maxBytes {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = file.Close()
		}
	}()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || openedInfo.Size() <= 0 || openedInfo.Size() > maxBytes {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	if time.Since(openedInfo.ModTime()) > ttl {
		_ = os.Remove(filePath)
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	header := make([]byte, minInt64(openedInfo.Size(), 512))
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	contentType, err := codexImagePreviewContentType(header)
	if err != nil {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", 0, 0, ErrCodexImagePreviewNotFound
	}
	remainingTTL := ttl - time.Since(openedInfo.ModTime())
	if remainingTTL < 0 {
		remainingTTL = 0
	}
	closeOnError = false
	return file, contentType, openedInfo.Size(), remainingTTL, nil
}

func (s *OpenAIGatewayService) DeleteCodexImagePreviews(tokens []string) {
	dir, _, _, err := s.codexImagePreviewConfig()
	if err != nil {
		return
	}
	for _, token := range tokens {
		if codexImagePreviewTokenPattern.MatchString(token) {
			_ = os.Remove(filepath.Join(dir, token))
		}
	}
}

func (s *OpenAIGatewayService) codexImagePreviewConfig() (string, time.Duration, int64, error) {
	if s == nil || s.cfg == nil || !s.cfg.Gateway.CodexImagePreview.Enabled {
		return "", 0, 0, ErrCodexImagePreviewDisabled
	}
	dir := strings.TrimSpace(s.cfg.Gateway.CodexImagePreview.DataDir)
	if dir == "" {
		dir = "./data/codex-image-previews"
	}
	ttl := time.Duration(s.cfg.Gateway.CodexImagePreview.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultCodexImagePreviewTTL
	}
	maxBytes := s.cfg.Gateway.CodexImagePreview.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = defaultCodexImagePreviewMaxBytes
	}
	return filepath.Clean(dir), ttl, maxBytes, nil
}

func codexImagePreviewContentType(data []byte) (string, error) {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return contentType, nil
	default:
		return "", fmt.Errorf("codex image preview has unsupported content type %q", contentType)
	}
}

func codexImagePreviewFileError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) && linkErr.Err != nil {
		return linkErr.Err
	}
	return err
}

func (s *OpenAIGatewayService) cleanupExpiredCodexImagePreviews(dir string, ttl time.Duration) {
	now := time.Now()
	codexImagePreviewCleanupState.Lock()
	if now.Sub(codexImagePreviewCleanupState.lastRun) < 10*time.Minute {
		codexImagePreviewCleanupState.Unlock()
		return
	}
	codexImagePreviewCleanupState.lastRun = now
	codexImagePreviewCleanupState.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !codexImagePreviewTokenPattern.MatchString(name) && !strings.HasSuffix(name, ".tmp") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.IsDir() {
			continue
		}
		if now.Sub(info.ModTime()) > ttl {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

func minInt64(a int64, b int64) int {
	if a < b {
		return int(a)
	}
	return int(b)
}
