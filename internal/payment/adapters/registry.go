package adapters

import (
	"strings"

	"github.com/smallbiznis/valora/internal/payment/domain"
)

type Registry struct {
	factories map[string]domain.AdapterFactory
}

func NewRegistry(factories ...domain.AdapterFactory) *Registry {
	registry := &Registry{factories: map[string]domain.AdapterFactory{}}
	for _, factory := range factories {
		if factory == nil {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(factory.Provider()))
		if provider == "" {
			continue
		}
		registry.factories[provider] = factory
	}
	return registry
}

func (r *Registry) ProviderExists(provider string) bool {
	if r == nil {
		return false
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	_, ok := r.factories[provider]
	return ok
}

func (r *Registry) NewAdapter(provider string, cfg domain.AdapterConfig) (domain.PaymentAdapter, error) {
	if r == nil {
		return nil, domain.ErrProviderNotFound
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	factory, ok := r.factories[provider]
	if !ok {
		return nil, domain.ErrProviderNotFound
	}
	return factory.NewAdapter(cfg)
}
