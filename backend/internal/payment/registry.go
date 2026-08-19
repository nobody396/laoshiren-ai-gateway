package payment

// Registered provider names.
const (
	ProviderEasyPay = "easypay"
)

// Registry resolves payment providers by name.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry builds a Registry from the given providers, keyed by Name().
func NewRegistry(providers ...Provider) *Registry {
	r := &Registry{providers: make(map[string]Provider, len(providers))}
	for _, p := range providers {
		if p == nil {
			continue
		}
		r.providers[p.Name()] = p
	}
	return r
}

// Get returns the provider registered under name.
func (r *Registry) Get(name string) (Provider, error) {
	if r != nil {
		if p, ok := r.providers[name]; ok {
			return p, nil
		}
	}
	return nil, ErrPaymentProviderNotFound
}
