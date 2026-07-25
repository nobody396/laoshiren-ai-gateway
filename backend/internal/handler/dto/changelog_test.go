package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPublicChangelogDTOExcludesInternalGitAndWorkflowFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	sha := "abcdef1234567"
	prURL := "https://github.com/example/project/pull/42"
	entry := &service.ChangelogEntry{
		ID:             1,
		Slug:           "public-update",
		Title:          "Public update",
		Summary:        "A public summary",
		Rationale:      "A public rationale",
		Content:        "Public content",
		Category:       service.ChangelogCategoryFeature,
		Status:         service.ChangelogStatusPublished,
		PublishedAt:    &now,
		CommitSHA:      &sha,
		PullRequestURL: &prURL,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	raw, err := json.Marshal(PublicChangelogFromService(entry))

	require.NoError(t, err)
	body := string(raw)
	require.NotContains(t, body, "commit_sha")
	require.NotContains(t, body, "pull_request_url")
	require.NotContains(t, body, `"status"`)
	require.NotContains(t, body, "created_by")
	require.NotContains(t, body, "updated_by")
	require.Contains(t, body, `"slug":"public-update"`)
}
