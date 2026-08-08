package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// proxyGrokResponsesWebSocketHTTPBridge adapts the client-facing Responses WS
// v2 turn protocol to xAI's HTTP/SSE Responses endpoint. Grok accounts must
// never be sent to the OpenAI upstream WebSocket URL.
func (s *OpenAIGatewayService) proxyGrokResponsesWebSocketHTTPBridge(
	ctx context.Context,
	c *gin.Context,
	clientConn *coderws.Conn,
	account *Account,
	token string,
	firstClientMessage []byte,
	hooks *OpenAIWSIngressHooks,
) error {
	payload := append([]byte(nil), firstClientMessage...)
	turn := 1
	fallbackModel := ""
	if hooks != nil {
		fallbackModel = strings.TrimSpace(hooks.InitialRequestModel)
	}

	for {
		if hooks != nil && hooks.BeforeTurn != nil {
			if err := hooks.BeforeTurn(turn); err != nil {
				return err
			}
		}
		result, err := s.proxyGrokResponsesWebSocketHTTPBridgeTurn(ctx, c, clientConn, account, token, payload, fallbackModel, turn, hooks)
		if hooks != nil && hooks.AfterTurn != nil {
			hooks.AfterTurn(turn, result, err)
		}
		if err != nil {
			return err
		}
		if result != nil && strings.TrimSpace(result.Model) != "" {
			fallbackModel = strings.TrimSpace(result.Model)
		}

		msgType, next, readErr := clientConn.Read(ctx)
		if readErr != nil {
			status := coderws.CloseStatus(readErr)
			if status == coderws.StatusNormalClosure || status == coderws.StatusGoingAway || errors.Is(readErr, context.Canceled) {
				return nil
			}
			return readErr
		}
		if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
			return NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "unsupported websocket message type", nil)
		}
		payload = next
		turn++
	}
}

func (s *OpenAIGatewayService) proxyGrokResponsesWebSocketHTTPBridgeTurn(
	ctx context.Context,
	c *gin.Context,
	clientConn *coderws.Conn,
	account *Account,
	token string,
	payload []byte,
	fallbackModel string,
	turn int,
	hooks *OpenAIWSIngressHooks,
) (*OpenAIForwardResult, error) {
	if !gjson.ValidBytes(payload) {
		return nil, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", nil)
	}
	eventType := strings.TrimSpace(gjson.GetBytes(payload, "type").String())
	if eventType != "" && eventType != "response.create" {
		return nil, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "unsupported websocket request type: "+eventType, nil)
	}
	originalModel := strings.TrimSpace(gjson.GetBytes(payload, "model").String())
	if originalModel == "" {
		originalModel = strings.TrimSpace(fallbackModel)
	}
	if originalModel == "" {
		return nil, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "model is required in response.create payload", nil)
	}
	mappedModel := originalModel
	if hooks != nil && hooks.MapRequestModel != nil {
		mapped, err := hooks.MapRequestModel(turn, originalModel)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(mapped) != "" {
			mappedModel = strings.TrimSpace(mapped)
		}
	}

	var request map[string]any
	if err := json.Unmarshal(payload, &request); err != nil || request == nil {
		return nil, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", err)
	}
	request["model"] = mappedModel
	bridgePayload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	return s.proxyOpenAIWSHTTPBridgeTurn(
		ctx,
		c,
		account,
		token,
		bridgePayload,
		len(bridgePayload),
		originalModel,
		"",
		"",
		"",
		"",
		turn,
		func(message []byte) error {
			return clientConn.Write(ctx, coderws.MessageText, message)
		},
	)
}
