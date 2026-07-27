package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var ErrAffiliateCommunityNotAvailable = infraerrors.NotFound(
	"AFFILIATE_COMMUNITY_NOT_AVAILABLE",
	"affiliate community is not available",
)

var ErrAffiliateCommunityRevisionConflict = infraerrors.Conflict(
	"AFFILIATE_COMMUNITY_REVISION_CONFLICT",
	"affiliate community settings changed; reload and retry",
)

type AffiliateCommunitySettings struct {
	Enabled            bool       `json:"enabled"`
	Title              string     `json:"title"`
	Message            string     `json:"message"`
	QRObjectKey        string     `json:"-"`
	QRContentType      string     `json:"-"`
	QROriginalFilename string     `json:"qr_original_filename,omitempty"`
	QRSize             int64      `json:"qr_size"`
	HasQRCode          bool       `json:"has_qr_code"`
	QRCodeURL          string     `json:"qr_code_url,omitempty"`
	Revision           int64      `json:"revision"`
	UpdatedBy          *int64     `json:"updated_by,omitempty"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

type AffiliateCommunityQRCodeUpload struct {
	Filename    string
	ContentType string
	Size        int64
	Body        io.Reader
}

type AffiliateCommunityQRCodeFile struct {
	Path        string
	ContentType string
	Filename    string
}

type AffiliateCommunityRepository interface {
	GetCommunitySettings(ctx context.Context) (*AffiliateCommunitySettings, error)
	UpdateCommunitySettings(
		ctx context.Context,
		title, message string,
		enabled bool,
		updatedBy int64,
		expectedRevision int64,
	) (*AffiliateCommunitySettings, error)
	UpdateCommunityQRCode(
		ctx context.Context,
		objectKey, contentType, originalName string,
		size int64,
		updatedBy int64,
	) (*AffiliateCommunitySettings, error)
}

type AffiliateCommunityService struct {
	repo AffiliateCommunityRepository
}

func NewAffiliateCommunityService(repo AffiliateCommunityRepository) *AffiliateCommunityService {
	return &AffiliateCommunityService{repo: repo}
}

func (s *AffiliateCommunityService) Get(ctx context.Context, agentView bool) (*AffiliateCommunitySettings, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate community repository is not configured")
	}
	settings, err := s.repo.GetCommunitySettings(ctx)
	if err != nil {
		return nil, err
	}
	normalizeAffiliateCommunity(settings)
	if agentView && !settings.Enabled {
		return &AffiliateCommunitySettings{Enabled: false, Revision: settings.Revision}, nil
	}
	return settings, nil
}

func (s *AffiliateCommunityService) Update(
	ctx context.Context,
	title, message string,
	enabled bool,
	updatedBy int64,
	expectedRevision int64,
) (*AffiliateCommunitySettings, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate community repository is not configured")
	}
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	if title == "" || len([]rune(title)) > 100 {
		return nil, infraerrors.BadRequest("INVALID_COMMUNITY_TITLE", "community title is required and must be at most 100 characters")
	}
	if len([]rune(message)) > 1_000 {
		return nil, infraerrors.BadRequest("INVALID_COMMUNITY_MESSAGE", "community message must be at most 1000 characters")
	}
	if updatedBy <= 0 || expectedRevision <= 0 {
		return nil, ErrInvalidInput
	}
	current, err := s.Get(ctx, false)
	if err != nil {
		return nil, err
	}
	if enabled && (!current.HasQRCode || message == "") {
		return nil, infraerrors.Conflict(
			"AFFILIATE_COMMUNITY_INCOMPLETE",
			"community message and QR code are required before enabling",
		)
	}
	settings, err := s.repo.UpdateCommunitySettings(
		ctx,
		title,
		message,
		enabled,
		updatedBy,
		expectedRevision,
	)
	if err != nil {
		return nil, err
	}
	normalizeAffiliateCommunity(settings)
	return settings, nil
}

func (s *AffiliateCommunityService) UploadQRCode(
	ctx context.Context,
	updatedBy int64,
	upload AffiliateCommunityQRCodeUpload,
) (*AffiliateCommunitySettings, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate community repository is not configured")
	}
	if updatedBy <= 0 {
		return nil, ErrInvalidInput
	}
	if upload.Body == nil || upload.Size <= 0 {
		return nil, infraerrors.BadRequest("COMMUNITY_QR_REQUIRED", "community QR image is required")
	}
	if upload.Size > agentPaymentQRCodeMaxSize {
		return nil, infraerrors.BadRequest("COMMUNITY_QR_TOO_LARGE", "community QR image must be at most 5MB")
	}
	ext, ok := agentPaymentQRExt(upload.Filename, upload.ContentType)
	if !ok {
		return nil, infraerrors.BadRequest("COMMUNITY_QR_UNSUPPORTED", "only jpg/png/webp images are supported")
	}
	token, err := randomPaymentHex(12)
	if err != nil {
		return nil, fmt.Errorf("generate community QR filename: %w", err)
	}
	objectKey := filepath.ToSlash(filepath.Join(
		"affiliate-community",
		time.Now().UTC().Format("20060102T150405Z")+"_"+strconv.FormatInt(updatedBy, 10)+"_"+token+ext,
	))
	path, err := affiliateCommunityObjectPath(objectKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create community QR directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create community QR file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(upload.Body, upload.Size+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("save community QR file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close community QR file: %w", closeErr)
	}
	if written != upload.Size {
		_ = os.Remove(path)
		return nil, infraerrors.BadRequest("COMMUNITY_QR_INVALID_SIZE", "community QR file size does not match upload size")
	}
	settings, err := s.repo.UpdateCommunityQRCode(
		ctx,
		objectKey,
		upload.ContentType,
		filepath.Base(upload.Filename),
		upload.Size,
		updatedBy,
	)
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	normalizeAffiliateCommunity(settings)
	return settings, nil
}

func (s *AffiliateCommunityService) GetQRCodeFile(
	ctx context.Context,
	agentView bool,
) (*AffiliateCommunityQRCodeFile, error) {
	settings, err := s.Get(ctx, agentView)
	if err != nil {
		return nil, err
	}
	if (agentView && !settings.Enabled) || !settings.HasQRCode {
		return nil, ErrAffiliateCommunityNotAvailable
	}
	path, err := affiliateCommunityObjectPath(settings.QRObjectKey)
	if err != nil {
		return nil, err
	}
	return &AffiliateCommunityQRCodeFile{
		Path:        path,
		ContentType: settings.QRContentType,
		Filename:    settings.QROriginalFilename,
	}, nil
}

func normalizeAffiliateCommunity(settings *AffiliateCommunitySettings) {
	if settings == nil {
		return
	}
	settings.Title = strings.TrimSpace(settings.Title)
	settings.Message = strings.TrimSpace(settings.Message)
	settings.QRObjectKey = strings.TrimSpace(settings.QRObjectKey)
	settings.QRContentType = strings.TrimSpace(settings.QRContentType)
	settings.QROriginalFilename = strings.TrimSpace(settings.QROriginalFilename)
	settings.HasQRCode = settings.QRObjectKey != ""
}

func affiliateCommunityObjectPath(objectKey string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(objectKey))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", infraerrors.BadRequest("INVALID_COMMUNITY_QR_PATH", "invalid community QR path")
	}
	return filepath.Join(agentPaymentDataDir(), "uploads", clean), nil
}
