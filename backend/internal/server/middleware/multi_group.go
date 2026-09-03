package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type MultiGroupAuthorizer interface {
	AuthorizeMultiGroupTarget(context.Context, *service.APIKey, *service.Group) (*service.User, error)
}
type MultiGroupModelCatalog func(context.Context, *service.Group) ([]string, error)

type multiGroupDeclaration struct {
	patterns   []string
	authorized bool
}

// MultiGroupRouting selects one billing group, never another model. Once a
// matching priority is found, authorization/quota failure is terminal; neither
// wallet fallback nor cross-group error fallback is implied by selecting groups.
func MultiGroupRouting(groups UniversalTargetGroupLoader, access MultiGroupAuthorizer, subscriptions *service.SubscriptionService, catalog MultiGroupModelCatalog, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := GetAPIKeyFromContext(c)
		if !ok || !key.IsMultiGroup() {
			c.Next()
			return
		}
		fail := func(status int, message string) {
			if status < 400 || status > 599 {
				status = http.StatusServiceUnavailable
			}
			if strings.Contains(c.Request.URL.Path, "/v1beta/") {
				abortWithGoogleError(c, status, message)
			} else {
				writeUniversalRouteError(c, status, "MULTI_GROUP_REQUEST_REJECTED", message)
			}
		}
		if groups == nil || access == nil || catalog == nil {
			fail(503, "Multi-group routing is unavailable")
			return
		}
		path := c.Request.URL.Path
		listing := c.Request.Method == http.MethodGet && strings.HasSuffix(path, "/models")
		if c.Request.Method == http.MethodGet && strings.HasSuffix(path, "/usage") {
			c.Next()
			return
		}
		protocol, model, err := multiGroupRequestIdentity(c.Request)
		if err != nil || (!listing && (protocol == "" || model == "")) {
			fail(400, "This multi-group key requires a supported model-bearing HTTP request; use a single-group key for realtime/media endpoints")
			return
		}
		forced, _ := GetForcePlatformFromContext(c)
		if forced == "" && strings.HasPrefix(path, "/antigravity/") {
			forced = service.PlatformAntigravity
		}
		if listing && forced == service.PlatformGPTImage {
			fail(400, "This media endpoint requires a single-group key")
			return
		}
		var discovered []string
		priorDeclarations := map[string][]multiGroupDeclaration{}
		for _, id := range key.GroupIDs {
			group, loadErr := groups.GetByID(c.Request.Context(), id)
			if loadErr != nil || group == nil {
				fail(503, "An authorized group is unavailable; review this key's group selection")
				return
			}
			if forced != "" && group.Platform != forced {
				continue
			}
			if listing {
				if !service.MultiGroupProtocolSupported(group, "") {
					continue
				}
				if strings.Contains(path, "/v1beta/") && !service.MultiGroupProtocolSupported(group, "generate_content") {
					continue
				}
			} else if !service.MultiGroupProtocolSupported(group, protocol) {
				continue
			}
			models, catalogErr := catalog(c.Request.Context(), group)
			if catalogErr != nil {
				fail(503, "Model catalog is temporarily unavailable")
				return
			}
			if !listing && !service.MultiGroupModelMatches(models, model) {
				continue
			}
			payer, authErr := access.AuthorizeMultiGroupTarget(c.Request.Context(), key, group)
			if listing {
				// Collect concrete names independently from routing decisions.
				// A later group's concrete name may be owned by an earlier
				// authorized wildcard. Resolve each name after all declarations
				// are known, using the same first-match order as requests.
				protocols := []string{"messages", "responses", "chat_completions", "generate_content"}
				if strings.Contains(path, "/v1beta/") {
					protocols = []string{"generate_content"}
				}
				for _, p := range protocols {
					if service.MultiGroupProtocolSupported(group, p) {
						priorDeclarations[p] = append(priorDeclarations[p], multiGroupDeclaration{patterns: models, authorized: authErr == nil})
					}
				}
				for _, candidate := range models {
					if !strings.Contains(candidate, "*") {
						discovered = append(discovered, candidate)
					}
				}
				continue
			}
			if authErr != nil {
				fail(403, "The selected model group is no longer authorized; review this key's group selection")
				return
			}
			var subscription *service.UserSubscription
			if cfg == nil || cfg.RunMode != config.RunModeSimple {
				if group.IsSubscriptionType() {
					if subscriptions == nil {
						fail(503, "Subscription validation is unavailable")
						return
					}
					subscription, err = subscriptions.GetActiveSubscription(c.Request.Context(), payer.ID, group.ID)
					if err != nil {
						fail(403, "No active subscription for the selected group; wallet fallback is disabled")
						return
					}
					maintenance, limitErr := subscriptions.ValidateAndCheckLimits(subscription, group)
					if limitErr != nil {
						fail(infraerrors.Code(limitErr), infraerrors.Message(limitErr))
						return
					}
					if maintenance {
						copy := *subscription
						subscriptions.DoWindowMaintenance(&copy)
					}
				} else if payer.Balance <= 0 {
					fail(403, "Insufficient account balance")
					return
				}
			}
			boundGroup := *group
			// Existing legacy fallback can cross group boundaries. Disable it for this
			// explicit multi-group contract until a price/funding-safe fallback exists.
			boundGroup.FallbackGroupID = nil
			boundGroup.FallbackGroupIDOnInvalidRequest = nil
			bound := *key
			bound.User = payer
			bound.GroupID = &boundGroup.ID
			bound.Group = &boundGroup
			c.Set(string(ContextKeyAPIKey), &bound)
			if subscription != nil {
				c.Set(string(ContextKeySubscription), subscription)
			} else {
				c.Set(string(ContextKeySubscription), nil)
			}
			setGroupContext(c, &boundGroup)
			c.Next()
			return
		}
		if listing {
			slices.Sort(discovered)
			discovered = slices.Compact(discovered)
			data := make([]gin.H, 0, len(discovered))
			google := strings.Contains(path, "/v1beta/")
			for _, name := range discovered {
				authorized := false
				for _, declarations := range priorDeclarations {
					for _, declaration := range declarations {
						if service.MultiGroupModelMatches(declaration.patterns, name) {
							authorized = authorized || declaration.authorized
							break
						}
					}
				}
				if !authorized {
					continue
				}
				if google {
					data = append(data, gin.H{"name": "models/" + name, "displayName": name, "supportedGenerationMethods": []string{"generateContent", "countTokens"}})
				} else {
					data = append(data, gin.H{"id": name, "object": "model", "created": 0, "owned_by": "laoshirenai"})
				}
			}
			if google {
				c.JSON(200, gin.H{"models": data})
			} else {
				c.JSON(200, gin.H{"object": "list", "data": data})
			}
			c.Abort()
			return
		}
		fail(404, "No authorized group declares this model for the requested protocol")
	}
}

func multiGroupRequestIdentity(r *http.Request) (string, string, error) {
	path := r.URL.Path
	if r.Method == http.MethodGet && strings.HasSuffix(path, "/models") {
		return "", "", nil
	}
	if r.Method != http.MethodPost {
		return "", "", nil
	}
	if strings.Contains(path, "/v1beta/models/") {
		tail := strings.SplitN(path, "/v1beta/models/", 2)[1]
		model, action, ok := strings.Cut(tail, ":")
		if ok && (action == "generateContent" || action == "streamGenerateContent" || action == "countTokens") {
			return "generate_content", model, nil
		}
		return "", "", nil
	}
	protocol := ""
	switch {
	case strings.HasSuffix(path, "/messages"), strings.HasSuffix(path, "/messages/count_tokens"):
		protocol = "messages"
	case strings.HasSuffix(path, "/responses"), strings.HasSuffix(path, "/responses/compact"), strings.HasSuffix(path, "/responses/input_tokens"):
		protocol = "responses"
	case strings.HasSuffix(path, "/chat/completions"):
		protocol = "chat_completions"
	default:
		return "", "", nil
	}
	original := r.Body
	body, err := io.ReadAll(original)
	_ = original.Close()
	if err != nil {
		return "", "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	model, err := multiGroupBodyModel(body)
	return protocol, strings.TrimSpace(model), err
}

// Reject duplicate top-level fields so different gateway parsers cannot disagree
// about which model was authorized while forwarding the original request bytes.
func multiGroupBodyModel(body []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", errors.New("expected JSON object")
	}
	seen := map[string]bool{}
	model := ""
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return "", err
		}
		name, ok := keyToken.(string)
		if !ok || seen[name] {
			return "", errors.New("duplicate JSON field")
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", err
		}
		if name == "model" {
			if err := json.Unmarshal(value, &model); err != nil {
				return "", err
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", errors.New("unexpected trailing JSON")
	}
	return model, nil
}
