package service

import "strings"

const (
	ImageBillingSize1K = "1K"
	ImageBillingSize2K = "2K"
	ImageBillingSize4K = "4K"

	VideoBillingResolution480P  = "480p"
	VideoBillingResolution720P  = "720p"
	VideoBillingResolution1080P = "1080p"

	VideoBillingMinDurationSeconds     = 1
	VideoBillingMaxDurationSeconds     = 15
	VideoBillingDefaultDurationSeconds = 8
)

func NormalizeImageBillingTierOrDefault(size string) string {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "1k", "1024x1024", "1024x768", "768x1024":
		return ImageBillingSize1K
	case "4k", "3840x2160", "2160x3840":
		return ImageBillingSize4K
	default:
		return ImageBillingSize2K
	}
}

func NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds int) int {
	if durationSeconds <= 0 {
		return VideoBillingDefaultDurationSeconds
	}
	if durationSeconds < VideoBillingMinDurationSeconds {
		return VideoBillingMinDurationSeconds
	}
	if durationSeconds > VideoBillingMaxDurationSeconds {
		return VideoBillingMaxDurationSeconds
	}
	return durationSeconds
}

func NormalizeVideoBillingResolutionOrDefault(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "720", "720p", "hd":
		return VideoBillingResolution720P
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		return VideoBillingResolution1080P
	default:
		return VideoBillingResolution480P
	}
}
