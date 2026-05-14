package dto

import "github.com/bozhouDev/DragonCode-sub2api/internal/service"

type NotifyEmailEntry struct {
	Email    string `json:"email"`
	Disabled bool   `json:"disabled"`
	Verified bool   `json:"verified"`
}

func NotifyEmailEntriesFromService(entries []service.NotifyEmailEntry) []NotifyEmailEntry {
	if entries == nil {
		return nil
	}
	result := make([]NotifyEmailEntry, len(entries))
	for i, entry := range entries {
		result[i] = NotifyEmailEntry{
			Email:    entry.Email,
			Disabled: entry.Disabled,
			Verified: entry.Verified,
		}
	}
	return result
}

func NotifyEmailEntriesToService(entries []NotifyEmailEntry) []service.NotifyEmailEntry {
	if entries == nil {
		return nil
	}
	result := make([]service.NotifyEmailEntry, len(entries))
	for i, entry := range entries {
		result[i] = service.NotifyEmailEntry{
			Email:    entry.Email,
			Disabled: entry.Disabled,
			Verified: entry.Verified,
		}
	}
	return result
}
