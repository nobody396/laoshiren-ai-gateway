package service

import (
	"encoding/json"
	"regexp"
	"strings"
)

// NewMetadataFormatMinVersion is the first Claude Code version that switched
// metadata.user_id from the legacy concatenated string to JSON.
const NewMetadataFormatMinVersion = "2.1.78"

// ParsedUserID contains the extracted metadata.user_id components.
type ParsedUserID struct {
	DeviceID    string
	AccountUUID string
	SessionID   string
	IsNewFormat bool
}

var legacyUserIDRegex = regexp.MustCompile(`^user_([a-fA-F0-9]{64})_account_([a-fA-F0-9-]*)_session_([a-fA-F0-9-]{36})$`)

type jsonUserID struct {
	DeviceID    string `json:"device_id"`
	AccountUUID string `json:"account_uuid"`
	SessionID   string `json:"session_id"`
}

// ParseMetadataUserID supports both the legacy concatenated format and the new
// JSON format used by newer Claude Code versions.
func ParseMetadataUserID(raw string) *ParsedUserID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if raw[0] == '{' {
		var v jsonUserID
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil
		}
		if v.DeviceID == "" || v.SessionID == "" {
			return nil
		}
		return &ParsedUserID{
			DeviceID:    v.DeviceID,
			AccountUUID: v.AccountUUID,
			SessionID:   v.SessionID,
			IsNewFormat: true,
		}
	}

	matches := legacyUserIDRegex.FindStringSubmatch(raw)
	if matches == nil {
		return nil
	}
	return &ParsedUserID{
		DeviceID:    matches[1],
		AccountUUID: matches[2],
		SessionID:   matches[3],
		IsNewFormat: false,
	}
}

// FormatMetadataUserID emits the format expected by the given Claude Code
// version, defaulting to the legacy format when the version is unknown.
func FormatMetadataUserID(deviceID, accountUUID, sessionID, uaVersion string) string {
	if IsNewMetadataFormatVersion(uaVersion) {
		b, _ := json.Marshal(jsonUserID{
			DeviceID:    deviceID,
			AccountUUID: accountUUID,
			SessionID:   sessionID,
		})
		return string(b)
	}
	return "user_" + deviceID + "_account_" + accountUUID + "_session_" + sessionID
}

func IsNewMetadataFormatVersion(version string) bool {
	if version == "" {
		return false
	}
	return CompareVersions(version, NewMetadataFormatMinVersion) >= 0
}

// ExtractCLIVersion extracts the Claude Code CLI version from a User-Agent.
func ExtractCLIVersion(ua string) string {
	matches := claudeCodeUAVersionPattern.FindStringSubmatch(ua)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
