package service

import (
	"strings"
)

// headerWireCasing 定义每个白名单 header 在真实 Claude CLI 抓包中的准确大小写。
// Go 的 HTTP server 解析请求时会将所有 header key 转为 Canonical 形式（如 x-app → X-App），
// 此 map 用于在转发时恢复到真实的 wire format。
//
// 来源：对真实 Claude CLI (claude-cli/2.1.81) 到 api.anthropic.com 的 HTTPS 流量抓包。
var headerWireCasing = map[string]string{
	// Title case
	"accept":     "Accept",
	"user-agent": "User-Agent",

	// X-Stainless-* 保持 SDK 原始大小写
	"x-stainless-retry-count":     "X-Stainless-Retry-Count",
	"x-stainless-timeout":         "X-Stainless-Timeout",
	"x-stainless-lang":            "X-Stainless-Lang",
	"x-stainless-package-version": "X-Stainless-Package-Version",
	"x-stainless-os":              "X-Stainless-OS",
	"x-stainless-arch":            "X-Stainless-Arch",
	"x-stainless-runtime":         "X-Stainless-Runtime",
	"x-stainless-runtime-version": "X-Stainless-Runtime-Version",
	"x-stainless-helper-method":   "x-stainless-helper-method",

	// Anthropic SDK 自身设置的 header，全小写
	"anthropic-dangerous-direct-browser-access": "anthropic-dangerous-direct-browser-access",
	"anthropic-version":                         "anthropic-version",
	"anthropic-beta":                            "anthropic-beta",
	"x-app":                                     "x-app",
	"content-type":                              "content-type",
	"accept-language":                           "accept-language",
	"sec-fetch-mode":                            "sec-fetch-mode",
	"accept-encoding":                           "accept-encoding",
	"authorization":                             "authorization",

	// Claude Code 2.1.87+ 新增 header
	"x-claude-code-session-id": "X-Claude-Code-Session-Id",
	"x-client-request-id":      "x-client-request-id",
	"content-length":           "content-length",
}

// resolveWireCasing 将 Go canonical key（如 X-Stainless-Os）映射为真实 wire casing（如 X-Stainless-OS）。
// 如果 map 中没有对应条目，返回原始 key 不变。
func resolveWireCasing(key string) string {
	if wk, ok := headerWireCasing[strings.ToLower(key)]; ok {
		return wk
	}
	return key
}
