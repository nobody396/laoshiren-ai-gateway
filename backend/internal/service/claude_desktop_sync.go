package service

import (
	"archive/zip"
	"bytes"
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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	claudeDesktopASARPath  = "app/resources/app.asar"
	claudeDesktopMaxASAR   = 128 * 1024 * 1024
	claudeCodeAssetRole    = "claude-desktop-code"
	claudeInstallerRole    = "installer"
	claudeCodeOfficialBase = "https://downloads.claude.ai/claude-code-releases"
)

var claudeDesktopCommitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

type claudeDesktopLatest struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

type claudeDesktopBuildInfo struct {
	CommitHash string `json:"commitHash"`
	AppVersion string `json:"appVersion"`
}

type claudeCodePlatform struct {
	Binary   string `json:"binary"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

type claudeCodeManifest struct {
	Version   string                        `json:"version"`
	Platforms map[string]claudeCodePlatform `json:"platforms"`
}

type claudeCodePin struct {
	Version  string             `json:"version"`
	Manifest claudeCodeManifest `json:"manifest"`
	BaseURL  string             `json:"baseUrl"`
}

// SyncClaudeDesktop discovers Anthropic's current immutable MSIX release and
// the exact Desktop Code binary pinned inside it. The root manifest advances
// only after both architectures and both layers are downloaded and verified.
func (s *DownloadResourceService) SyncClaudeDesktop(ctx context.Context) error {
	if s == nil {
		return errors.New("nil download resource service")
	}
	if s.githubClient == nil {
		return errors.New("download client is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
		return fmt.Errorf("create download cache: %w", err)
	}

	x64Latest, err := s.fetchClaudeDesktopLatest(ctx, "x64")
	if err != nil {
		return err
	}
	arm64Latest, err := s.fetchClaudeDesktopLatest(ctx, "arm64")
	if err != nil {
		return err
	}
	if x64Latest != arm64Latest {
		return fmt.Errorf("claude desktop architecture releases disagree: x64=%s/%s arm64=%s/%s",
			x64Latest.Version, x64Latest.Hash, arm64Latest.Version, arm64Latest.Hash)
	}

	releaseID := claudeDesktopReleaseID(x64Latest)
	if current, readErr := s.readManifest(claudeDesktopToolID); readErr == nil &&
		current.Version == releaseID && s.claudeManifestComplete(current) {
		return nil
	}

	versionDir := filepath.Join(s.cacheDir, claudeDesktopToolID, releaseID)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create claude desktop cache dir: %w", err)
	}

	assets := make([]CachedDownloadAsset, 0, 5)
	for _, arch := range []string{"x64", "arm64"} {
		installer, pin, cacheErr := s.cacheClaudeDesktopArchitecture(ctx, versionDir, arch, x64Latest)
		if cacheErr != nil {
			return cacheErr
		}
		assets = append(assets, installer)

		component, cacheErr := s.cacheClaudeDesktopCode(ctx, versionDir, arch, pin)
		if cacheErr != nil {
			return cacheErr
		}
		assets = append(assets, component)
	}

	// macOS is independent of the Windows/Desktop Code pair. Preserve the
	// existing download option when its upstream is reachable, but never block
	// the mainland Windows repair because the macOS snapshot is unavailable.
	if macAsset, macErr := s.cacheClaudeDesktopMac(ctx, versionDir); macErr != nil {
		slog.Warn("claude desktop macOS cache unavailable", "error", macErr)
	} else if macAsset != nil {
		assets = append(assets, *macAsset)
	}

	// A release can change while more than 1 GB of packages are being copied.
	// Recheck both tiny pointers and publish only if the prepared pair is still
	// authoritative. The previous manifest remains untouched on any mismatch.
	x64Final, err := s.fetchClaudeDesktopLatest(ctx, "x64")
	if err != nil {
		return fmt.Errorf("recheck claude desktop x64 release: %w", err)
	}
	arm64Final, err := s.fetchClaudeDesktopLatest(ctx, "arm64")
	if err != nil {
		return fmt.Errorf("recheck claude desktop arm64 release: %w", err)
	}
	if x64Final != x64Latest || arm64Final != arm64Latest {
		return errors.New("claude desktop release changed during synchronization; keeping previous complete release")
	}

	manifest := CachedDownloadManifest{
		Tool:        claudeDesktopToolID,
		Repo:        "downloads.claude.ai",
		Version:     releaseID,
		ReleaseName: "Claude Desktop " + x64Latest.Version,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Assets:      assets,
	}
	if !s.claudeManifestComplete(&manifest) {
		return errors.New("claude desktop release is missing a verified installer/component pair")
	}
	if err := s.writeManifest(manifest); err != nil {
		return err
	}
	s.cleanupUnreferencedAssets(versionDir, assets)
	if err := s.cleanupOldClaudeDesktopVersions(releaseID, s.cfg.ClaudeDesktopRetainVersions); err != nil {
		slog.Warn("claude desktop cache retention cleanup failed", "error", err)
	}
	slog.Info("download resource synced", "tool", claudeDesktopToolID, "version", releaseID, "assets", len(assets))
	return nil
}

func (s *DownloadResourceService) fetchClaudeDesktopLatest(ctx context.Context, arch string) (claudeDesktopLatest, error) {
	if arch != "x64" && arch != "arm64" {
		return claudeDesktopLatest{}, fmt.Errorf("unsupported claude desktop architecture: %s", arch)
	}
	base := strings.TrimRight(strings.TrimSpace(s.cfg.ClaudeDesktopLatestBaseURL), "/")
	if base == "" {
		return claudeDesktopLatest{}, errors.New("claude desktop latest base URL is empty")
	}
	raw, err := s.downloadSmallText(ctx, base+"/"+arch+"/.latest", 4096)
	if err != nil {
		return claudeDesktopLatest{}, fmt.Errorf("fetch claude desktop %s latest metadata: %w", arch, err)
	}
	var latest claudeDesktopLatest
	if err := json.Unmarshal([]byte(raw), &latest); err != nil {
		return claudeDesktopLatest{}, fmt.Errorf("decode claude desktop %s latest metadata: %w", arch, err)
	}
	latest.Version = strings.TrimSpace(latest.Version)
	latest.Hash = strings.ToLower(strings.TrimSpace(latest.Hash))
	if !grokBuildVersionPattern.MatchString(latest.Version) || !claudeDesktopCommitPattern.MatchString(latest.Hash) {
		return claudeDesktopLatest{}, fmt.Errorf("invalid claude desktop %s latest metadata", arch)
	}
	return latest, nil
}

func claudeDesktopReleaseID(latest claudeDesktopLatest) string {
	return sanitizePathSegment(latest.Version + "-" + latest.Hash[:12])
}

func (s *DownloadResourceService) claudeManifestComplete(manifest *CachedDownloadManifest) bool {
	if manifest == nil || strings.TrimSpace(manifest.Version) == "" {
		return false
	}
	found := make(map[string]bool, 4)
	for _, asset := range manifest.Assets {
		if asset.Platform != "windows" || (asset.Arch != "x64" && asset.Arch != "arm64") {
			continue
		}
		if asset.Role != claudeInstallerRole && asset.Role != claudeCodeAssetRole {
			continue
		}
		if asset.Role == claudeCodeAssetRole &&
			(strings.TrimSpace(asset.ComponentVersion) == "" || !sha256Pattern.MatchString(asset.UpstreamSHA256)) {
			continue
		}
		path := filepath.Join(s.cacheDir, claudeDesktopToolID, sanitizePathSegment(manifest.Version), filepath.Base(asset.Name))
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != asset.Size || asset.Size <= 0 ||
			!sha256Pattern.MatchString(asset.SHA256) {
			continue
		}
		found[asset.Arch+":"+asset.Role] = true
	}
	return found["x64:"+claudeInstallerRole] && found["x64:"+claudeCodeAssetRole] &&
		found["arm64:"+claudeInstallerRole] && found["arm64:"+claudeCodeAssetRole]
}

func (s *DownloadResourceService) cacheClaudeDesktopArchitecture(
	ctx context.Context,
	versionDir, arch string,
	latest claudeDesktopLatest,
) (CachedDownloadAsset, claudeCodePin, error) {
	name := fmt.Sprintf("Claude-%s-%s.msix", latest.Version, arch)
	dest := filepath.Join(versionDir, name)
	base := strings.TrimRight(strings.TrimSpace(s.cfg.ClaudeDesktopLatestBaseURL), "/")
	url := fmt.Sprintf("%s/%s/%s/Claude-%s.msix", base, arch, latest.Version, latest.Hash)

	if err := s.ensureStaticAsset(ctx, url, dest); err != nil {
		return CachedDownloadAsset{}, claudeCodePin{}, fmt.Errorf("cache claude desktop %s MSIX: %w", arch, err)
	}
	build, pin, err := inspectClaudeDesktopMSIX(dest)
	if err != nil || build.AppVersion != latest.Version || !strings.EqualFold(build.CommitHash, latest.Hash) {
		_ = os.Remove(dest)
		if err == nil {
			err = errors.New("embedded build identity does not match .latest")
		}
		return CachedDownloadAsset{}, claudeCodePin{}, fmt.Errorf("verify claude desktop %s MSIX: %w", arch, err)
	}
	platform := "win32-" + arch
	if _, ok := pin.Manifest.Platforms[platform]; !ok {
		_ = os.Remove(dest)
		return CachedDownloadAsset{}, claudeCodePin{}, fmt.Errorf("claude desktop %s MSIX has no %s code component", arch, platform)
	}

	info, err := os.Stat(dest)
	if err != nil {
		return CachedDownloadAsset{}, claudeCodePin{}, err
	}
	sum, err := fileSHA256(dest)
	if err != nil {
		return CachedDownloadAsset{}, claudeCodePin{}, err
	}
	return CachedDownloadAsset{
		ID: makeAssetID(name), Name: name, Size: info.Size(), SHA256: sum,
		Platform: "windows", Arch: arch, Role: claudeInstallerRole, Path: dest,
	}, pin, nil
}

func inspectClaudeDesktopMSIX(path string) (claudeDesktopBuildInfo, claudeCodePin, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return claudeDesktopBuildInfo{}, claudeCodePin{}, err
	}
	defer func() { _ = reader.Close() }()

	var asarFile *zip.File
	hasSignature := false
	for _, entry := range reader.File {
		switch entry.Name {
		case claudeDesktopASARPath:
			asarFile = entry
		case "AppxSignature.p7x":
			hasSignature = entry.UncompressedSize64 > 0
		}
	}
	if asarFile == nil || !hasSignature || asarFile.UncompressedSize64 == 0 || asarFile.UncompressedSize64 > claudeDesktopMaxASAR {
		return claudeDesktopBuildInfo{}, claudeCodePin{}, errors.New("MSIX is missing a bounded app.asar or AppX signature")
	}
	stream, err := asarFile.Open()
	if err != nil {
		return claudeDesktopBuildInfo{}, claudeCodePin{}, err
	}
	defer func() { _ = stream.Close() }()
	raw, err := io.ReadAll(io.LimitReader(stream, claudeDesktopMaxASAR+1))
	if err != nil || int64(len(raw)) > claudeDesktopMaxASAR {
		return claudeDesktopBuildInfo{}, claudeCodePin{}, errors.New("read bounded Claude app.asar")
	}

	var build claudeDesktopBuildInfo
	var pin claudeCodePin
	for _, literal := range jsonParseLiterals(raw) {
		if build.AppVersion == "" {
			var candidate claudeDesktopBuildInfo
			if json.Unmarshal(literal, &candidate) == nil && candidate.AppVersion != "" && candidate.CommitHash != "" {
				build = candidate
			}
		}
		if pin.Version == "" {
			var candidate claudeCodePin
			if json.Unmarshal(literal, &candidate) == nil && candidate.BaseURL == claudeCodeOfficialBase &&
				candidate.Version != "" && candidate.Version == candidate.Manifest.Version {
				pin = candidate
			}
		}
	}
	if build.AppVersion == "" || !claudeDesktopCommitPattern.MatchString(strings.ToLower(build.CommitHash)) {
		return claudeDesktopBuildInfo{}, claudeCodePin{}, errors.New("claude app.asar is missing build identity")
	}
	if pin.Version == "" || pin.BaseURL != claudeCodeOfficialBase || len(pin.Manifest.Platforms) == 0 {
		return claudeDesktopBuildInfo{}, claudeCodePin{}, errors.New("claude app.asar is missing the Desktop Code pin")
	}
	return build, pin, nil
}

func jsonParseLiterals(raw []byte) [][]byte {
	prefix := []byte("JSON.parse(`")
	suffix := []byte("`)")
	result := make([][]byte, 0, 4)
	for start := 0; ; {
		rel := bytes.Index(raw[start:], prefix)
		if rel < 0 {
			break
		}
		contentStart := start + rel + len(prefix)
		contentEndRel := bytes.Index(raw[contentStart:], suffix)
		if contentEndRel < 0 {
			break
		}
		contentEnd := contentStart + contentEndRel
		literal := raw[contentStart:contentEnd]
		if json.Valid(literal) {
			result = append(result, bytes.Clone(literal))
		} else if decoded, err := strconv.Unquote(`"` + string(literal) + `"`); err == nil && json.Valid([]byte(decoded)) {
			result = append(result, []byte(decoded))
		}
		start = contentEnd + 2
	}
	return result
}

func (s *DownloadResourceService) cleanupOldClaudeDesktopVersions(current string, retain int) error {
	if retain < 1 {
		retain = 1
	}
	root := filepath.Join(s.cacheDir, claudeDesktopToolID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type candidate struct {
		path    string
		modTime time.Time
	}
	old := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == current || name == "macos-static" || sanitizePathSegment(name) != name {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, name, versionManifestName))
		if err != nil {
			continue
		}
		var manifest CachedDownloadManifest
		if json.Unmarshal(raw, &manifest) != nil || manifest.Tool != claudeDesktopToolID || sanitizePathSegment(manifest.Version) != name {
			continue
		}
		old = append(old, candidate{path: filepath.Join(root, name), modTime: info.ModTime()})
	}
	sort.Slice(old, func(i, j int) bool { return old[i].modTime.After(old[j].modTime) })
	keepOld := retain - 1
	if keepOld > len(old) {
		keepOld = len(old)
	}
	for _, stale := range old[keepOld:] {
		if err := os.RemoveAll(stale.path); err != nil {
			return err
		}
	}
	return nil
}

func (s *DownloadResourceService) cacheClaudeDesktopCode(
	ctx context.Context,
	versionDir, arch string,
	pin claudeCodePin,
) (CachedDownloadAsset, error) {
	platform := "win32-" + arch
	upstream, ok := pin.Manifest.Platforms[platform]
	if !ok || upstream.Size <= 0 || !sha256Pattern.MatchString(upstream.Checksum) {
		return CachedDownloadAsset{}, fmt.Errorf("invalid Claude Desktop Code metadata for %s", platform)
	}
	binary := filepath.Base(strings.TrimSpace(upstream.Binary))
	if binary != upstream.Binary || !strings.HasSuffix(strings.ToLower(binary), ".zst") {
		return CachedDownloadAsset{}, fmt.Errorf("unsafe Claude Desktop Code binary name for %s", platform)
	}
	name := fmt.Sprintf("Claude-Desktop-Code-%s-%s.exe", pin.Version, arch)
	dest := filepath.Join(versionDir, name)
	compressed := dest + ".zst.tmp"
	decompressed := dest + ".tmp"
	_ = os.Remove(compressed)
	_ = os.Remove(decompressed)
	defer func() {
		_ = os.Remove(compressed)
		_ = os.Remove(decompressed)
	}()

	url := strings.TrimRight(pin.BaseURL, "/") + "/" + pin.Version + "/" + platform + "/" + binary
	if err := s.githubClient.DownloadFile(ctx, url, compressed, s.cfg.MaxAssetBytes); err != nil {
		return CachedDownloadAsset{}, fmt.Errorf("download Claude Desktop Code %s: %w", platform, err)
	}
	info, err := os.Stat(compressed)
	if err != nil || info.Size() != upstream.Size {
		return CachedDownloadAsset{}, fmt.Errorf("claude Desktop Code %s compressed size mismatch", platform)
	}
	compressedSHA, err := fileSHA256(compressed)
	if err != nil || !strings.EqualFold(compressedSHA, upstream.Checksum) {
		return CachedDownloadAsset{}, fmt.Errorf("claude Desktop Code %s compressed checksum mismatch", platform)
	}

	rawSHA, rawSize, err := decompressClaudeCode(compressed, decompressed, s.cfg.MaxAssetBytes)
	if err != nil {
		return CachedDownloadAsset{}, fmt.Errorf("decompress Claude Desktop Code %s: %w", platform, err)
	}
	if err := validateWindowsExecutable(decompressed); err != nil {
		return CachedDownloadAsset{}, fmt.Errorf("verify Claude Desktop Code %s executable: %w", platform, err)
	}
	if err := os.Rename(decompressed, dest); err != nil {
		return CachedDownloadAsset{}, err
	}
	return CachedDownloadAsset{
		ID: makeAssetID(name), Name: name, Size: rawSize, SHA256: rawSHA,
		Platform: "windows", Arch: arch, Role: claudeCodeAssetRole,
		ComponentVersion: pin.Version, UpstreamSHA256: strings.ToLower(upstream.Checksum),
		UpstreamCompressed: upstream.Size, Path: dest,
	}, nil
}

func decompressClaudeCode(source, dest string, maxBytes int64) (string, int64, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = in.Close() }()
	decoder, err := zstd.NewReader(in)
	if err != nil {
		return "", 0, err
	}
	defer decoder.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(decoder, maxBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	if written <= 0 || written > maxBytes {
		return "", 0, fmt.Errorf("decompressed payload size %d exceeds limit", written)
	}
	return hex.EncodeToString(hash.Sum(nil)), written, nil
}

func validateWindowsExecutable(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	header := make([]byte, 2)
	if _, err := io.ReadFull(file, header); err != nil {
		return err
	}
	if string(header) != "MZ" {
		return errors.New("missing PE header")
	}
	return nil
}

func (s *DownloadResourceService) cacheClaudeDesktopMac(ctx context.Context, versionDir string) (*CachedDownloadAsset, error) {
	url := strings.TrimSpace(s.cfg.ClaudeDesktopMacURL)
	if url == "" {
		return nil, nil
	}
	stableDir := filepath.Join(s.cacheDir, claudeDesktopToolID, "macos-static")
	if err := os.MkdirAll(stableDir, 0755); err != nil {
		return nil, err
	}
	stable := filepath.Join(stableDir, "Claude.dmg")
	if err := s.ensureStaticAsset(ctx, url, stable); err != nil {
		return nil, err
	}
	dest := filepath.Join(versionDir, "Claude.dmg")
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		if err := os.Link(stable, dest); err != nil {
			if err := copyFileAtomically(stable, dest); err != nil {
				return nil, err
			}
		}
	}
	info, err := os.Stat(dest)
	if err != nil {
		return nil, err
	}
	sum, err := fileSHA256(dest)
	if err != nil {
		return nil, err
	}
	return &CachedDownloadAsset{
		ID: makeAssetID("Claude.dmg"), Name: "Claude.dmg", Size: info.Size(), SHA256: sum,
		Platform: "macos", Arch: "universal", Role: claudeInstallerRole, Path: dest,
	}, nil
}

func copyFileAtomically(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dest + ".tmp"
	_ = os.Remove(tmp)
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dest)
}
