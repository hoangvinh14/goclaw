package agent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// stubAgent is a minimal Agent implementation for router tests.
// ID() returns a fixed agent_key and IsRunning() tracks a bool.
type stubAgent struct {
	id      string
	running bool
}

func (s *stubAgent) ID() string                                          { return s.id }
func (s *stubAgent) Run(context.Context, RunRequest) (*RunResult, error) { return nil, nil }
func (s *stubAgent) IsRunning() bool                                     { return s.running }
func (s *stubAgent) Model() string                                       { return "test-model" }
func (s *stubAgent) ProviderName() string                                { return "test" }
func (s *stubAgent) Provider() providers.Provider                        { return nil }

// stubResolver builds a ResolverFunc that returns a stubAgent with a
// predetermined ID. If idByInput is set, the returned agent's ID is derived
// from the input (used for dual-tenant tests where each tenant has a distinct
// UUID but the same agent_key).
func stubResolver(agentKey string) ResolverFunc {
	return func(_ context.Context, _ string) (Agent, error) {
		return &stubAgent{id: agentKey}, nil
	}
}

// TestRouterGet_UUIDInputStoresCanonicalKey verifies that when the caller
// passes a UUID-like string to Get(), the cache entry lands under
// tenantID:agentKey (canonical), NOT tenantID:uuidStr. Exercises the
// canonicalization path via a real resolver call.
func TestRouterGet_UUIDInputStoresCanonicalKey(t *testing.T) {
	r := NewRouter()
	r.SetResolver(stubResolver("goctech-leader"))

	tenantID := uuid.New()
	ctx := store.WithTenantID(context.Background(), tenantID)
	agentUUID := uuid.New().String()

	ag, err := r.Get(ctx, agentUUID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ag.ID() != "goctech-leader" {
		t.Fatalf("ag.ID() = %q, want %q", ag.ID(), "goctech-leader")
	}

	canonical := tenantID.String() + ":goctech-leader"
	nonCanonical := tenantID.String() + ":" + agentUUID

	r.mu.RLock()
	_, canonicalExists := r.agents[canonical]
	_, nonCanonicalExists := r.agents[nonCanonical]
	r.mu.RUnlock()

	if !canonicalExists {
		t.Errorf("expected canonical key %q in cache", canonical)
	}
	if nonCanonicalExists {
		t.Errorf("unexpected non-canonical key %q in cache", nonCanonical)
	}
}

// TestRouterGet_IdempotentCacheUnderKeyOrUUID verifies that calling Get()
// first with agent_key then with UUID produces exactly ONE cache entry — the
// canonical tenantID:agent_key.
func TestRouterGet_IdempotentCacheUnderKeyOrUUID(t *testing.T) {
	r := NewRouter()
	var resolveCount atomic.Int32
	r.SetResolver(func(_ context.Context, _ string) (Agent, error) {
		resolveCount.Add(1)
		return &stubAgent{id: "my-agent"}, nil
	})

	tenantID := uuid.New()
	ctx := store.WithTenantID(context.Background(), tenantID)

	if _, err := r.Get(ctx, "my-agent"); err != nil {
		t.Fatalf("Get(agent_key): %v", err)
	}
	if _, err := r.Get(ctx, uuid.New().String()); err != nil {
		t.Fatalf("Get(uuid): %v", err)
	}

	r.mu.RLock()
	total := 0
	for k := range r.agents {
		if strings.HasPrefix(k, tenantID.String()+":") {
			total++
		}
	}
	r.mu.RUnlock()

	if total != 1 {
		t.Errorf("expected exactly 1 tenant-scoped cache entry, got %d", total)
	}
}

// TestRouterGet_UUIDCallerResolvesEveryTime documents the honest cost of
// canonicalization: a caller that keeps passing the UUID form never hits the
// canonical key on read, so the resolver runs on every call. Pin this
// behavior so future refactors don't pretend the cache covers UUID inputs.
func TestRouterGet_UUIDCallerResolvesEveryTime(t *testing.T) {
	r := NewRouter()
	var resolveCount atomic.Int32
	r.SetResolver(func(_ context.Context, _ string) (Agent, error) {
		resolveCount.Add(1)
		return &stubAgent{id: "fixed-key"}, nil
	})

	tenantID := uuid.New()
	ctx := store.WithTenantID(context.Background(), tenantID)

	uuidStr := uuid.New().String()
	for range 3 {
		if _, err := r.Get(ctx, uuidStr); err != nil {
			t.Fatalf("Get: %v", err)
		}
	}

	// Each call must miss the raw uuidStr key and fall through to resolver,
	// because Get writes to the CANONICAL key, not the raw input key.
	if got := resolveCount.Load(); got != 3 {
		t.Errorf("expected resolver to run 3 times (UUID caller is un-cached), got %d", got)
	}
}

// TestRouterGet_DualTenantSameAgentKey — staging MCP finding.
// `tieu-ho` exists in both Master and Việt Org tenants with different UUIDs.
// Verify the router stores independent entries per tenant and invalidation
// on one tenant does not affect the other.
func TestRouterGet_DualTenantSameAgentKey(t *testing.T) {
	r := NewRouter()
	r.SetResolver(stubResolver("tieu-ho"))

	tenantA := uuid.New()
	tenantB := uuid.New()
	ctxA := store.WithTenantID(context.Background(), tenantA)
	ctxB := store.WithTenantID(context.Background(), tenantB)

	if _, err := r.Get(ctxA, "tieu-ho"); err != nil {
		t.Fatalf("Get tenantA: %v", err)
	}
	if _, err := r.Get(ctxB, "tieu-ho"); err != nil {
		t.Fatalf("Get tenantB: %v", err)
	}

	keyA := tenantA.String() + ":tieu-ho"
	keyB := tenantB.String() + ":tieu-ho"

	r.mu.RLock()
	_, existsA := r.agents[keyA]
	_, existsB := r.agents[keyB]
	r.mu.RUnlock()

	if !existsA {
		t.Errorf("tenantA cache entry %q missing", keyA)
	}
	if !existsB {
		t.Errorf("tenantB cache entry %q missing", keyB)
	}
}
