package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
)

const (
	ccSwitchToolID                = "cc-switch"
	codexToolID                   = "codex"
	codexPlusPlusToolID           = "codex-plus-plus"
	claudeDesktopToolID           = "claude-desktop"
	defaultCCSwitchRepo           = "farion1231/cc-switch"
	defaultCodexRepo              = "openai/codex"
	defaultCodexWindowsMirrorRepo = "Wangnov/codex-app-mirror"
	defaultCodexMacOfficialURL    = "https://persistent.oaistatic.com/codex-app-prod/ChatGPT.dmg"
	defaultCodexPPRepo            = "BigPizzaV3/CodexPlusPlus"
	defaultClaudeCodeRepo         = "anthropics/claude-code"
	defaultClaudeMacURL           = "https://storage.googleapis.com/osprey-downloads-c02f6a0d-347c-492b-a752-3e0651722e97/nest/Claude.dmg"
	defaultClaudeWinURL           = "https://downloads.claude.ai/releases/win32/x64/1.25927.0/Claude-003700efafbc2ccb4b1177a5e637b14da381799e.exe"
	defaultClaudeARMURL           = "https://downloads.claude.ai/releases/win32/arm64/1.25927.0/Claude-003700efafbc2ccb4b1177a5e637b14da381799e.exe"
)

var (
	ErrDownloadManifestNotReady = errors.New("download manifest is not ready")
	ErrDownloadAssetNotFound    = errors.New("download asset not found")
	ErrDownloadToolNotFound     = errors.New("download tool not found")
	ErrDownloadTokenInvalid     = errors.New("download token is invalid or expired")
	assetIDUnsafeChars          = regexp.MustCompile(`[^a-z0-9._-]+`)
)

type staticDownloadSource struct {
	Name     string
	URL      string
	Platform string
	Arch     string
}

type CachedDownloadManifest struct {
	Tool        string                `json:"tool"`
	Repo        string                `json:"repo"`
	Version     string                `json:"version"`
	ReleaseName string                `json:"release_name"`
	PublishedAt string                `json:"published_at"`
	UpdatedAt   string                `json:"updated_at"`
	Assets      []CachedDownloadAsset `json:"assets"`
}

// DownloadVersionStatus keeps the download page honest about where a package
// comes from. Some tools are cached by us, while Claude Code currently uses
// Anthropic's official installer directly.
type DownloadVersionStatus struct {
	Tool                string `json:"tool"`
	Name                string `json:"name"`
	CachedVersion       string `json:"cached_version"`
	CachedUpdatedAt     string `json:"cached_updated_at"`
	OfficialVersion     string `json:"official_version"`
	OfficialPublishedAt string `json:"official_published_at"`
	OfficialURL         string `json:"official_url"`
	CacheMode           string `json:"cache_mode"`
	State               string `json:"state"`
	Note                string `json:"note"`
}

type CachedDownloadAsset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Path     string `json:"-"`
}

type DownloadAssetFile struct {
	Asset CachedDownloadAsset
	Path  string
}

type assetDownloadToken struct {
	ToolID    string
	AssetID   string
	ExpiresAt time.Time
}

type DownloadResourceService struct {
	cfg          config.DownloadsConfig
	githubClient GitHubReleaseClient
	cacheDir     string
	ctx          context.Context
	cancel       context.CancelFunc

	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	started atomic.Bool
	stopped atomic.Bool

	tokenMu        sync.Mutex
	downloadTokens map[string]assetDownloadToken
}

func NewDownloadResourceService(cfg *config.Config, githubClient GitHubReleaseClient) *DownloadResourceService {
	downloadCfg := config.DownloadsConfig{
		Enabled:                      true,
		CacheDir:                     "./data/downloads",
		UpdateIntervalHours:          24,
		StartupSync:                  true,
		CCSwitchRepo:                 defaultCCSwitchRepo,
		CodexRepo:                    defaultCodexRepo,
		CodexWindowsMirrorRepo:       defaultCodexWindowsMirrorRepo,
		CodexMacOfficialURL:          defaultCodexMacOfficialURL,
		CodexPlusPlusRepo:            defaultCodexPPRepo,
		ClaudeDesktopMacURL:          defaultClaudeMacURL,
		ClaudeDesktopWindowsX64URL:   defaultClaudeWinURL,
		ClaudeDesktopWindowsARM64URL: defaultClaudeARMURL,
		MaxAssetBytes:                1024 * 1024 * 1024,
	}
	if cfg != nil {
		downloadCfg = cfg.Downloads
	}
	if strings.TrimSpace(downloadCfg.CacheDir) == "" {
		downloadCfg.CacheDir = "./data/downloads"
	}
	if strings.TrimSpace(downloadCfg.CCSwitchRepo) == "" {
		downloadCfg.CCSwitchRepo = defaultCCSwitchRepo
	}
	if strings.TrimSpace(downloadCfg.CodexRepo) == "" {
		downloadCfg.CodexRepo = defaultCodexRepo
	}
	if strings.TrimSpace(downloadCfg.CodexWindowsMirrorRepo) == "" {
		downloadCfg.CodexWindowsMirrorRepo = defaultCodexWindowsMirrorRepo
	}
	if strings.TrimSpace(downloadCfg.CodexMacOfficialURL) == "" {
		downloadCfg.CodexMacOfficialURL = defaultCodexMacOfficialURL
	}
	if strings.TrimSpace(downloadCfg.CodexPlusPlusRepo) == "" {
		downloadCfg.CodexPlusPlusRepo = defaultCodexPPRepo
	}
	if strings.TrimSpace(downloadCfg.ClaudeDesktopMacURL) == "" {
		downloadCfg.ClaudeDesktopMacURL = defaultClaudeMacURL
	}
	if strings.TrimSpace(downloadCfg.ClaudeDesktopWindowsX64URL) == "" {
		downloadCfg.ClaudeDesktopWindowsX64URL = defaultClaudeWinURL
	}
	if strings.TrimSpace(downloadCfg.ClaudeDesktopWindowsARM64URL) == "" {
		downloadCfg.ClaudeDesktopWindowsARM64URL = defaultClaudeARMURL
	}
	if downloadCfg.UpdateIntervalHours <= 0 {
		downloadCfg.UpdateIntervalHours = 24
	}
	if downloadCfg.MaxAssetBytes <= 0 {
		downloadCfg.MaxAssetBytes = 1024 * 1024 * 1024
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &DownloadResourceService{
		cfg:            downloadCfg,
		githubClient:   githubClient,
		cacheDir:       filepath.Clean(downloadCfg.CacheDir),
		ctx:            ctx,
		cancel:         cancel,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
		downloadTokens: make(map[string]assetDownloadToken),
	}
}

func (s *DownloadResourceService) Start() {
	if s == nil || !s.cfg.Enabled || !s.started.CompareAndSwap(false, true) {
		return
	}
	go s.loop()
}

func (s *DownloadResourceService) Stop() {
	if s == nil || !s.started.Load() || !s.stopped.CompareAndSwap(false, true) {
		return
	}
	s.cancel()
	close(s.stopCh)
	<-s.doneCh
}

func (s *DownloadResourceService) loop() {
	defer close(s.doneCh)

	if s.cfg.StartupSync {
		s.syncWithTimeout("startup")
	}

	ticker := time.NewTicker(time.Duration(s.cfg.UpdateIntervalHours) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.syncWithTimeout("scheduled")
		case <-s.stopCh:
			return
		}
	}
}

func (s *DownloadResourceService) syncWithTimeout(reason string) {
	for _, syncFn := range []struct {
		tool string
		fn   func(context.Context) error
	}{
		{tool: ccSwitchToolID, fn: s.SyncCCSwitch},
		{tool: codexToolID, fn: s.SyncCodex},
		{tool: codexPlusPlusToolID, fn: s.SyncCodexPlusPlus},
		{tool: claudeDesktopToolID, fn: s.SyncClaudeDesktop},
	} {
		// Large desktop packages can approach 1 GB. Give every tool its own
		// deadline so one slow source cannot consume the whole update window and
		// starve the remaining caches.
		ctx, cancel := context.WithTimeout(s.ctx, 45*time.Minute)
		if err := syncFn.fn(ctx); err != nil {
			slog.Warn("download resource sync failed", "tool", syncFn.tool, "reason", reason, "error", err)
		}
		cancel()
	}
}

func (s *DownloadResourceService) SyncCCSwitch(ctx context.Context) error {
	return s.syncGitHubRelease(ctx, ccSwitchToolID, s.cfg.CCSwitchRepo, isCCSwitchInstallAsset)
}

func (s *DownloadResourceService) SyncCodex(ctx context.Context) error {
	if s == nil {
		return errors.New("nil download resource service")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	officialRelease, err := s.githubClient.FetchLatestRelease(ctx, s.cfg.CodexRepo)
	if err != nil {
		return fmt.Errorf("fetch latest %s release: %w", codexToolID, err)
	}
	desktopRelease, err := s.githubClient.FetchLatestRelease(ctx, s.cfg.CodexWindowsMirrorRepo)
	if err != nil {
		return fmt.Errorf("fetch latest %s desktop mirror release: %w", codexToolID, err)
	}
	if strings.TrimSpace(officialRelease.TagName) == "" || strings.TrimSpace(desktopRelease.TagName) == "" {
		return errors.New("latest codex release has empty tag")
	}

	// OpenAI is authoritative for the CLI and macOS desktop installer. The
	// configured mirror is used only for Windows MSIX packages that OpenAI does
	// not publish through the public Codex release repository.
	version := desktopRelease.TagName
	versionDir := filepath.Join(s.cacheDir, codexToolID, sanitizePathSegment(version))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	assets := make([]CachedDownloadAsset, 0, len(officialRelease.Assets)+len(desktopRelease.Assets)+1)
	for _, source := range []struct {
		release *GitHubRelease
		include func(string) bool
	}{
		{release: officialRelease, include: isCodexInstallAsset},
		{release: desktopRelease, include: isCodexWindowsDesktopAsset},
	} {
		for _, asset := range source.release.Assets {
			if !source.include(asset.Name) {
				continue
			}
			cached, err := s.cacheGitHubAsset(ctx, codexToolID, versionDir, asset)
			if err != nil {
				return err
			}
			if cached != nil {
				assets = append(assets, *cached)
			}
		}
	}
	if macURL := strings.TrimSpace(s.cfg.CodexMacOfficialURL); macURL != "" {
		name := "ChatGPT.dmg"
		dest := filepath.Join(versionDir, name)
		if err := s.ensureStaticAsset(ctx, macURL, dest); err != nil {
			return fmt.Errorf("cache official macOS Codex app: %w", err)
		}
		info, err := os.Stat(dest)
		if err != nil {
			return fmt.Errorf("stat official macOS Codex app: %w", err)
		}
		sum, err := fileSHA256(dest)
		if err != nil {
			return fmt.Errorf("checksum official macOS Codex app: %w", err)
		}
		assets = append(assets, CachedDownloadAsset{
			ID: makeAssetID(name), Name: name, Size: info.Size(), SHA256: sum,
			Platform: "macos", Arch: "universal", Path: dest,
		})
	}
	if len(assets) == 0 {
		return errors.New("latest codex releases have no downloadable installer assets")
	}

	manifest := CachedDownloadManifest{
		Tool:        codexToolID,
		Repo:        s.cfg.CodexRepo + ", " + s.cfg.CodexWindowsMirrorRepo + ", persistent.oaistatic.com",
		Version:     version,
		ReleaseName: desktopRelease.Name,
		PublishedAt: desktopRelease.PublishedAt,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assets:      assets,
	}
	if err := s.writeManifest(manifest); err != nil {
		return err
	}
	s.cleanupOldVersions(codexToolID, version)
	s.cleanupUnreferencedAssets(versionDir, assets)
	slog.Info("download resource synced", "tool", codexToolID, "version", version, "assets", len(assets))
	return nil
}

func (s *DownloadResourceService) SyncCodexPlusPlus(ctx context.Context) error {
	return s.syncGitHubRelease(ctx, codexPlusPlusToolID, s.cfg.CodexPlusPlusRepo, isCodexPlusPlusInstallAsset)
}

func (s *DownloadResourceService) syncGitHubRelease(ctx context.Context, toolID, repo string, include func(string) bool) error {
	if s == nil {
		return errors.New("nil download resource service")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	release, err := s.githubClient.FetchLatestRelease(ctx, repo)
	if err != nil {
		return fmt.Errorf("fetch latest %s release: %w", toolID, err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return fmt.Errorf("latest %s release has empty tag", toolID)
	}

	versionDir := filepath.Join(s.cacheDir, toolID, sanitizePathSegment(release.TagName))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	assets := make([]CachedDownloadAsset, 0, len(release.Assets))
	for _, asset := range release.Assets {
		if !include(asset.Name) {
			continue
		}
		if asset.Size <= 0 || asset.Size > s.cfg.MaxAssetBytes {
			slog.Warn("skip download asset with invalid size", "tool", toolID, "asset", asset.Name, "size", asset.Size)
			continue
		}

		cached, err := s.cacheGitHubAsset(ctx, toolID, versionDir, asset)
		if err != nil {
			return err
		}
		if cached != nil {
			assets = append(assets, *cached)
		}
	}
	if len(assets) == 0 {
		return fmt.Errorf("latest %s release has no downloadable installer assets", toolID)
	}

	manifest := CachedDownloadManifest{
		Tool:        toolID,
		Repo:        repo,
		Version:     release.TagName,
		ReleaseName: release.Name,
		PublishedAt: release.PublishedAt,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assets:      assets,
	}
	if err := s.writeManifest(manifest); err != nil {
		return err
	}
	s.cleanupOldVersions(toolID, release.TagName)
	slog.Info("download resource synced", "tool", toolID, "version", release.TagName, "assets", len(assets))
	return nil
}

func (s *DownloadResourceService) cacheGitHubAsset(ctx context.Context, toolID, versionDir string, asset GitHubAsset) (*CachedDownloadAsset, error) {
	if asset.Size <= 0 || asset.Size > s.cfg.MaxAssetBytes {
		slog.Warn("skip download asset with invalid size", "tool", toolID, "asset", asset.Name, "size", asset.Size)
		return nil, nil
	}
	dest := filepath.Join(versionDir, filepath.Base(asset.Name))
	if err := s.ensureAsset(ctx, asset, dest); err != nil {
		return nil, fmt.Errorf("cache asset %s: %w", asset.Name, err)
	}
	sum, err := fileSHA256(dest)
	if err != nil {
		return nil, fmt.Errorf("checksum asset %s: %w", asset.Name, err)
	}
	if digest := strings.TrimSpace(asset.Digest); digest != "" {
		const sha256Prefix = "sha256:"
		if !strings.HasPrefix(strings.ToLower(digest), sha256Prefix) {
			return nil, fmt.Errorf("asset %s has unsupported digest: %s", asset.Name, digest)
		}
		expected := strings.TrimSpace(digest[len(sha256Prefix):])
		if len(expected) != sha256.Size*2 || !strings.EqualFold(sum, expected) {
			_ = os.Remove(dest)
			return nil, fmt.Errorf("asset %s checksum mismatch", asset.Name)
		}
	}
	return &CachedDownloadAsset{
		ID:       makeAssetID(asset.Name),
		Name:     asset.Name,
		Size:     asset.Size,
		SHA256:   sum,
		Platform: classifyPlatform(asset.Name),
		Arch:     classifyArch(asset.Name),
		Path:     dest,
	}, nil
}

func (s *DownloadResourceService) SyncClaudeDesktop(ctx context.Context) error {
	if s == nil {
		return errors.New("nil download resource service")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	sources := []staticDownloadSource{
		{Name: "Claude.dmg", URL: s.cfg.ClaudeDesktopMacURL, Platform: "macos", Arch: "universal"},
		{Name: "Claude-Setup-x64.exe", URL: s.cfg.ClaudeDesktopWindowsX64URL, Platform: "windows", Arch: "x64"},
		{Name: "Claude-Setup-arm64.exe", URL: s.cfg.ClaudeDesktopWindowsARM64URL, Platform: "windows", Arch: "arm64"},
	}

	version := "latest"
	versionDir := filepath.Join(s.cacheDir, claudeDesktopToolID, version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	assets := make([]CachedDownloadAsset, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.URL) == "" {
			continue
		}
		dest := filepath.Join(versionDir, filepath.Base(source.Name))
		if err := s.downloadStaticAsset(ctx, source.URL, dest); err != nil {
			return fmt.Errorf("cache asset %s: %w", source.Name, err)
		}
		info, err := os.Stat(dest)
		if err != nil {
			return fmt.Errorf("stat asset %s: %w", source.Name, err)
		}
		sum, err := fileSHA256(dest)
		if err != nil {
			return fmt.Errorf("checksum asset %s: %w", source.Name, err)
		}
		assets = append(assets, CachedDownloadAsset{
			ID:       makeAssetID(source.Name),
			Name:     source.Name,
			Size:     info.Size(),
			SHA256:   sum,
			Platform: source.Platform,
			Arch:     source.Arch,
			Path:     dest,
		})
	}
	if len(assets) == 0 {
		return errors.New("claude desktop has no configured downloadable assets")
	}

	manifest := CachedDownloadManifest{
		Tool:        claudeDesktopToolID,
		Repo:        "claude.com/download",
		Version:     version,
		ReleaseName: "Claude Desktop",
		PublishedAt: "",
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assets:      assets,
	}
	if err := s.writeManifest(manifest); err != nil {
		return err
	}
	s.cleanupOldVersions(claudeDesktopToolID, version)
	slog.Info("download resource synced", "tool", claudeDesktopToolID, "version", version, "assets", len(assets))
	return nil
}

func (s *DownloadResourceService) ensureAsset(ctx context.Context, asset GitHubAsset, dest string) error {
	if info, err := os.Stat(dest); err == nil && info.Size() == asset.Size {
		return nil
	}
	tmp := dest + ".tmp"
	_ = os.Remove(tmp)
	if err := s.githubClient.DownloadFile(ctx, asset.BrowserDownloadURL, tmp, s.cfg.MaxAssetBytes); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if info, err := os.Stat(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	} else if info.Size() != asset.Size {
		_ = os.Remove(tmp)
		return fmt.Errorf("downloaded size mismatch: got %d want %d", info.Size(), asset.Size)
	}
	return os.Rename(tmp, dest)
}

func (s *DownloadResourceService) downloadStaticAsset(ctx context.Context, url, dest string) error {
	tmp := dest + ".tmp"
	_ = os.Remove(tmp)
	if err := s.githubClient.DownloadFile(ctx, url, tmp, s.cfg.MaxAssetBytes); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func (s *DownloadResourceService) ensureStaticAsset(ctx context.Context, url, dest string) error {
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		return nil
	}
	return s.downloadStaticAsset(ctx, url, dest)
}

func (s *DownloadResourceService) ListCCSwitch(ctx context.Context) (*CachedDownloadManifest, error) {
	return s.ListTool(ctx, ccSwitchToolID)
}

func (s *DownloadResourceService) GetCCSwitchAsset(ctx context.Context, assetID string) (*DownloadAssetFile, error) {
	return s.GetToolAsset(ctx, ccSwitchToolID, assetID)
}

func (s *DownloadResourceService) ListTool(ctx context.Context, toolID string) (*CachedDownloadManifest, error) {
	_ = ctx
	toolID, ok := normalizeDownloadToolID(toolID)
	if !ok {
		return nil, ErrDownloadToolNotFound
	}
	return s.readManifest(toolID)
}

// ListVersionStatus compares the local download cache with each authoritative
// upstream release. A failure for one upstream never hides the other tools.
func (s *DownloadResourceService) ListVersionStatus(ctx context.Context) []DownloadVersionStatus {
	items := []struct {
		tool        string
		name        string
		manifest    string
		repo        string
		officialURL string
		cacheMode   string
		note        string
		comparable  bool
	}{
		{
			tool:        "codex",
			name:        "Codex",
			manifest:    codexToolID,
			repo:        s.cfg.CodexRepo,
			officialURL: "https://github.com/openai/codex/releases/latest",
			cacheMode:   "cached",
			note:        "macOS 缓存 OpenAI 官方安装包；Windows 缓存第三方镜像的 MSIX，并通过本站 AppInstaller 提供更新。",
			comparable:  false,
		},
		{
			tool:        "claude-code",
			name:        "Claude Code",
			repo:        defaultClaudeCodeRepo,
			officialURL: "https://github.com/anthropics/claude-code/releases/latest",
			cacheMode:   "npm-mirror",
			note:        "一键安装优先使用国内 npm 镜像，失败后才回退官方 npm；不走 Anthropic 安装器直连。",
			comparable:  false,
		},
		{
			tool:        "cc-switch",
			name:        "CC Switch",
			manifest:    ccSwitchToolID,
			repo:        s.cfg.CCSwitchRepo,
			officialURL: "https://github.com/farion1231/cc-switch/releases/latest",
			cacheMode:   "cached",
			note:        "本站定时同步官方 Release，用户下载时不需要访问 GitHub。",
			comparable:  true,
		},
	}

	result := make([]DownloadVersionStatus, 0, len(items))
	for _, item := range items {
		status := DownloadVersionStatus{
			Tool:        item.tool,
			Name:        item.name,
			OfficialURL: item.officialURL,
			CacheMode:   item.cacheMode,
			State:       "unknown",
			Note:        item.note,
		}
		if item.manifest != "" {
			if manifest, err := s.readManifest(item.manifest); err == nil {
				status.CachedVersion = manifest.Version
				status.CachedUpdatedAt = manifest.UpdatedAt
			}
		}

		release, err := s.githubClient.FetchLatestRelease(ctx, item.repo)
		if err == nil && release != nil {
			status.OfficialVersion = release.TagName
			status.OfficialPublishedAt = release.PublishedAt
		}

		switch {
		case item.cacheMode == "npm-mirror" && status.OfficialVersion != "":
			status.State = "npm-mirror"
		case status.CachedVersion == "":
			status.State = "cache-missing"
		case status.OfficialVersion == "":
			status.State = "official-unavailable"
		case !item.comparable:
			status.State = "cached"
		case versionsEqual(status.CachedVersion, status.OfficialVersion):
			status.State = "current"
		default:
			status.State = "update-available"
		}
		result = append(result, status)
	}
	return result
}

func versionsEqual(left, right string) bool {
	normalize := func(value string) string {
		return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "v")
	}
	return normalize(left) != "" && normalize(left) == normalize(right)
}

func (s *DownloadResourceService) GetToolAsset(ctx context.Context, toolID, assetID string) (*DownloadAssetFile, error) {
	_ = ctx
	toolID, ok := normalizeDownloadToolID(toolID)
	if !ok {
		return nil, ErrDownloadToolNotFound
	}
	manifest, err := s.readManifest(toolID)
	if err != nil {
		return nil, err
	}
	for _, asset := range manifest.Assets {
		if asset.ID != assetID {
			continue
		}
		if asset.Path == "" {
			asset.Path = filepath.Join(s.cacheDir, toolID, sanitizePathSegment(manifest.Version), filepath.Base(asset.Name))
		}
		if !isPathWithin(filepath.Join(s.cacheDir, toolID), asset.Path) {
			return nil, errors.New("cached asset path escapes cache dir")
		}
		if _, err := os.Stat(asset.Path); err != nil {
			return nil, fmt.Errorf("cached asset missing: %w", err)
		}
		return &DownloadAssetFile{Asset: asset, Path: asset.Path}, nil
	}
	return nil, ErrDownloadAssetNotFound
}

func (s *DownloadResourceService) GetCodexWindowsDesktopAsset(ctx context.Context) (*DownloadAssetFile, error) {
	manifest, err := s.ListTool(ctx, codexToolID)
	if err != nil {
		return nil, err
	}
	for _, asset := range manifest.Assets {
		if asset.Platform == "windows" && asset.Arch == "x64" && isCodexWindowsDesktopAsset(asset.Name) {
			return s.GetToolAsset(ctx, codexToolID, asset.ID)
		}
	}
	return nil, ErrDownloadAssetNotFound
}

func (s *DownloadResourceService) CreateToolAssetDownloadToken(ctx context.Context, toolID, assetID string, ttl time.Duration) (string, time.Time, error) {
	if s == nil {
		return "", time.Time{}, errors.New("nil download resource service")
	}
	toolID, ok := normalizeDownloadToolID(toolID)
	if !ok {
		return "", time.Time{}, ErrDownloadToolNotFound
	}
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return "", time.Time{}, ErrDownloadAssetNotFound
	}
	if _, err := s.GetToolAsset(ctx, toolID, assetID); err != nil {
		return "", time.Time{}, err
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	token, err := randomDownloadToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().UTC().Add(ttl)

	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.downloadTokens == nil {
		s.downloadTokens = make(map[string]assetDownloadToken)
	}
	s.cleanupExpiredDownloadTokensLocked(time.Now().UTC())
	s.downloadTokens[token] = assetDownloadToken{
		ToolID:    toolID,
		AssetID:   assetID,
		ExpiresAt: expiresAt,
	}
	return token, expiresAt, nil
}

func (s *DownloadResourceService) GetToolAssetByDownloadToken(ctx context.Context, token string) (*DownloadAssetFile, error) {
	if s == nil {
		return nil, errors.New("nil download resource service")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrDownloadTokenInvalid
	}

	now := time.Now().UTC()
	s.tokenMu.Lock()
	entry, ok := s.downloadTokens[token]
	if !ok || !entry.ExpiresAt.After(now) {
		delete(s.downloadTokens, token)
		s.tokenMu.Unlock()
		return nil, ErrDownloadTokenInvalid
	}
	s.cleanupExpiredDownloadTokensLocked(now)
	s.tokenMu.Unlock()

	return s.GetToolAsset(ctx, entry.ToolID, entry.AssetID)
}

func (s *DownloadResourceService) cleanupExpiredDownloadTokensLocked(now time.Time) {
	for token, entry := range s.downloadTokens {
		if !entry.ExpiresAt.After(now) {
			delete(s.downloadTokens, token)
		}
	}
}

func (s *DownloadResourceService) manifestPath(toolID string) string {
	return filepath.Join(s.cacheDir, toolID, "manifest.json")
}

func (s *DownloadResourceService) readManifest(toolID string) (*CachedDownloadManifest, error) {
	raw, err := os.ReadFile(s.manifestPath(toolID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDownloadManifestNotReady
		}
		return nil, err
	}
	var manifest CachedDownloadManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	for i := range manifest.Assets {
		if manifest.Assets[i].Path == "" {
			manifest.Assets[i].Path = filepath.Join(s.cacheDir, toolID, sanitizePathSegment(manifest.Version), filepath.Base(manifest.Assets[i].Name))
		}
	}
	return &manifest, nil
}

func (s *DownloadResourceService) writeManifest(manifest CachedDownloadManifest) error {
	toolID, ok := normalizeDownloadToolID(manifest.Tool)
	if !ok {
		return ErrDownloadToolNotFound
	}
	dir := filepath.Join(s.cacheDir, toolID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.manifestPath(toolID) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.manifestPath(toolID))
}

func (s *DownloadResourceService) cleanupOldVersions(toolID, currentTag string) {
	root := filepath.Join(s.cacheDir, toolID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	current := sanitizePathSegment(currentTag)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == current {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}

func (s *DownloadResourceService) cleanupUnreferencedAssets(versionDir string, assets []CachedDownloadAsset) {
	keep := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		keep[filepath.Base(asset.Name)] = struct{}{}
	}
	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		if _, ok := keep[entry.Name()]; !ok {
			_ = os.Remove(filepath.Join(versionDir, entry.Name()))
		}
	}
}

func isCCSwitchInstallAsset(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".sig") || lower == "latest.json" {
		return false
	}
	return strings.HasSuffix(lower, ".msi") ||
		strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".dmg") ||
		strings.HasSuffix(lower, ".deb") ||
		strings.HasSuffix(lower, ".rpm") ||
		strings.HasSuffix(lower, ".appimage") ||
		strings.HasSuffix(lower, ".tar.gz")
}

func isCodexInstallAsset(name string) bool {
	lower := strings.ToLower(name)
	if !strings.HasPrefix(lower, "codex-") ||
		strings.HasPrefix(lower, "codex-app-server-") ||
		strings.HasPrefix(lower, "codex-package-") ||
		strings.HasPrefix(lower, "codex-npm-") ||
		strings.HasPrefix(lower, "codex-symbols-") ||
		strings.HasPrefix(lower, "codex-code-mode-") ||
		strings.HasPrefix(lower, "codex-command-") ||
		strings.HasPrefix(lower, "codex-responses-") ||
		strings.HasPrefix(lower, "codex-windows-") {
		return false
	}
	return (strings.HasSuffix(lower, "apple-darwin.tar.gz") &&
		(strings.Contains(lower, "aarch64") || strings.Contains(lower, "x86_64"))) ||
		(strings.HasSuffix(lower, "pc-windows-msvc.exe.zip") && strings.Contains(lower, "x86_64"))
}

func isCodexWindowsDesktopAsset(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".msix") &&
		(strings.Contains(lower, "_x64__") || strings.Contains(lower, "_arm64__"))
}

func isCodexPlusPlusInstallAsset(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".sig") || lower == "latest.json" {
		return false
	}
	if !strings.HasPrefix(lower, "codexplusplus-") {
		return false
	}
	return strings.HasSuffix(lower, "-windows-x64-setup.exe") ||
		strings.HasSuffix(lower, "-macos-x64.dmg") ||
		strings.HasSuffix(lower, "-macos-arm64.dmg")
}

func classifyPlatform(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "windows") || strings.Contains(lower, "win32") || strings.Contains(lower, "pc-windows") || strings.HasSuffix(lower, ".msix"):
		return "windows"
	case strings.Contains(lower, "macos") || strings.Contains(lower, "darwin") || strings.HasSuffix(lower, ".dmg"):
		return "macos"
	case strings.Contains(lower, "linux"):
		return "linux"
	default:
		return "other"
	}
}

func classifyArch(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64"):
		return "arm64"
	case strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "x64"):
		return "x64"
	default:
		return "universal"
	}
}

func normalizeDownloadToolID(toolID string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(toolID)) {
	case ccSwitchToolID:
		return ccSwitchToolID, true
	case codexToolID:
		return codexToolID, true
	case codexPlusPlusToolID:
		return codexPlusPlusToolID, true
	case claudeDesktopToolID:
		return claudeDesktopToolID, true
	default:
		return "", false
	}
}

func makeAssetID(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = assetIDUnsafeChars.ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")
	if id == "" {
		return "asset"
	}
	return id
}

func sanitizePathSegment(v string) string {
	clean := makeAssetID(v)
	if clean == "." || clean == ".." || clean == "" {
		return "unknown"
	}
	return clean
}

func randomDownloadToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate download token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isPathWithin(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}
