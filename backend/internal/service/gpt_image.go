package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/responseheaders"
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	gptImageOnlyModel         = "gpt-image-2"
	gptImageGenerationsPath   = "/v1/images/generations"
	gptImageDefaultResolution = "1k"
	gptImageMaxReferenceCount = 16
	gptImageDefaultMaxDataURI = int64(20 * 1024 * 1024)
)

var (
	validGPTImageSizes = map[string]struct{}{
		"auto": {}, "1:1": {}, "3:2": {}, "2:3": {}, "4:3": {}, "3:4": {}, "5:4": {}, "4:5": {},
		"16:9": {}, "9:16": {}, "2:1": {}, "1:2": {}, "21:9": {}, "9:21": {},
	}
	gptImage4KSizes = map[string]struct{}{
		"16:9": {}, "9:16": {}, "2:1": {}, "1:2": {}, "21:9": {}, "9:21": {},
	}
	gptImageTaskUsageStore = newGPTImagePendingTaskStore()
)

type GPTImageRequest struct {
	Model          string
	Prompt         string
	N              int
	Size           string
	Resolution     string
	Quality        string
	ResponseFormat string
	Body           []byte
}

func (r *GPTImageRequest) StickySessionSeed() string {
	if r == nil {
		return ""
	}
	return strings.Join([]string{
		"gpt-image",
		strings.TrimSpace(r.Model),
		strings.TrimSpace(r.Size),
		strings.TrimSpace(r.Resolution),
		strings.TrimSpace(r.Quality),
		strings.TrimSpace(r.Prompt),
	}, "|")
}

func (s *OpenAIGatewayService) ParseGPTImageRequest(body []byte) (*GPTImageRequest, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}
	if !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("failed to parse request body")
	}

	if streamResult := gjson.GetBytes(body, "stream"); streamResult.Exists() && streamResult.Bool() {
		return nil, fmt.Errorf("image streaming is not supported")
	}

	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		model = gptImageOnlyModel
	}
	if !strings.EqualFold(model, gptImageOnlyModel) {
		return nil, fmt.Errorf("gpt-image endpoint only supports model %q", gptImageOnlyModel)
	}

	if nResult := gjson.GetBytes(body, "n"); nResult.Exists() {
		if nResult.Type != gjson.Number {
			return nil, fmt.Errorf("invalid n field type")
		}
		n := int(nResult.Int())
		if n != 1 {
			return nil, fmt.Errorf("gpt-image endpoint only supports n=1")
		}
	}

	size := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "size").String()))
	if size != "" {
		if _, ok := validGPTImageSizes[size]; !ok {
			return nil, fmt.Errorf("invalid size field; gpt-image-2 supports auto, 1:1, 3:2, 2:3, 4:3, 3:4, 5:4, 4:5, 16:9, 9:16, 2:1, 1:2, 21:9, 9:21")
		}
	}

	resolution, resolutionTier, err := normalizeGPTImageResolution(gjson.GetBytes(body, "resolution"))
	if err != nil {
		return nil, err
	}
	if resolution == "4k" {
		if _, ok := gptImage4KSizes[size]; !ok {
			return nil, fmt.Errorf("resolution 4k only supports size 16:9, 9:16, 2:1, 1:2, 21:9, 9:21")
		}
	}
	if err := s.validateGPTImageReferenceImages(body); err != nil {
		return nil, err
	}

	normalizedBody, err := sjson.SetBytes(body, "model", gptImageOnlyModel)
	if err != nil {
		return nil, fmt.Errorf("rewrite request model: %w", err)
	}
	normalizedBody, err = sjson.SetBytes(normalizedBody, "n", 1)
	if err != nil {
		return nil, fmt.Errorf("rewrite request n: %w", err)
	}
	normalizedBody, err = sjson.SetBytes(normalizedBody, "resolution", resolution)
	if err != nil {
		return nil, fmt.Errorf("rewrite request resolution: %w", err)
	}
	if size != "" {
		normalizedBody, err = sjson.SetBytes(normalizedBody, "size", size)
		if err != nil {
			return nil, fmt.Errorf("rewrite request size: %w", err)
		}
	}

	return &GPTImageRequest{
		Model:          gptImageOnlyModel,
		Prompt:         strings.TrimSpace(gjson.GetBytes(normalizedBody, "prompt").String()),
		N:              1,
		Size:           size,
		Resolution:     resolutionTier,
		Quality:        strings.TrimSpace(gjson.GetBytes(normalizedBody, "quality").String()),
		ResponseFormat: strings.ToLower(strings.TrimSpace(gjson.GetBytes(normalizedBody, "response_format").String())),
		Body:           normalizedBody,
	}, nil
}

func (s *OpenAIGatewayService) validateGPTImageReferenceImages(body []byte) error {
	result := gjson.GetBytes(body, "image_urls")
	if !result.Exists() {
		return nil
	}
	if !result.IsArray() {
		return fmt.Errorf("image_urls must be an array")
	}

	items := result.Array()
	if len(items) > gptImageMaxReferenceCount {
		return fmt.Errorf("image_urls exceeds max %d", gptImageMaxReferenceCount)
	}

	maxDataURIBytes := gptImageDefaultMaxDataURI
	if s != nil && s.cfg != nil && s.cfg.Gateway.GPTImageS3.MaxImageBytes > 0 {
		maxDataURIBytes = s.cfg.Gateway.GPTImageS3.MaxImageBytes
	}
	for idx, item := range items {
		if item.Type != gjson.String {
			return fmt.Errorf("image_urls[%d] must be a string", idx)
		}
		value := strings.TrimSpace(item.String())
		if value == "" {
			return fmt.Errorf("image_urls[%d] must not be empty", idx)
		}
		if strings.HasPrefix(strings.ToLower(value), "data:") {
			if err := validateGPTImageDataURI(value, maxDataURIBytes); err != nil {
				return fmt.Errorf("invalid image_urls[%d]: %w", idx, err)
			}
			continue
		}
		if err := validateGPTImageReferenceURL(value); err != nil {
			return fmt.Errorf("invalid image_urls[%d]: %w", idx, err)
		}
	}
	return nil
}

func validateGPTImageReferenceURL(raw string) error {
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{AllowPrivate: false})
	if err != nil {
		return err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return err
	}
	if err := urlvalidator.ValidateResolvedIP(parsed.Hostname()); err != nil {
		return err
	}
	return nil
}

func validateGPTImageDataURI(raw string, maxBytes int64) error {
	header, payload, ok := strings.Cut(raw, ",")
	if !ok || strings.TrimSpace(payload) == "" {
		return fmt.Errorf("data URI must include base64 payload")
	}
	header = strings.ToLower(strings.TrimSpace(header))
	if !strings.HasPrefix(header, "data:image/") || !strings.Contains(header, ";base64") {
		return fmt.Errorf("data URI must be image/* base64")
	}

	mimeType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
	switch mimeType {
	case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif":
	default:
		return fmt.Errorf("unsupported data URI image type: %s", mimeType)
	}

	if maxBytes > 0 && maxBytes < int64(len(payload)) && int64(len(payload)) > int64(base64.StdEncoding.EncodedLen(int(maxBytes)))+4 {
		return fmt.Errorf("data URI image exceeds max %d bytes", maxBytes)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		var rawErr error
		decoded, rawErr = base64.RawStdEncoding.DecodeString(payload)
		if rawErr != nil {
			return fmt.Errorf("invalid base64 payload")
		}
	}
	if maxBytes > 0 && int64(len(decoded)) > maxBytes {
		return fmt.Errorf("data URI image exceeds max %d bytes", maxBytes)
	}
	return nil
}

func normalizeGPTImageResolution(result gjson.Result) (string, string, error) {
	resolution := gptImageDefaultResolution
	if result.Exists() {
		if result.Type != gjson.String {
			return "", "", fmt.Errorf("invalid resolution field type")
		}
		resolution = strings.ToLower(strings.TrimSpace(result.String()))
	}
	switch resolution {
	case "1k":
		return "1k", "1K", nil
	case "2k":
		return "2k", "2K", nil
	case "4k":
		return "4k", "4K", nil
	default:
		return "", "", fmt.Errorf("resolution must be one of 1k, 2k, 4k")
	}
}

type GPTImagePendingTaskUsage struct {
	TaskID             string
	APIKeyID           int64
	UserID             int64
	Account            Account
	Model              string
	UpstreamModel      string
	Resolution         string
	Size               string
	ImageCount         int
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	RequestPayloadHash string
	CreatedAt          time.Time
	billingClaimed     bool
	billed             bool
}

type gptImagePendingTaskStore struct {
	mu    sync.Mutex
	tasks map[string]*GPTImagePendingTaskUsage
}

func newGPTImagePendingTaskStore() *gptImagePendingTaskStore {
	return &gptImagePendingTaskStore{tasks: make(map[string]*GPTImagePendingTaskUsage)}
}

func (s *gptImagePendingTaskStore) put(task *GPTImagePendingTaskUsage) {
	if task == nil || strings.TrimSpace(task.TaskID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for taskID, existing := range s.tasks {
		if existing == nil || now.Sub(existing.CreatedAt) > 48*time.Hour {
			delete(s.tasks, taskID)
		}
	}
	task.CreatedAt = now
	s.tasks[task.TaskID] = task
}

func (s *gptImagePendingTaskStore) get(taskID string) (*GPTImagePendingTaskUsage, bool) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok || task == nil {
		return nil, false
	}
	copied := *task
	return &copied, true
}

func (s *gptImagePendingTaskStore) claimBilling(taskID string) (*GPTImagePendingTaskUsage, bool) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok || task == nil || task.billed || task.billingClaimed {
		return nil, false
	}
	task.billingClaimed = true
	copied := *task
	return &copied, true
}

func (s *gptImagePendingTaskStore) markBilled(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.billed = true
		task.billingClaimed = false
	}
}

func (s *gptImagePendingTaskStore) releaseBillingClaim(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil && !task.billed {
		task.billingClaimed = false
	}
}

func (s *gptImagePendingTaskStore) remove(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, taskID)
}

func (s *OpenAIGatewayService) RegisterGPTImagePendingTask(task *GPTImagePendingTaskUsage) {
	if s != nil && s.gptImageTaskRepo != nil && task != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.gptImageTaskRepo.UpsertSubmitted(ctx, &GPTImageTask{
			TaskID:             task.TaskID,
			UserID:             task.UserID,
			APIKeyID:           task.APIKeyID,
			AccountID:          task.Account.ID,
			Model:              task.Model,
			UpstreamModel:      task.UpstreamModel,
			Resolution:         task.Resolution,
			Size:               task.Size,
			ImageCount:         task.ImageCount,
			InboundEndpoint:    task.InboundEndpoint,
			UpstreamEndpoint:   task.UpstreamEndpoint,
			UserAgent:          task.UserAgent,
			IPAddress:          task.IPAddress,
			RequestPayloadHash: task.RequestPayloadHash,
		})
	}
	gptImageTaskUsageStore.put(task)
}

func (s *OpenAIGatewayService) GetGPTImagePendingTask(taskID string) (*GPTImagePendingTaskUsage, bool) {
	if s != nil && s.gptImageTaskRepo != nil {
		task, err := s.gptImageTaskRepo.GetByTaskID(context.Background(), taskID)
		if err == nil && task != nil {
			account := Account{ID: task.AccountID, Platform: PlatformGPTImage, Type: AccountTypeUpstream}
			if s.accountRepo != nil {
				if fresh, getErr := s.accountRepo.GetByID(context.Background(), task.AccountID); getErr == nil && fresh != nil {
					account = *fresh
				}
			}
			return &GPTImagePendingTaskUsage{
				TaskID:             task.TaskID,
				APIKeyID:           task.APIKeyID,
				UserID:             task.UserID,
				Account:            account,
				Model:              task.Model,
				UpstreamModel:      task.UpstreamModel,
				Resolution:         task.Resolution,
				Size:               task.Size,
				ImageCount:         task.ImageCount,
				InboundEndpoint:    task.InboundEndpoint,
				UpstreamEndpoint:   task.UpstreamEndpoint,
				UserAgent:          task.UserAgent,
				IPAddress:          task.IPAddress,
				RequestPayloadHash: task.RequestPayloadHash,
			}, true
		}
	}
	return gptImageTaskUsageStore.get(taskID)
}

func (s *OpenAIGatewayService) ClaimGPTImageTaskBilling(taskID string) (*GPTImagePendingTaskUsage, bool) {
	if s != nil && s.gptImageTaskRepo != nil {
		task, ok, err := s.gptImageTaskRepo.TryClaimBilling(context.Background(), taskID)
		if err == nil && ok && task != nil {
			account := Account{ID: task.AccountID, Platform: PlatformGPTImage, Type: AccountTypeUpstream}
			if s.accountRepo != nil {
				if fresh, getErr := s.accountRepo.GetByID(context.Background(), task.AccountID); getErr == nil && fresh != nil {
					account = *fresh
				}
			}
			return &GPTImagePendingTaskUsage{
				TaskID:             task.TaskID,
				APIKeyID:           task.APIKeyID,
				UserID:             task.UserID,
				Account:            account,
				Model:              task.Model,
				UpstreamModel:      task.UpstreamModel,
				Resolution:         task.Resolution,
				Size:               task.Size,
				ImageCount:         task.ImageCount,
				InboundEndpoint:    task.InboundEndpoint,
				UpstreamEndpoint:   task.UpstreamEndpoint,
				UserAgent:          task.UserAgent,
				IPAddress:          task.IPAddress,
				RequestPayloadHash: task.RequestPayloadHash,
			}, true
		}
		return nil, false
	}
	return gptImageTaskUsageStore.claimBilling(taskID)
}

func (s *OpenAIGatewayService) MarkGPTImageTaskBilled(taskID string) {
	if s != nil && s.gptImageTaskRepo != nil {
		_ = s.gptImageTaskRepo.MarkBilled(context.Background(), taskID)
		return
	}
	gptImageTaskUsageStore.markBilled(taskID)
}

func (s *OpenAIGatewayService) ReleaseGPTImageTaskBillingClaim(taskID string) {
	if s != nil && s.gptImageTaskRepo != nil {
		_ = s.gptImageTaskRepo.ReleaseBillingClaim(context.Background(), taskID, "record usage failed")
		return
	}
	gptImageTaskUsageStore.releaseBillingClaim(taskID)
}

func (s *OpenAIGatewayService) RemoveGPTImagePendingTask(taskID string) {
	gptImageTaskUsageStore.remove(taskID)
}

func supportsGPTImageAccount(account *Account) bool {
	if account == nil {
		return false
	}
	return account.Platform == PlatformGPTImage && account.Type == AccountTypeUpstream
}

func (s *OpenAIGatewayService) listSchedulableGPTImageAccounts(ctx context.Context, groupID *int64) ([]Account, error) {
	if s.schedulerSnapshot != nil {
		accounts, _, err := s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformGPTImage, false)
		return accounts, err
	}

	var (
		accounts []Account
		err      error
	)
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformGPTImage)
	} else if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformGPTImage)
	} else {
		accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformGPTImage)
	}
	if err != nil {
		return nil, fmt.Errorf("query gpt-image accounts failed: %w", err)
	}
	return accounts, nil
}

func (s *OpenAIGatewayService) resolveFreshSchedulableGPTImageAccount(ctx context.Context, account *Account, requestedModel string) *Account {
	if account == nil {
		return nil
	}

	fresh := account
	if s.schedulerSnapshot != nil {
		current, err := s.getSchedulableAccount(ctx, account.ID)
		if err != nil || current == nil {
			return nil
		}
		fresh = current
	}

	if !fresh.IsSchedulable() || !supportsGPTImageAccount(fresh) {
		return nil
	}
	if requestedModel != "" && !fresh.IsModelSupported(requestedModel) {
		return nil
	}
	return fresh
}

func (s *OpenAIGatewayService) recheckSelectedGPTImageAccountFromDB(ctx context.Context, account *Account, requestedModel string) *Account {
	if account == nil {
		return nil
	}
	if s.schedulerSnapshot == nil || s.accountRepo == nil {
		return account
	}

	latest, err := s.accountRepo.GetByID(ctx, account.ID)
	if err != nil || latest == nil {
		return nil
	}
	if !latest.IsSchedulable() || !supportsGPTImageAccount(latest) {
		return nil
	}
	if requestedModel != "" && !latest.IsModelSupported(requestedModel) {
		return nil
	}
	return latest
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForGPTImage(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
) (*AccountSelectionResult, error) {
	cfg := s.schedulingConfig()
	var stickyAccountID int64
	if sessionHash != "" && s.cache != nil {
		if accountID, err := s.getStickySessionAccountID(ctx, groupID, sessionHash); err == nil {
			stickyAccountID = accountID
		}
	}

	if s.concurrencyService == nil || !cfg.LoadBatchEnabled {
		accounts, err := s.listSchedulableGPTImageAccounts(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if len(accounts) == 0 {
			return nil, ErrNoAvailableAccounts
		}

		for i := range accounts {
			acc := &accounts[i]
			if excludedIDs != nil {
				if _, excluded := excludedIDs[acc.ID]; excluded {
					continue
				}
			}
			fresh := s.resolveFreshSchedulableGPTImageAccount(ctx, acc, requestedModel)
			if fresh == nil {
				continue
			}
			result, err := s.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
			if err == nil && result.Acquired {
				if sessionHash != "" {
					_ = s.setStickySessionAccountID(ctx, groupID, sessionHash, fresh.ID, openaiStickySessionTTL)
				}
				return s.newSelectionResult(ctx, fresh, true, result.ReleaseFunc, nil)
			}
			return s.newSelectionResult(ctx, fresh, false, nil, &AccountWaitPlan{
				AccountID:      fresh.ID,
				MaxConcurrency: fresh.Concurrency,
				Timeout:        cfg.FallbackWaitTimeout,
				MaxWaiting:     cfg.FallbackMaxWaiting,
			})
		}
		return nil, ErrNoAvailableAccounts
	}

	accounts, err := s.listSchedulableGPTImageAccounts(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrNoAvailableAccounts
	}

	isExcluded := func(accountID int64) bool {
		if excludedIDs == nil {
			return false
		}
		_, excluded := excludedIDs[accountID]
		return excluded
	}

	if sessionHash != "" {
		accountID := stickyAccountID
		if accountID > 0 && !isExcluded(accountID) {
			account, err := s.getSchedulableAccount(ctx, accountID)
			if err == nil {
				clearSticky := shouldClearStickySession(account, requestedModel)
				if clearSticky {
					_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
				}
				if !clearSticky && account.IsSchedulable() && supportsGPTImageAccount(account) &&
					(requestedModel == "" || account.IsModelSupported(requestedModel)) {
					account = s.recheckSelectedGPTImageAccountFromDB(ctx, account, requestedModel)
					if account == nil {
						_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
					} else {
						result, err := s.tryAcquireAccountSlot(ctx, accountID, account.Concurrency)
						if err == nil && result.Acquired {
							_ = s.refreshStickySessionTTL(ctx, groupID, sessionHash, openaiStickySessionTTL)
							return s.newSelectionResult(ctx, account, true, result.ReleaseFunc, nil)
						}

						waitingCount, _ := s.concurrencyService.GetAccountWaitingCount(ctx, accountID)
						if waitingCount < cfg.StickySessionMaxWaiting {
							return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
								AccountID:      accountID,
								MaxConcurrency: account.Concurrency,
								Timeout:        cfg.StickySessionWaitTimeout,
								MaxWaiting:     cfg.StickySessionMaxWaiting,
							})
						}
					}
				}
			}
		}
	}

	candidates := make([]*Account, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if isExcluded(acc.ID) || !acc.IsSchedulable() || !supportsGPTImageAccount(acc) {
			continue
		}
		if requestedModel != "" && !acc.IsModelSupported(requestedModel) {
			continue
		}
		candidates = append(candidates, acc)
	}
	if len(candidates) == 0 {
		return nil, ErrNoAvailableAccounts
	}

	accountLoads := make([]AccountWithConcurrency, 0, len(candidates))
	for _, acc := range candidates {
		accountLoads = append(accountLoads, AccountWithConcurrency{
			ID:             acc.ID,
			MaxConcurrency: acc.EffectiveLoadFactor(),
		})
	}

	loadMap, err := s.concurrencyService.GetAccountsLoadBatch(ctx, accountLoads)
	if err != nil {
		ordered := append([]*Account(nil), candidates...)
		sortAccountsByPriorityAndLastUsed(ordered, false)
		for _, acc := range ordered {
			fresh := s.resolveFreshSchedulableGPTImageAccount(ctx, acc, requestedModel)
			if fresh == nil {
				continue
			}
			result, err := s.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
			if err == nil && result.Acquired {
				if sessionHash != "" {
					_ = s.setStickySessionAccountID(ctx, groupID, sessionHash, fresh.ID, openaiStickySessionTTL)
				}
				return s.newSelectionResult(ctx, fresh, true, result.ReleaseFunc, nil)
			}
		}
	} else {
		var available []accountWithLoad
		for _, acc := range candidates {
			loadInfo := loadMap[acc.ID]
			if loadInfo == nil {
				loadInfo = &AccountLoadInfo{AccountID: acc.ID}
			}
			if loadInfo.LoadRate < 100 {
				available = append(available, accountWithLoad{
					account:  acc,
					loadInfo: loadInfo,
				})
			}
		}

		if len(available) > 0 {
			sort.SliceStable(available, func(i, j int) bool {
				a, b := available[i], available[j]
				if a.account.Priority != b.account.Priority {
					return a.account.Priority < b.account.Priority
				}
				if a.loadInfo.LoadRate != b.loadInfo.LoadRate {
					return a.loadInfo.LoadRate < b.loadInfo.LoadRate
				}
				switch {
				case a.account.LastUsedAt == nil && b.account.LastUsedAt != nil:
					return true
				case a.account.LastUsedAt != nil && b.account.LastUsedAt == nil:
					return false
				case a.account.LastUsedAt == nil && b.account.LastUsedAt == nil:
					return false
				default:
					return a.account.LastUsedAt.Before(*b.account.LastUsedAt)
				}
			})
			shuffleWithinSortGroups(available)

			for _, item := range available {
				fresh := s.resolveFreshSchedulableGPTImageAccount(ctx, item.account, requestedModel)
				if fresh == nil {
					continue
				}
				result, err := s.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
				if err == nil && result.Acquired {
					if sessionHash != "" {
						_ = s.setStickySessionAccountID(ctx, groupID, sessionHash, fresh.ID, openaiStickySessionTTL)
					}
					return s.newSelectionResult(ctx, fresh, true, result.ReleaseFunc, nil)
				}
			}
		}
	}

	sortAccountsByPriorityAndLastUsed(candidates, false)
	for _, acc := range candidates {
		fresh := s.resolveFreshSchedulableGPTImageAccount(ctx, acc, requestedModel)
		if fresh == nil {
			continue
		}
		return s.newSelectionResult(ctx, fresh, false, nil, &AccountWaitPlan{
			AccountID:      fresh.ID,
			MaxConcurrency: fresh.Concurrency,
			Timeout:        cfg.FallbackWaitTimeout,
			MaxWaiting:     cfg.FallbackMaxWaiting,
		})
	}

	return nil, ErrNoAvailableAccounts
}

func (s *OpenAIGatewayService) ForwardGPTImage(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *GPTImageRequest,
) (*OpenAIForwardResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed gpt-image request is required")
	}
	if !supportsGPTImageAccount(account) {
		return nil, fmt.Errorf("selected account does not support gpt-image")
	}

	startTime := time.Now()
	upstreamModel := account.GetMappedModel(parsed.Model)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = parsed.Model
	}

	forwardBody, err := sjson.SetBytes(parsed.Body, "model", upstreamModel)
	if err != nil {
		return nil, fmt.Errorf("rewrite gpt-image request model: %w", err)
	}
	setOpsUpstreamRequestBody(c, forwardBody)

	req, err := s.buildGPTImageRequest(ctx, c, account, forwardBody)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
		return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
			}
		}

		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		safeErr := SafeClientUpstreamError(resp.StatusCode)
		MarkResponseCommitted(c)
		c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("upstream error: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", upstreamMsg)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	taskIDs := extractGPTImageSubmittedTaskIDs(respBody)
	imageCount := 0
	if len(taskIDs) == 0 {
		imageCount = extractOpenAIImageCountFromJSONBytes(respBody)
	}
	if len(taskIDs) == 0 && imageCount <= 0 {
		usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
		if !openAIImageResponseMayAlreadyBeBillable(respBody, usage) {
			return nil, &UpstreamFailoverError{
				StatusCode:             http.StatusBadGateway,
				RequestScopedTransient: true,
				Stage:                  GatewayFailureStageInference,
				Scope:                  GatewayFailureScopeRequest,
				Reason:                 GatewayFailureReason("gpt_image_no_output"),
				NextAccountAction:      NextAccountRetry,
				ClientStatusCode:       http.StatusBadGateway,
				ClientMessage:          "Upstream image generation did not produce an image",
			}
		}
		safeErr := SafeClientUpstreamError(http.StatusBadGateway)
		c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		return nil, fmt.Errorf("gpt-image response contained no valid image output")
	}
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)

	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Model:           parsed.Model,
		UpstreamModel:   upstreamModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		ImageCount:      imageCount,
		ImageSize:       parsed.Resolution,
		GPTImageTaskIDs: taskIDs,
	}, nil
}

func (s *OpenAIGatewayService) ForwardGPTImageTask(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	taskID string,
) (*OpenAIForwardResult, error) {
	result, respHeader, err := s.fetchGPTImageTask(ctx, c, account, taskID)
	if respHeader != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), respHeader, s.responseHeaderFilter)
		c.Writer.Header().Del("Content-Length")
	}
	if err != nil {
		if result != nil && result.ResponseStatus >= 400 {
			MarkResponseCommitted(c)
			c.Data(result.ResponseStatus, result.ResponseType, result.ResponseBody)
		}
		return nil, err
	}
	return result, nil
}

func (s *OpenAIGatewayService) FetchGPTImageTask(
	ctx context.Context,
	account *Account,
	taskID string,
) (*OpenAIForwardResult, error) {
	result, _, err := s.fetchGPTImageTask(ctx, nil, account, taskID)
	return result, err
}

func (s *OpenAIGatewayService) fetchGPTImageTask(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	taskID string,
) (*OpenAIForwardResult, http.Header, error) {
	if !supportsGPTImageAccount(account) {
		return nil, nil, fmt.Errorf("selected account does not support gpt-image")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil, fmt.Errorf("task_id is required")
	}

	startTime := time.Now()
	req, err := s.buildGPTImageTaskRequest(ctx, c, account, taskID)
	if err != nil {
		return nil, nil, err
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	if resp.StatusCode >= 400 {
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("upstream error: %d", resp.StatusCode)
		}
		safeErr := SafeClientUpstreamError(resp.StatusCode)
		safeBody, _ := json.Marshal(OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		return &OpenAIForwardResult{
			RequestID:      resp.Header.Get("x-request-id"),
			Model:          gptImageOnlyModel,
			UpstreamModel:  gptImageOnlyModel,
			Duration:       time.Since(startTime),
			ResponseBody:   safeBody,
			ResponseStatus: safeErr.StatusCode,
			ResponseType:   "application/json",
		}, resp.Header.Clone(), fmt.Errorf("%s", upstreamMsg)
	}

	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(respBody, "data.status").String()))
	if status != "" && s.gptImageTaskRepo != nil {
		_ = s.gptImageTaskRepo.UpdateUpstreamStatus(ctx, taskID, status, extractGPTImageTaskErrorMessage(respBody))
	}
	imageCount := 0
	if status == "completed" {
		imageCount = extractGPTImageTaskImageCount(respBody)
		if imageCount <= 0 {
			imageCount = 1
		}
	}

	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Model:           gptImageOnlyModel,
		UpstreamModel:   gptImageOnlyModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		ImageCount:      imageCount,
		ResponseBody:    respBody,
		ResponseStatus:  resp.StatusCode,
		ResponseType:    contentType,
	}, resp.Header.Clone(), nil
}

func extractGPTImageTaskErrorMessage(body []byte) string {
	for _, path := range []string{"data.error.message", "error.message", "data.message"} {
		if msg := strings.TrimSpace(gjson.GetBytes(body, path).String()); msg != "" {
			return msg
		}
	}
	return ""
}

func (s *OpenAIGatewayService) StoreGPTImageTaskResult(ctx context.Context, taskID string, upstreamBody []byte, mediaBaseURL string) ([]byte, int, error) {
	if s == nil || s.gptImageTaskRepo == nil {
		return nil, 0, fmt.Errorf("gpt-image task repository is not configured")
	}
	task, err := s.gptImageTaskRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, 0, err
	}
	if task == nil {
		return nil, 0, fmt.Errorf("gpt-image task not found")
	}
	if task.StorageStatus == GPTImageStorageStatusStored && len(task.S3ObjectKeys) > 0 {
		return rewriteGPTImageTaskImageURLs(upstreamBody, task.TaskID, task.S3ObjectKeys, task.MediaToken, mediaBaseURL), len(task.S3ObjectKeys), nil
	}

	claimed, ok, err := s.gptImageTaskRepo.TryClaimStorage(ctx, taskID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		latest, getErr := s.gptImageTaskRepo.GetByTaskID(ctx, taskID)
		if getErr == nil && latest != nil && latest.StorageStatus == GPTImageStorageStatusStored && len(latest.S3ObjectKeys) > 0 {
			return rewriteGPTImageTaskImageURLs(upstreamBody, latest.TaskID, latest.S3ObjectKeys, latest.MediaToken, mediaBaseURL), len(latest.S3ObjectKeys), nil
		}
		return nil, 0, fmt.Errorf("gpt-image task storage is in progress")
	}
	task = claimed

	urls := extractGPTImageTaskImageURLs(upstreamBody)
	if len(urls) == 0 {
		_ = s.gptImageTaskRepo.MarkStorageFailed(ctx, taskID, "completed task response has no image urls")
		return nil, 0, fmt.Errorf("completed task response has no image urls")
	}
	if s.gptImageS3Storage == nil || !s.gptImageS3Storage.Enabled() {
		_ = s.gptImageTaskRepo.MarkStorageFailed(ctx, taskID, ErrGPTImageS3NotConfigured.Error())
		return nil, 0, ErrGPTImageS3NotConfigured
	}

	keys := make([]string, 0, len(urls))
	for i, imageURL := range urls {
		obj, uploadErr := s.gptImageS3Storage.UploadFromURL(ctx, task.UserID, task.TaskID, i, imageURL)
		if uploadErr != nil {
			_ = s.gptImageTaskRepo.MarkStorageFailed(ctx, taskID, uploadErr.Error())
			return nil, 0, uploadErr
		}
		keys = append(keys, obj.Key)
	}
	token := task.MediaToken
	if strings.TrimSpace(token) == "" {
		generated, tokenErr := generateRandomToken(24)
		if tokenErr != nil {
			_ = s.gptImageTaskRepo.MarkStorageFailed(ctx, taskID, tokenErr.Error())
			return nil, 0, tokenErr
		}
		token = generated
	}
	if err := s.gptImageTaskRepo.MarkStorageStored(ctx, taskID, keys, len(keys), token); err != nil {
		return nil, 0, err
	}
	return rewriteGPTImageTaskImageURLs(upstreamBody, task.TaskID, keys, token, mediaBaseURL), len(keys), nil
}

func (s *OpenAIGatewayService) OpenGPTImageMedia(ctx context.Context, taskID string, index int, token string) (io.ReadCloser, string, int64, error) {
	if s == nil || s.gptImageTaskRepo == nil {
		return nil, "", 0, fmt.Errorf("gpt-image task repository is not configured")
	}
	task, err := s.gptImageTaskRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, "", 0, err
	}
	if task == nil || task.StorageStatus != GPTImageStorageStatusStored || token == "" || token != task.MediaToken {
		return nil, "", 0, fmt.Errorf("gpt-image media not found")
	}
	if index < 0 || index >= len(task.S3ObjectKeys) {
		return nil, "", 0, fmt.Errorf("gpt-image media not found")
	}
	if s.gptImageS3Storage == nil {
		return nil, "", 0, ErrGPTImageS3NotConfigured
	}
	return s.gptImageS3Storage.GetObject(ctx, task.S3ObjectKeys[index])
}

func (s *OpenAIGatewayService) buildGPTImageRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*http.Request, error) {
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	targetURL := strings.TrimRight(validatedURL, "/") + gptImageGenerationsPath
	if strings.HasSuffix(strings.TrimRight(validatedURL, "/"), "/v1") {
		targetURL = strings.TrimRight(validatedURL, "/") + "/images/generations"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("api_key")))
	req.Header.Set("Content-Type", "application/json")
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if !openaiPassthroughAllowedHeaders[strings.ToLower(key)] {
				continue
			}
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	if customUA := strings.TrimSpace(account.GetCredential("user_agent")); customUA != "" {
		req.Header.Set("User-Agent", customUA)
	}
	return req, nil
}

func (s *OpenAIGatewayService) buildGPTImageTaskRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	taskID string,
) (*http.Request, error) {
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	targetURL := strings.TrimRight(validatedURL, "/") + "/v1/tasks/" + url.PathEscape(taskID)
	if strings.HasSuffix(strings.TrimRight(validatedURL, "/"), "/v1") {
		targetURL = strings.TrimRight(validatedURL, "/") + "/tasks/" + url.PathEscape(taskID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("api_key")))
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if !openaiPassthroughAllowedHeaders[strings.ToLower(key)] {
				continue
			}
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	if customUA := strings.TrimSpace(account.GetCredential("user_agent")); customUA != "" {
		req.Header.Set("User-Agent", customUA)
	}
	return req, nil
}

func extractGPTImageSubmittedTaskIDs(body []byte) []string {
	data := gjson.GetBytes(body, "data")
	if !data.Exists() || !data.IsArray() {
		return nil
	}
	taskIDs := make([]string, 0)
	data.ForEach(func(_, item gjson.Result) bool {
		status := strings.ToLower(strings.TrimSpace(item.Get("status").String()))
		taskID := strings.TrimSpace(item.Get("task_id").String())
		if taskID != "" && (status == "" || status == "submitted" || status == "processing") {
			taskIDs = append(taskIDs, taskID)
		}
		return true
	})
	return taskIDs
}

func extractGPTImageTaskImageCount(body []byte) int {
	images := gjson.GetBytes(body, "data.result.images")
	if !images.Exists() || !images.IsArray() {
		return 0
	}
	count := 0
	images.ForEach(func(_, item gjson.Result) bool {
		urls := item.Get("url")
		if urls.Exists() && urls.IsArray() {
			if n := len(urls.Array()); n > 0 {
				count += n
				return true
			}
		}
		count++
		return true
	})
	return count
}

func extractGPTImageTaskImageURLs(body []byte) []string {
	images := gjson.GetBytes(body, "data.result.images")
	if !images.Exists() || !images.IsArray() {
		return nil
	}
	urls := make([]string, 0)
	images.ForEach(func(_, item gjson.Result) bool {
		urlValue := item.Get("url")
		if urlValue.Exists() && urlValue.IsArray() {
			urlValue.ForEach(func(_, value gjson.Result) bool {
				if u := strings.TrimSpace(value.String()); u != "" {
					urls = append(urls, u)
				}
				return true
			})
			return true
		}
		if u := strings.TrimSpace(urlValue.String()); u != "" {
			urls = append(urls, u)
		}
		return true
	})
	return urls
}

func rewriteGPTImageTaskImageURLs(body []byte, taskID string, objectKeys []string, token string, mediaBaseURL string) []byte {
	if len(objectKeys) == 0 {
		return body
	}
	mediaBaseURL = strings.TrimRight(strings.TrimSpace(mediaBaseURL), "/")
	if mediaBaseURL == "" {
		mediaBaseURL = "/gpt-image/media"
	}

	result := append([]byte(nil), body...)
	nextIndex := 0
	images := gjson.GetBytes(body, "data.result.images")
	if images.Exists() && images.IsArray() {
		images.ForEach(func(key, item gjson.Result) bool {
			urlCount := 1
			if urls := item.Get("url"); urls.Exists() && urls.IsArray() && len(urls.Array()) > 0 {
				urlCount = len(urls.Array())
			}
			replacements := make([]string, 0, urlCount)
			for i := 0; i < urlCount && nextIndex < len(objectKeys); i++ {
				replacements = append(replacements, buildGPTImageMediaURL(mediaBaseURL, taskID, nextIndex, token))
				nextIndex++
			}
			if len(replacements) > 0 {
				updated, err := sjson.SetBytes(result, "data.result.images."+key.String()+".url", replacements)
				if err == nil {
					result = updated
				}
			}
			return nextIndex < len(objectKeys)
		})
	}
	return result
}

func buildGPTImageMediaURL(baseURL, taskID string, index int, token string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/%s/%d?token=%s", baseURL, url.PathEscape(taskID), index, url.QueryEscape(token))
}
