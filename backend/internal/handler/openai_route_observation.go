package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func reportOpenAIRouteAttempt(
	gateway *service.OpenAIGatewayService,
	account *service.Account,
	groupID *int64,
	model string,
	requestClass service.OpenAIRouteRequestClass,
	routeEndpoint string,
	duration time.Duration,
	firstTokenMs *int,
	err error,
	streamStarted bool,
) {
	if gateway == nil || account == nil {
		return
	}
	signal := service.OpenAIRouteFailureSignal{StatusCode: 200}
	if err != nil {
		signal.HasError = true
		signal.StreamStarted = streamStarted
		var failoverErr *service.UpstreamFailoverError
		if errors.As(err, &failoverErr) && failoverErr != nil {
			signal.StatusCode = failoverErr.StatusCode
			signal.ErrorCode = string(failoverErr.Reason)
			signal.Message = safeOpenAIRouteFailureMessage(failoverErr)
		} else {
			signal.StatusCode = 0
			signal.LocalOrigin = true
			signal.Message = err.Error()
		}
	}
	gateway.ReportOpenAIRouteAttempt(account, groupID, model, requestClass, routeEndpoint, duration, firstTokenMs, signal)
}

func openAIRouteRequestClass(image bool) service.OpenAIRouteRequestClass {
	if image {
		return service.OpenAIRouteRequestClassImage
	}
	return service.OpenAIRouteRequestClassText
}

func openAIRouteObservationEndpoint(result *service.OpenAIForwardResult, fallback string) string {
	if result != nil && strings.TrimSpace(result.UpstreamEndpoint) != "" {
		return strings.TrimSpace(result.UpstreamEndpoint)
	}
	return fallback
}

// Only bounded, classifier-relevant text is passed to the service and nothing
// from this signal is persisted verbatim. This function must never log it.
func safeOpenAIRouteFailureMessage(failoverErr *service.UpstreamFailoverError) string {
	if failoverErr == nil {
		return ""
	}
	value := strings.TrimSpace(string(failoverErr.ResponseBody))
	if len(value) > 512 {
		value = value[:512]
	}
	return value
}
