package service

import (
	"context"
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
	ccSwitchToolID      = "cc-switch"
	codexToolID         = "codex"
	codexPlusPlusToolID = "codex-plus-plus"
	claudeDesktopToolID = "claude-desktop"
	defaultCCSwitchRepo = "farion1231/cc-switch"
	defaultCodexRepo    = "openai/codex"
	defaultCodexPPRepo  = "BigPizzaV3/CodexPlusPlus"
	defaultClaudeMacURL = "https://storage.googleapis.com/osprey-downloads-c02f6a0d-347c-492b-a752-3e0651722e97/nest/Claude.dmg"
	defaultClaudeWinURL = "https://storage.googleapis.com/osprey-downloads-c02f6a0d-347c-492b-a752-3e0651722e97/nest-win-x64/Claude-Setup-x64.exe"
	defaultClaudeARMURL = "https://storage.googleapis.com/osprey-downloads-c02f6a0d-347c-492b-a752-3e0651722e97/nest-win-arm64/Claude-Setup-arm64.exe"
)

var (
	ErrDownloadManifestNotReady = errors.New("download manifest is not ready")
	ErrDownloadAssetNotFound    = errors.New("download asset not found")
	ErrDownloadToolNotFound     = errors.New("download tool not found")
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
}

func NewDownloadResourceService(cfg *config.Config, githubClient GitHubReleaseClient) *DownloadResourceService {
	downloadCfg := config.DownloadsConfig{
		Enabled:                      true,
		CacheDir:                     "./data/downloads",
		UpdateIntervalHours:          48,
		StartupSync:                  true,
		CCSwitchRepo:                 defaultCCSwitchRepo,
		CodexRepo:                    defaultCodexRepo,
		CodexPlusPlusRepo:            defaultCodexPPRepo,
		ClaudeDesktopMacURL:          defaultClaudeMacURL,
		ClaudeDesktopWindowsX64URL:   defaultClaudeWinURL,
		ClaudeDesktopWindowsARM64URL: defaultClaudeARMURL,
		MaxAssetBytes:                300 * 1024 * 1024,
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
		downloadCfg.UpdateIntervalHours = 48
	}
	if downloadCfg.MaxAssetBytes <= 0 {
		downloadCfg.MaxAssetBytes = 300 * 1024 * 1024
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &DownloadResourceService{
		cfg:          downloadCfg,
		githubClient: githubClient,
		cacheDir:     filepath.Clean(downloadCfg.CacheDir),
		ctx:          ctx,
		cancel:       cancel,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
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
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Minute)
	defer cancel()
	for _, syncFn := range []struct {
		tool string
		fn   func(context.Context) error
	}{
		{tool: ccSwitchToolID, fn: s.SyncCCSwitch},
		{tool: codexToolID, fn: s.SyncCodex},
		{tool: codexPlusPlusToolID, fn: s.SyncCodexPlusPlus},
		{tool: claudeDesktopToolID, fn: s.SyncClaudeDesktop},
	} {
		if err := syncFn.fn(ctx); err != nil {
			slog.Warn("download resource sync failed", "tool", syncFn.tool, "reason", reason, "error", err)
		}
	}
}

func (s *DownloadResourceService) SyncCCSwitch(ctx context.Context) error {
	return s.syncGitHubRelease(ctx, ccSwitchToolID, s.cfg.CCSwitchRepo, isCCSwitchInstallAsset)
}

func (s *DownloadResourceService) SyncCodex(ctx context.Context) error {
	return s.syncGitHubRelease(ctx, codexToolID, s.cfg.CodexRepo, isCodexInstallAsset)
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

		dest := filepath.Join(versionDir, filepath.Base(asset.Name))
		if err := s.ensureAsset(ctx, asset, dest); err != nil {
			return fmt.Errorf("cache asset %s: %w", asset.Name, err)
		}
		sum, err := fileSHA256(dest)
		if err != nil {
			return fmt.Errorf("checksum asset %s: %w", asset.Name, err)
		}
		assets = append(assets, CachedDownloadAsset{
			ID:       makeAssetID(asset.Name),
			Name:     asset.Name,
			Size:     asset.Size,
			SHA256:   sum,
			Platform: classifyPlatform(asset.Name),
			Arch:     classifyArch(asset.Name),
			Path:     dest,
		})
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
	if strings.Contains(lower, "sigstore") ||
		strings.HasSuffix(lower, ".zst") ||
		strings.HasSuffix(lower, ".whl") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.Contains(lower, "argument-comment-lint") ||
		strings.Contains(lower, "bwrap") ||
		strings.Contains(lower, "command-runner") ||
		strings.Contains(lower, "responses-api-proxy") ||
		strings.Contains(lower, "windows-sandbox-setup") ||
		lower == "codex" ||
		lower == "codex-app-server" ||
		lower == "config-schema.json" ||
		lower == "install.sh" ||
		lower == "install.ps1" {
		return false
	}
	if strings.HasPrefix(lower, "codex-package-") ||
		(strings.HasPrefix(lower, "codex-app-server-") && !strings.HasPrefix(lower, "codex-app-server-package-")) {
		return false
	}
	if strings.HasPrefix(lower, "codex-app-server-package-") {
		return strings.HasSuffix(lower, ".tar.gz")
	}
	if !strings.HasPrefix(lower, "codex-") {
		return false
	}
	return strings.HasSuffix(lower, "apple-darwin.tar.gz") ||
		strings.HasSuffix(lower, "unknown-linux-musl.tar.gz") ||
		strings.HasSuffix(lower, "pc-windows-msvc.exe.zip")
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
	case strings.Contains(lower, "windows") || strings.Contains(lower, "win32") || strings.Contains(lower, "pc-windows"):
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
