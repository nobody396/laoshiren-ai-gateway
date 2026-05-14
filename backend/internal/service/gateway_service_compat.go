package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

// ResolveChannelMappingAndRestrict keeps handler-facing gateway compatibility.
func (s *GatewayService) ResolveChannelMappingAndRestrict(ctx context.Context, groupID *int64, model string) (ChannelMappingResult, bool) {
	if s.channelService == nil {
		return ChannelMappingResult{MappedModel: model}, false
	}
	return s.channelService.ResolveChannelMappingAndRestrict(ctx, groupID, model)
}

// ReplaceModelInBody exposes the shared model replacement helper to handlers.
func (s *GatewayService) ReplaceModelInBody(body []byte, newModel string) []byte {
	return s.replaceModelInBody(body, newModel)
}

// SetOpsUpstreamError exposes ops upstream error recording to handlers.
func SetOpsUpstreamError(c *gin.Context, upstreamStatusCode int, upstreamMessage, upstreamDetail string) {
	setOpsUpstreamError(c, upstreamStatusCode, upstreamMessage, upstreamDetail)
}
