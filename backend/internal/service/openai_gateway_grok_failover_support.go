package service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// failoverOpenAIUpstreamHTTPError centralizes the pre-response failover decision
// used by the Grok Responses, Messages, Chat Completions, and WebSocket bridges.
// Grok content-policy refusals are request-scoped and therefore deliberately do
// not consume or cool down another account.
func (s *OpenAIGatewayService) failoverOpenAIUpstreamHTTPError(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	respBody []byte,
	upstreamMsg string,
	upstreamModel string,
) *UpstreamFailoverError {
	if s == nil || account == nil || resp == nil {
		return nil
	}

	shouldFailover := s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody)
	tempUnscheduled := false
	if account.Platform == PlatformGrok {
		shouldFailover = s.shouldFailoverGrokUpstreamError(resp.StatusCode, respBody)
		s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
	} else if !shouldFailover && c != nil && !IsResponseCommitted(c) && s.rateLimitService != nil {
		tempUnscheduled = s.rateLimitService.CheckErrorPolicy(ctx, account, resp.StatusCode, respBody) == ErrorPolicyTempUnscheduled
		shouldFailover = tempUnscheduled
	}
	if !shouldFailover {
		return nil
	}
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(respBody), maxBytes)
	}

	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               "failover",
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})

	shouldDisable := tempUnscheduled
	if account.Platform != PlatformGrok && !tempUnscheduled && s.rateLimitService != nil {
		shouldDisable = s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
	}
	return &UpstreamFailoverError{
		StatusCode:             resp.StatusCode,
		ResponseBody:           respBody,
		ResponseHeaders:        resp.Header.Clone(),
		RetryableOnSameAccount: allowOpenAISameAccountRetry(isTransientOpenAIOAuth429(account, resp.StatusCode, resp.Header, respBody) || (!shouldDisable && account.IsPoolMode() && (account.IsPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody))), respBody),
	}
}
