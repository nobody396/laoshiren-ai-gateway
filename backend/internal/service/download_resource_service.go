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
	gitForWindowsToolID           = "git-for-windows"
	grokBuildToolID               = "grok-build"
	defaultCCSwitchRepo           = "farion1231/cc-switch"
	defaultCodexRepo              = "openai/codex"
	defaultCodexWindowsMirrorRepo = "Wangnov/codex-app-mirror"
	defaultCodexPPRepo            = "BigPizzaV3/CodexPlusPlus"
	defaultGitForWindowsRepo      = "git-for-windows/git"
	defaultGrokBuildPrimaryBase   = "https://x.ai/cli"
	defaultGrokBuildFallbackBase  = "https://storage.googleapis.com/grok-build-public-artifacts/cli"
	defaultClaudeCodeRepo         = "anthropics/claude-code"
	defaultClaudeMacURL           = "https://storage.googleapis.com/osprey-downloads-c02f6a0d-347c-492b-a752-3e0651722e97/nest/Claude.dmg"
	defaultClaudeLatestBaseURL    = "https://downloads.claude.ai/releases/win32"
	versionManifestName           = ".manifest.json"
)

var (
	ErrDownloadManifestNotReady = errors.New("download manifest is not ready")
	ErrDownloadAssetNotFound    = errors.New("download asset not found")
	ErrDownloadToolNotFound     = errors.New("download tool not found")
	ErrDownloadTokenInvalid     = errors.New("download token is invalid or expired")
	assetIDUnsafeChars          = regexp.MustCompile(`[^a-z0-9._-]+`)
	sha256Pattern               = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	grokBuildVersionPattern     = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9._-]+)?$`)
	gitForWindowsAssetPattern   = regexp.MustCompile(`^git-[0-9].*-(64-bit|arm64)\.exe$`)
)

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
// comes from. Desktop/native packages are cached by us, while npm clients use
// the configured registry mirror.
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
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	SHA256             string `json:"sha256"`
	Platform           string `json:"platform"`
	Arch               string `json:"arch"`
	Role               string `json:"role,omitempty"`
	ComponentVersion   string `json:"component_version,omitempty"`
	UpstreamSHA256     string `json:"upstream_sha256,omitempty"`
	UpstreamCompressed int64  `json:"upstream_compressed_size,omitempty"`
	Path               string `json:"-"`
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
		Enabled:                     true,
		CacheDir:                    "./data/downloads",
		VersionCheckIntervalMinutes: 30,
		ClaudeDesktopCheckMinutes:   5,
		StartupSync:                 true,
		CCSwitchRepo:                defaultCCSwitchRepo,
		CodexRepo:                   defaultCodexRepo,
		CodexWindowsMirrorRepo:      defaultCodexWindowsMirrorRepo,
		CodexPlusPlusRepo:           defaultCodexPPRepo,
		GitForWindowsRepo:           defaultGitForWindowsRepo,
		GrokBuildPrimaryBaseURL:     defaultGrokBuildPrimaryBase,
		GrokBuildFallbackBaseURL:    defaultGrokBuildFallbackBase,
		ClaudeDesktopMacURL:         defaultClaudeMacURL,
		ClaudeDesktopLatestBaseURL:  defaultClaudeLatestBaseURL,
		MaxAssetBytes:               1024 * 1024 * 1024,
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
	if strings.TrimSpace(downloadCfg.CodexPlusPlusRepo) == "" {
		downloadCfg.CodexPlusPlusRepo = defaultCodexPPRepo
	}
	if strings.TrimSpace(downloadCfg.GitForWindowsRepo) == "" {
		downloadCfg.GitForWindowsRepo = defaultGitForWindowsRepo
	}
	if strings.TrimSpace(downloadCfg.GrokBuildPrimaryBaseURL) == "" {
		downloadCfg.GrokBuildPrimaryBaseURL = defaultGrokBuildPrimaryBase
	}
	if strings.TrimSpace(downloadCfg.GrokBuildFallbackBaseURL) == "" {
		downloadCfg.GrokBuildFallbackBaseURL = defaultGrokBuildFallbackBase
	}
	if strings.TrimSpace(downloadCfg.ClaudeDesktopMacURL) == "" {
		downloadCfg.ClaudeDesktopMacURL = defaultClaudeMacURL
	}
	if strings.TrimSpace(downloadCfg.ClaudeDesktopLatestBaseURL) == "" {
		downloadCfg.ClaudeDesktopLatestBaseURL = defaultClaudeLatestBaseURL
	}
	if downloadCfg.VersionCheckIntervalMinutes <= 0 {
		downloadCfg.VersionCheckIntervalMinutes = 30
	}
	if downloadCfg.ClaudeDesktopCheckMinutes <= 0 {
		downloadCfg.ClaudeDesktopCheckMinutes = 5
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
		s.syncClaudeWithTimeout("startup")
	}

	ticker := time.NewTicker(time.Duration(s.cfg.VersionCheckIntervalMinutes) * time.Minute)
	defer ticker.Stop()
	claudeTicker := time.NewTicker(time.Duration(s.cfg.ClaudeDesktopCheckMinutes) * time.Minute)
	defer claudeTicker.Stop()

	for {
		select {
		case <-ticker.C:
			s.syncWithTimeout("scheduled")
		case <-claudeTicker.C:
			s.syncClaudeWithTimeout("version-check")
		case <-s.stopCh:
			return
		}
	}
}

func (s *DownloadResourceService) syncClaudeWithTimeout(reason string) {
	ctx, cancel := context.WithTimeout(s.ctx, 45*time.Minute)
	defer cancel()
	if err := s.SyncClaudeDesktop(ctx); err != nil {
		slog.Warn("download resource sync failed", "tool", claudeDesktopToolID, "reason", reason, "error", err)
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
		{tool: gitForWindowsToolID, fn: s.SyncGitForWindows},
		{tool: grokBuildToolID, fn: s.SyncGrokBuild},
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
	if s.githubClient == nil {
		return errors.New("download client is not configured")
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

	// OpenAI is authoritative for the CLI and versioned macOS desktop installers. The
	// configured mirror is used only for Windows MSIX packages that OpenAI does
	// not publish through the public Codex release repository.
	version := desktopRelease.TagName + "__" + officialRelease.TagName
	versionDir := filepath.Join(s.cacheDir, codexToolID, sanitizePathSegment(version))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	assets := make([]CachedDownloadAsset, 0, len(officialRelease.Assets)+len(desktopRelease.Assets))
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
	if len(assets) == 0 {
		return errors.New("latest codex releases have no downloadable installer assets")
	}

	manifest := CachedDownloadManifest{
		Tool:        codexToolID,
		Repo:        s.cfg.CodexRepo + ", " + s.cfg.CodexWindowsMirrorRepo,
		Version:     version,
		ReleaseName: desktopRelease.Name,
		PublishedAt: desktopRelease.PublishedAt,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assets:      assets,
	}
	if err := s.writeManifest(manifest); err != nil {
		return err
	}
	s.cleanupUnreferencedAssets(versionDir, assets)
	if err := s.cleanupHistoricalToolVersions(codexToolID, version); err != nil {
		slog.Warn("download resource history cleanup failed", "tool", codexToolID, "error", err)
	}
	slog.Info("download resource synced", "tool", codexToolID, "version", version, "assets", len(assets))
	return nil
}

func (s *DownloadResourceService) SyncCodexPlusPlus(ctx context.Context) error {
	return s.syncGitHubRelease(ctx, codexPlusPlusToolID, s.cfg.CodexPlusPlusRepo, isCodexPlusPlusInstallAsset)
}

func (s *DownloadResourceService) SyncGitForWindows(ctx context.Context) error {
	return s.syncGitHubRelease(ctx, gitForWindowsToolID, s.cfg.GitForWindowsRepo, isGitForWindowsInstallAsset)
}

// SyncGrokBuild stores the native Windows binaries after verifying both xAI
// artifact hosts publish the same stable version. Each cached file is then
// exposed through a content-addressed same-site URL and reverified by clients.
func (s *DownloadResourceService) SyncGrokBuild(ctx context.Context) error {
	if s == nil {
		return errors.New("nil download resource service")
	}
	if s.githubClient == nil {
		return errors.New("download client is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
		return fmt.Errorf("create download cache root: %w", err)
	}

	bases := []string{
		strings.TrimRight(strings.TrimSpace(s.cfg.GrokBuildPrimaryBaseURL), "/"),
		strings.TrimRight(strings.TrimSpace(s.cfg.GrokBuildFallbackBaseURL), "/"),
	}
	if bases[0] == "" || bases[1] == "" || bases[0] == bases[1] {
		return errors.New("grok build requires two distinct official artifact bases")
	}

	versions := make([]string, 0, len(bases))
	for _, base := range bases {
		version, err := s.downloadSmallText(ctx, base+"/stable", 1024)
		if err != nil {
			return fmt.Errorf("fetch grok build stable version from %s: %w", base, err)
		}
		version = strings.TrimSpace(version)
		if !grokBuildVersionPattern.MatchString(version) {
			return fmt.Errorf("invalid grok build stable version from %s", base)
		}
		versions = append(versions, version)
	}
	if versions[0] != versions[1] {
		return fmt.Errorf("grok build official sources disagree on stable version: %s != %s", versions[0], versions[1])
	}

	version := versions[0]
	versionDir := filepath.Join(s.cacheDir, grokBuildToolID, sanitizePathSegment(version))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	assets := make([]CachedDownloadAsset, 0, 2)
	for _, arch := range []struct {
		upstream string
		public   string
	}{
		{upstream: "x86_64", public: "x64"},
		{upstream: "aarch64", public: "arm64"},
	} {
		name := fmt.Sprintf("grok-%s-windows-%s.exe", version, arch.upstream)
		dest := filepath.Join(versionDir, name)
		if err := s.downloadStaticAssetWithFallback(ctx, []string{
			bases[0] + "/" + name,
			bases[1] + "/" + name,
		}, dest); err != nil {
			return fmt.Errorf("cache grok build %s: %w", arch.public, err)
		}
		info, err := os.Stat(dest)
		if err != nil {
			return fmt.Errorf("stat grok build %s: %w", arch.public, err)
		}
		sum, err := fileSHA256(dest)
		if err != nil {
			return fmt.Errorf("checksum grok build %s: %w", arch.public, err)
		}
		assets = append(assets, CachedDownloadAsset{
			ID: makeAssetID(name), Name: name, Size: info.Size(), SHA256: sum,
			Platform: "windows", Arch: arch.public, Path: dest,
		})
	}

	manifest := CachedDownloadManifest{
		Tool:        grokBuildToolID,
		Repo:        strings.Join(bases, ", "),
		Version:     version,
		ReleaseName: "Grok Build " + version,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assets:      assets,
	}
	if err := s.writeManifest(manifest); err != nil {
		return err
	}
	s.cleanupUnreferencedAssets(versionDir, assets)
	if err := s.cleanupHistoricalToolVersions(grokBuildToolID, version); err != nil {
		slog.Warn("download resource history cleanup failed", "tool", grokBuildToolID, "error", err)
	}
	slog.Info("download resource synced", "tool", grokBuildToolID, "version", version, "assets", len(assets))
	return nil
}

func (s *DownloadResourceService) syncGitHubRelease(ctx context.Context, toolID, repo string, include func(string) bool) error {
	if s == nil {
		return errors.New("nil download resource service")
	}
	if s.githubClient == nil {
		return errors.New("download client is not configured")
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
	s.cleanupUnreferencedAssets(versionDir, assets)
	if err := s.cleanupHistoricalToolVersions(toolID, release.TagName); err != nil {
		slog.Warn("download resource history cleanup failed", "tool", toolID, "error", err)
	}
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

func (s *DownloadResourceService) downloadSmallText(ctx context.Context, url string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		return "", errors.New("invalid small text limit")
	}
	tmp, err := os.CreateTemp(s.cacheDir, ".download-text-*")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	defer func() { _ = os.Remove(path) }()
	if err := s.githubClient.DownloadFile(ctx, url, path, maxBytes); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s *DownloadResourceService) downloadStaticAssetWithFallback(ctx context.Context, urls []string, dest string) error {
	if len(urls) == 0 {
		return errors.New("at least one artifact source is required")
	}
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		return nil
	}

	tmp := dest + ".tmp"
	_ = os.Remove(tmp)
	defer func() { _ = os.Remove(tmp) }()
	var lastErr error
	for _, url := range urls {
		_ = os.Remove(tmp)
		if err := s.githubClient.DownloadFile(ctx, url, tmp, s.cfg.MaxAssetBytes); err != nil {
			lastErr = err
			continue
		}
		return os.Rename(tmp, dest)
	}
	return fmt.Errorf("all artifact sources failed: %w", lastErr)
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
			tool:        "codex-plus-plus",
			name:        "Codex++",
			manifest:    codexPlusPlusToolID,
			repo:        s.cfg.CodexPlusPlusRepo,
			officialURL: "https://github.com/BigPizzaV3/CodexPlusPlus/releases/latest",
			cacheMode:   "cached",
			note:        "本站缓存 Codex++ 官方 Release 中的 Windows 和 macOS 安装包。",
			comparable:  true,
		},
		{
			tool:        "claude-desktop",
			name:        "Claude Desktop",
			manifest:    claudeDesktopToolID,
			officialURL: "https://claude.com/download",
			cacheMode:   "cached",
			note:        "本站缓存 Claude Desktop 官方安装包，并通过内容寻址静态路径分发。",
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
			tool:        "grok-build",
			name:        "Grok Build",
			manifest:    grokBuildToolID,
			officialURL: "https://docs.x.ai/build/overview",
			cacheMode:   "cached",
			note:        "本站对比 xAI 两个官方制品源的 stable 版本，并缓存 Windows x64/ARM64 二进制。",
			comparable:  false,
		},
		{
			tool:        "git-for-windows",
			name:        "Git for Windows",
			manifest:    gitForWindowsToolID,
			repo:        s.cfg.GitForWindowsRepo,
			officialURL: "https://github.com/git-for-windows/git/releases/latest",
			cacheMode:   "cached",
			note:        "本站缓存官方 Windows x64/ARM64 安装版，供 Claude Code 自动准备 Git Bash。",
			comparable:  true,
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

		if strings.TrimSpace(item.repo) != "" && s.githubClient != nil {
			release, err := s.githubClient.FetchLatestRelease(ctx, item.repo)
			if err == nil && release != nil {
				status.OfficialVersion = release.TagName
				status.OfficialPublishedAt = release.PublishedAt
			}
		}

		switch {
		case item.cacheMode == "npm-mirror" && status.OfficialVersion != "":
			status.State = "npm-mirror"
		case status.CachedVersion == "":
			status.State = "cache-missing"
		case !item.comparable:
			status.State = "cached"
		case status.OfficialVersion == "":
			status.State = "official-unavailable"
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

// GetImmutableToolAsset resolves an asset from the manifest stored beside a
// concrete cached version. It deliberately does not consult manifest.json,
// because that pointer advances to the next release while previously emitted
// content-addressed URLs must remain valid for their advertised cache lifetime.
func (s *DownloadResourceService) GetImmutableToolAsset(
	ctx context.Context,
	toolID, version, assetID string,
) (*DownloadAssetFile, error) {
	_ = ctx
	toolID, ok := normalizeDownloadToolID(toolID)
	if !ok {
		return nil, ErrDownloadToolNotFound
	}
	version = strings.TrimSpace(version)
	assetID = strings.TrimSpace(assetID)
	if version == "" || assetID == "" || version != sanitizePathSegment(version) || assetID != makeAssetID(assetID) {
		return nil, ErrDownloadAssetNotFound
	}

	manifest, err := s.readVersionManifest(toolID, version)
	if err != nil {
		return nil, err
	}
	if sanitizePathSegment(manifest.Version) != version {
		return nil, ErrDownloadAssetNotFound
	}
	for _, asset := range manifest.Assets {
		if asset.ID != assetID || !sha256Pattern.MatchString(strings.TrimSpace(asset.SHA256)) {
			continue
		}
		asset.Path = filepath.Join(s.cacheDir, toolID, version, filepath.Base(asset.Name))
		if !isPathWithin(filepath.Join(s.cacheDir, toolID, version), asset.Path) {
			return nil, errors.New("cached immutable asset path escapes version dir")
		}
		info, statErr := os.Stat(asset.Path)
		if statErr != nil || !info.Mode().IsRegular() {
			if statErr == nil {
				statErr = errors.New("cached immutable asset is not a regular file")
			}
			return nil, fmt.Errorf("cached immutable asset missing: %w", statErr)
		}
		return &DownloadAssetFile{Asset: asset, Path: asset.Path}, nil
	}
	return nil, ErrDownloadAssetNotFound
}

// GetImmutableToolAssetBySHA preserves compatibility with the first generation
// of content-addressed URLs, which included the digest but not a version. New
// URLs use GetImmutableToolAsset and include both. The bounded directory scan
// is only used for those legacy URLs and still requires a manifest match before
// any file can be served.
func (s *DownloadResourceService) GetImmutableToolAssetBySHA(
	ctx context.Context,
	toolID, requestedSHA256, assetID string,
) (*DownloadAssetFile, error) {
	_ = ctx
	toolID, ok := normalizeDownloadToolID(toolID)
	if !ok {
		return nil, ErrDownloadToolNotFound
	}
	requestedSHA256 = strings.ToLower(strings.TrimSpace(requestedSHA256))
	assetID = strings.TrimSpace(assetID)
	if !sha256Pattern.MatchString(requestedSHA256) || (assetID != "" && assetID != makeAssetID(assetID)) {
		return nil, ErrDownloadAssetNotFound
	}

	toolDir := filepath.Join(s.cacheDir, toolID)
	entries, err := os.ReadDir(toolDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDownloadManifestNotReady
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() != sanitizePathSegment(entry.Name()) {
			continue
		}
		file, lookupErr := s.findAssetInVersionManifest(toolID, entry.Name(), requestedSHA256, assetID)
		if lookupErr == nil {
			return file, nil
		}
		if !errors.Is(lookupErr, ErrDownloadAssetNotFound) && !errors.Is(lookupErr, ErrDownloadManifestNotReady) {
			return nil, lookupErr
		}
	}

	// Upgrade compatibility for the one current release whose per-version
	// manifest may not exist until the first post-deploy sync finishes.
	current, err := s.readManifest(toolID)
	if err != nil {
		return nil, err
	}
	version := sanitizePathSegment(current.Version)
	return s.findAssetInManifest(toolID, version, current, requestedSHA256, assetID)
}

func (s *DownloadResourceService) findAssetInVersionManifest(
	toolID, version, requestedSHA256, assetID string,
) (*DownloadAssetFile, error) {
	manifest, err := s.readVersionManifest(toolID, version)
	if err != nil {
		return nil, err
	}
	return s.findAssetInManifest(toolID, version, manifest, requestedSHA256, assetID)
}

func (s *DownloadResourceService) findAssetInManifest(
	toolID, version string,
	manifest *CachedDownloadManifest,
	requestedSHA256, assetID string,
) (*DownloadAssetFile, error) {
	if manifest == nil || sanitizePathSegment(manifest.Version) != version {
		return nil, ErrDownloadAssetNotFound
	}
	for _, asset := range manifest.Assets {
		if !strings.EqualFold(strings.TrimSpace(asset.SHA256), requestedSHA256) ||
			(assetID != "" && asset.ID != assetID) {
			continue
		}
		asset.Path = filepath.Join(s.cacheDir, toolID, version, filepath.Base(asset.Name))
		if !isPathWithin(filepath.Join(s.cacheDir, toolID, version), asset.Path) {
			return nil, errors.New("cached immutable asset path escapes version dir")
		}
		info, err := os.Stat(asset.Path)
		if err != nil || !info.Mode().IsRegular() {
			if err == nil {
				err = errors.New("cached immutable asset is not a regular file")
			}
			return nil, fmt.Errorf("cached immutable asset missing: %w", err)
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

// GetClaudeDesktopWindowsX64Asset returns the verified Windows x64 installer
// from the local download cache. The public HTTP layer exposes this file using
// its SHA256 in the URL so edge caches can safely retain it for a year.
func (s *DownloadResourceService) GetClaudeDesktopWindowsX64Asset(ctx context.Context) (*DownloadAssetFile, error) {
	manifest, err := s.ListTool(ctx, claudeDesktopToolID)
	if err != nil {
		return nil, err
	}
	for _, asset := range manifest.Assets {
		if asset.Platform == "windows" && asset.Arch == "x64" && asset.Role == claudeInstallerRole &&
			strings.EqualFold(filepath.Ext(asset.Name), ".msix") {
			return s.GetToolAsset(ctx, claudeDesktopToolID, asset.ID)
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

func (s *DownloadResourceService) versionManifestPath(toolID, version string) string {
	return filepath.Join(s.cacheDir, toolID, version, versionManifestName)
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

func (s *DownloadResourceService) readVersionManifest(toolID, version string) (*CachedDownloadManifest, error) {
	raw, err := os.ReadFile(s.versionManifestPath(toolID, version))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		// Upgrade compatibility: the release active at deployment may only have
		// the old root manifest. Use it only when it names the requested version;
		// the next successful sync writes the durable per-version copy.
		current, currentErr := s.readManifest(toolID)
		if currentErr != nil {
			return nil, currentErr
		}
		if sanitizePathSegment(current.Version) != version {
			return nil, ErrDownloadAssetNotFound
		}
		return current, nil
	}
	var manifest CachedDownloadManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (s *DownloadResourceService) writeManifest(manifest CachedDownloadManifest) error {
	toolID, ok := normalizeDownloadToolID(manifest.Tool)
	if !ok {
		return ErrDownloadToolNotFound
	}
	rootDir := filepath.Join(s.cacheDir, toolID)
	version := sanitizePathSegment(manifest.Version)
	versionDir := filepath.Join(rootDir, version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	// Persist the version manifest before advancing the mutable pointer. If
	// either write fails, manifest.json never advertises an immutable URL we
	// cannot resolve. Successful sync then removes prior version directories.
	if err := writeFileAtomically(s.versionManifestPath(toolID, version), raw, 0644); err != nil {
		return err
	}
	return writeFileAtomically(s.manifestPath(toolID), raw, 0644)
}

func writeFileAtomically(path string, raw []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *DownloadResourceService) cleanupUnreferencedAssets(versionDir string, assets []CachedDownloadAsset) {
	keep := make(map[string]struct{}, len(assets)+1)
	keep[versionManifestName] = struct{}{}
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

// cleanupHistoricalToolVersions enforces the production cache policy: only
// the version named by the current manifest remains locally available. The
// caller holds s.mu, so a successful sync cannot race another in-process sync
// while old or incomplete version directories are removed.
func (s *DownloadResourceService) cleanupHistoricalToolVersions(toolID, current string, preservedNames ...string) error {
	toolID, ok := normalizeDownloadToolID(toolID)
	if !ok {
		return ErrDownloadToolNotFound
	}
	current = strings.TrimSpace(current)
	if current == "" {
		return errors.New("current download version is empty")
	}
	root := filepath.Join(s.cacheDir, toolID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	keep := map[string]struct{}{
		sanitizePathSegment(current): {},
	}
	for _, name := range preservedNames {
		keep[name] = struct{}{}
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := keep[name]; ok {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || sanitizePathSegment(name) != name {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
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
	return ((strings.HasSuffix(lower, "apple-darwin.tar.gz") || strings.HasSuffix(lower, "apple-darwin.dmg")) &&
		(strings.Contains(lower, "aarch64") || strings.Contains(lower, "x86_64"))) ||
		(strings.HasSuffix(lower, "pc-windows-msvc.exe.zip") &&
			(strings.Contains(lower, "aarch64") || strings.Contains(lower, "x86_64")))
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

func isGitForWindowsInstallAsset(name string) bool {
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".exe") || strings.Contains(lower, "portablegit") || strings.Contains(lower, "mingit") {
		return false
	}
	return gitForWindowsAssetPattern.MatchString(lower)
}

func classifyPlatform(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "windows") || strings.Contains(lower, "win32") || strings.Contains(lower, "pc-windows") || strings.HasSuffix(lower, ".msix") || strings.HasSuffix(lower, ".exe"):
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
	case strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") || strings.Contains(lower, "x64") || strings.Contains(lower, "64-bit"):
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
	case gitForWindowsToolID:
		return gitForWindowsToolID, true
	case grokBuildToolID:
		return grokBuildToolID, true
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
