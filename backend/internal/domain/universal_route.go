package domain

// UniversalRouteConfig maps one public model and inbound protocol to an
// existing concrete group. Existing groups remain the account, pricing and
// entitlement pools; the universal group never copies account bindings.
type UniversalRouteConfig struct {
	PublicModel     string `json:"public_model"`
	MatchType       string `json:"match_type"`       // exact | prefix
	InboundProtocol string `json:"inbound_protocol"` // any | chat_completions | anthropic | responses
	TargetGroupID   int64  `json:"target_group_id"`
	Priority        int    `json:"priority"` // lower wins until PR3 supplies Shadow ranking
	Enabled         bool   `json:"enabled"`
}
