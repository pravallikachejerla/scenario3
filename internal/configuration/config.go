// Package configuration provides versioned runtime configuration
// (routing, timeouts, feature flags) with a read-through cache, and
// the per-environment profile loader used at startup.
package configuration

import (
	"sync"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

// Store is the versioned source of truth for configuration, backed by
// the platform's storage layer.
type Store interface {
	PutConfig(c domain.ConfigVersion)
	GetConfigDirect(tenantID, key string) (domain.ConfigVersion, bool)
}

// Service is the read path application code actually calls. It
// fronts the Store with an in-process cache so that hot paths (for
// example resolving the fulfilment-timeout on every request) do not
// hit the store on every call.
type Service struct {
	store Store

	mu    sync.RWMutex
	cache map[string]domain.ConfigVersion
}

func NewService(store Store) *Service {
	return &Service{store: store, cache: make(map[string]domain.ConfigVersion)}
}

// Update writes a new configuration version to the store.
//
// SOURCE CONDITION (STAB-03): Update does not invalidate or refresh
// this service's read cache. A value already cached by Get keeps
// being served until the process restarts or the same key happens to
// be evicted, so an administrator's configuration change (for
// example a fulfilment-routing rule or a dependency timeout) can look
// "active" in the store while the running service keeps using the
// old value.
func (s *Service) Update(c domain.ConfigVersion) {
	s.store.PutConfig(c)
}

// Get resolves a configuration value, populating the cache on first
// read and then serving from the cache on every subsequent call for
// that key - including after Update has written a newer version.
func (s *Service) Get(tenantID, key string) (domain.ConfigVersion, bool) {
	ck := tenantID + "/" + key
	s.mu.RLock()
	if v, ok := s.cache[ck]; ok {
		s.mu.RUnlock()
		return v, true
	}
	s.mu.RUnlock()

	v, ok := s.store.GetConfigDirect(tenantID, key)
	if !ok {
		return domain.ConfigVersion{}, false
	}
	s.mu.Lock()
	s.cache[ck] = v
	s.mu.Unlock()
	return v, true
}

// Invalidate is the corrected primitive: a candidate repair is
// expected to call this (or an equivalent version-aware refresh) from
// Update so that Get can never observe a superseded value.
func (s *Service) Invalidate(tenantID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, tenantID+"/"+key)
}

// --- environment profile loader (STAB-07) -------------------------------

// Profile is a named, documented set of configuration defaults for
// one deployment environment.
type Profile struct {
	Name   string
	Values map[string]string
}

// documentedDefaults is the single source of truth every environment
// profile is supposed to honour explicitly.
var documentedDefaults = map[string]string{
	"fulfilment.dependency_timeout_ms": "2000",
	"fulfilment.retry_max_attempts":    "3",
	"routing.default_carrier":          "standard-carrier",
}

// EnvOne and EnvTwo model two sandbox environment profiles that
// should be behaviourally equivalent for the shared workload.
//
// SOURCE CONDITION (STAB-07): EnvTwo omits
// "fulfilment.dependency_timeout_ms" from its explicit profile. The
// loader below falls back to an implicit, undocumented default for
// any missing key rather than requiring every environment to state
// its value explicitly, so EnvTwo silently runs with a different
// dependency timeout than EnvOne and the documented default -
// identical source, different runtime behaviour.
var EnvOne = Profile{Name: "env-1", Values: map[string]string{
	"fulfilment.dependency_timeout_ms": "2000",
	"fulfilment.retry_max_attempts":    "3",
	"routing.default_carrier":          "standard-carrier",
}}

var EnvTwo = Profile{Name: "env-2", Values: map[string]string{
	"fulfilment.retry_max_attempts": "3",
	"routing.default_carrier":       "standard-carrier",
}}

// implicitFallback is the undocumented value the loader uses when a
// key is missing from a profile. It differs from
// documentedDefaults["fulfilment.dependency_timeout_ms"].
const implicitFallbackTimeoutMS = "8000"

// Load resolves a single key for a named profile the way the running
// service actually does it at startup.
func Load(p Profile, key string) string {
	if v, ok := p.Values[key]; ok {
		return v
	}
	if key == "fulfilment.dependency_timeout_ms" {
		return implicitFallbackTimeoutMS
	}
	return documentedDefaults[key]
}

// DocumentedDefault exposes the documented value so tests and
// operators can compare it against what Load actually resolves for a
// given environment.
func DocumentedDefault(key string) (string, bool) {
	v, ok := documentedDefaults[key]
	return v, ok
}
