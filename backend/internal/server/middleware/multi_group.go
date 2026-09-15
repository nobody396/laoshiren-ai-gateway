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
type MultiGroupModelCatalog func(context.Context, *service.Group) (*service.GroupModelDeclaration, error)

type multiGroupDeclaration struct {
	group      *service.Group
	catalog    *service.GroupModelDeclaration
	authorized bool
}

// Client-visible rejection codes. A 4xx names the caller's own selection so ops
// attribution books it against the client; 5xx keeps a distinct code because a
// routing or catalog outage really is ours and must still page.
const (
	MultiGroupRequestRejectedCode    = "MULTI_GROUP_REQUEST_REJECTED"
	MultiGroupRoutingUnavailableCode = "MULTI_GROUP_ROUTING_UNAVAILABLE"
)

// multiGroupDeferredKey stashes the router for a request whose model is not in
// the HTTP envelope at all. Codex Desktop opens its Responses stream with
// "GET /v1/responses" and sends the model in the first WebSocket frame, so the
// billing group can only be chosen after the handshake.
const multiGroupDeferredKey = "multi_group_deferred_router"

type multiGroupRouter struct {
	groups        UniversalTargetGroupLoader
	access        MultiGroupAuthorizer
	subscriptions *service.SubscriptionService
	catalog       MultiGroupModelCatalog
	cfg           *config.Config
	forced        string
}

func multiGroupFail(c *gin.Context, status int, message string) {
	if status < 400 || status > 599 {
		status = http.StatusServiceUnavailable
	}
	if strings.Contains(c.Request.URL.Path, "/v1beta/") {
		abortWithGoogleError(c, status, message)
		return
	}
	code := MultiGroupRequestRejectedCode
	if status >= 500 {
		code = MultiGroupRoutingUnavailableCode
	}
	writeUniversalRouteError(c, status, code, message)
}

// multiGroupDeferredRequest reports whether the model can only be known after a
// protocol handshake this middleware does not perform.
func multiGroupDeferredRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/responses")
}

// IsMultiGroupDeferred reports whether MultiGroupRouting let this request pass
// unbound so a handler can resolve the group once it knows the model.
func IsMultiGroupDeferred(c *gin.Context) bool {
	if c == nil {
		return false
	}
	_, ok := c.Get(multiGroupDeferredKey)
	return ok
}

// ResolveDeferredMultiGroup binds the billing group for a deferred request now
// that the handler has read the model off the wire. It applies exactly the same
// priority, authorization and funding rules as the HTTP path; ok=false carries a
// client-facing message and never falls through to an arbitrary group.
func ResolveDeferredMultiGroup(c *gin.Context, protocol, model string) (int, string, bool) {
	if c == nil {
		return 0, "", true
	}
	stashed, ok := c.Get(multiGroupDeferredKey)
	if !ok {
		return 0, "", true
	}
	router, ok := stashed.(*multiGroupRouter)
	if !ok || router == nil {
		return http.StatusServiceUnavailable, "Multi-group routing is unavailable", false
	}
	key, ok := GetAPIKeyFromContext(c)
	if !ok || !key.IsMultiGroup() {
		return 0, "", true
	}
	if strings.TrimSpace(model) == "" {
		return http.StatusBadRequest, "This multi-group key requires a model in the first message", false
	}
	c.Set(OpsModelKey, strings.TrimSpace(model))
	return router.selectGroup(c, key, protocol, model)
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
		fail := func(status int, message string) { multiGroupFail(c, status, message) }
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
		forced, _ := GetForcePlatformFromContext(c)
		if forced == "" && strings.HasPrefix(path, "/antigravity/") {
			forced = service.PlatformAntigravity
		}
		router := &multiGroupRouter{groups: groups, access: access, subscriptions: subscriptions, catalog: catalog, cfg: cfg, forced: forced}
		if !listing && multiGroupDeferredRequest(c.Request) {
			c.Set(multiGroupDeferredKey, router)
			c.Next()
			return
		}
		protocol, model, err := multiGroupRequestIdentity(c.Request)
		if err != nil || (!listing && (protocol == "" || model == "")) {
			fail(400, "This multi-group key requires a supported model-bearing HTTP request; use a single-group key for media and stateful endpoints")
			return
		}
		if listing && forced == service.PlatformGPTImage {
			fail(400, "This media endpoint requires a single-group key")
			return
		}
		if !listing {
			// Name the rejected model in ops logs even though no handler runs.
			c.Set(OpsModelKey, model)
			status, message, ok := router.selectGroup(c, key, protocol, model)
			if !ok {
				fail(status, message)
				return
			}
			c.Next()
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
			if !service.MultiGroupProtocolSupported(group, "") {
				continue
			}
			if strings.Contains(path, "/v1beta/") && !service.MultiGroupProtocolSupported(group, "generate_content") {
				continue
			}
			models, catalogErr := catalog(c.Request.Context(), group)
			if catalogErr != nil || models == nil {
				fail(503, "Model catalog is temporarily unavailable")
				return
			}
			_, authErr := access.AuthorizeMultiGroupTarget(c.Request.Context(), key, group)
			// Collect concrete names independently from routing decisions.
			// A later group's concrete name may be owned by an earlier
			// authorized wildcard. Resolve each name after all declarations
			// are known, using the same first-match order as requests.
			protocols := service.MultiGroupTextProtocols
			if strings.Contains(path, "/v1beta/") {
				protocols = []string{"generate_content"}
			}
			for _, p := range protocols {
				if service.MultiGroupProtocolSupported(group, p) {
					priorDeclarations[p] = append(priorDeclarations[p], multiGroupDeclaration{group: group, catalog: models, authorized: authErr == nil})
				}
			}
			for _, candidate := range models.Models {
				if !strings.Contains(candidate, "*") && !service.IsInternalOnlyModel(candidate) && !strings.EqualFold(candidate, service.OpenAIFixedImageRendererModel) {
					discovered = append(discovered, candidate)
				}
			}
		}
		{
			slices.Sort(discovered)
			discovered = slices.Compact(discovered)
			data := make([]gin.H, 0, len(discovered))
			google := strings.Contains(path, "/v1beta/")
			for _, name := range discovered {
				authorized := false
				for declarationProtocol, declarations := range priorDeclarations {
					for _, declaration := range declarations {
						if declaration.catalog.MatchesProtocol(declaration.group, declarationProtocol, name) {
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
	}
}

// selectGroup walks the key's group priority once and binds the first group that
// declares this model for this protocol. Once a group matches, an authorization
// or funding failure is terminal: it never falls through to the next group, so a
// request can never be silently paid for by another group's wallet.
func (r *multiGroupRouter) selectGroup(c *gin.Context, key *service.APIKey, protocol, model string) (int, string, bool) {
	ctx := c.Request.Context()
	var skipped []*service.Group
	for _, id := range key.GroupIDs {
		group, loadErr := r.groups.GetByID(ctx, id)
		if loadErr != nil || group == nil {
			return http.StatusServiceUnavailable, "An authorized group is unavailable; review this key's group selection", false
		}
		if r.forced != "" && group.Platform != r.forced {
			continue
		}
		if !service.MultiGroupProtocolSupported(group, protocol) {
			skipped = append(skipped, group)
			continue
		}
		models, catalogErr := r.catalog(ctx, group)
		if catalogErr != nil || models == nil {
			return http.StatusServiceUnavailable, "Model catalog is temporarily unavailable", false
		}
		if !models.MatchesProtocol(group, protocol, model) {
			skipped = append(skipped, group)
			continue
		}
		payer, authErr := r.access.AuthorizeMultiGroupTarget(ctx, key, group)
		if authErr != nil {
			return http.StatusForbidden, "The selected model group is no longer authorized; review this key's group selection", false
		}
		var subscription *service.UserSubscription
		if r.cfg == nil || r.cfg.RunMode != config.RunModeSimple {
			if group.IsSubscriptionType() {
				if r.subscriptions == nil {
					return http.StatusServiceUnavailable, "Subscription validation is unavailable", false
				}
				active, subErr := r.subscriptions.GetActiveSubscription(ctx, payer.ID, group.ID)
				if subErr != nil {
					return http.StatusForbidden, "No active subscription for the selected group; wallet fallback is disabled", false
				}
				subscription = active
				maintenance, limitErr := r.subscriptions.ValidateAndCheckLimits(subscription, group)
				if limitErr != nil {
					return infraerrors.Code(limitErr), infraerrors.Message(limitErr), false
				}
				if maintenance {
					snapshot := *subscription
					r.subscriptions.DoWindowMaintenance(&snapshot)
				}
			} else if payer.Balance <= 0 {
				return http.StatusForbidden, "Insufficient account balance", false
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
		return 0, "", true
	}
	return http.StatusNotFound, r.unmatchedModelMessage(ctx, skipped, protocol, model), false
}

// multiGroupAlternateProtocolScanLimit bounds the extra catalog reads spent on
// explaining a rejection. The hint is a courtesy; it must not turn one bad model
// name into a hundred database round trips.
const multiGroupAlternateProtocolScanLimit = 16

// unmatchedModelMessage names the model the caller actually asked for and, when
// one of the key's own groups serves it under a different native protocol, the
// endpoint that would work. Discovery returns the union across every authorized
// group, so an OpenAI-shaped client can legitimately see a Claude or Gemini name
// in its model picker; saying which endpoint owns it beats a bare 404.
func (r *multiGroupRouter) unmatchedModelMessage(ctx context.Context, skipped []*service.Group, protocol, model string) string {
	base := "No authorized group declares model " + model + " for the " + protocol + " endpoint"
	scanned := 0
	for _, group := range skipped {
		if scanned >= multiGroupAlternateProtocolScanLimit {
			break
		}
		scanned++
		models, err := r.catalog(ctx, group)
		if err != nil || models == nil {
			continue
		}
		for _, alternate := range service.MultiGroupTextProtocols {
			if alternate == protocol || !models.MatchesProtocol(group, alternate, model) {
				continue
			}
			if endpoint := multiGroupProtocolEndpoints[alternate]; endpoint != "" {
				return base + "; this key serves it on " + endpoint + " instead"
			}
		}
	}
	return base + "; pick a model this key exposes for this endpoint"
}

var multiGroupProtocolEndpoints = map[string]string{
	"messages":         "/v1/messages",
	"responses":        "/v1/responses",
	"chat_completions": "/v1/chat/completions",
	"generate_content": "/v1beta/models",
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
