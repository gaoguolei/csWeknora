package retriever

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// vectorStoreRepoOwnership adapts a VectorStoreRepository to the
// TenantStoreOwnership interface consumed by the factory functions.
//
// The store catalogue is platform-wide, so the repository's GetByID resolves any
// store regardless of workspace. "Owned by this tenant" therefore means "this
// deployment has a store with that ID" — the tenantID is forwarded only because
// the repository signature keeps it, and it no longer narrows the lookup.
type vectorStoreRepoOwnership struct {
	repo interfaces.VectorStoreRepository
}

// NewVectorStoreRepoOwnership returns the production
// TenantStoreOwnership implementation backed by VectorStoreRepository.
func NewVectorStoreRepoOwnership(repo interfaces.VectorStoreRepository) TenantStoreOwnership {
	return &vectorStoreRepoOwnership{repo: repo}
}

// StoreOwnedBy returns true iff a vector store with the given ID exists.
// Errors are reserved for infrastructure failures; a non-existent (but
// well-formed) store ID returns (false, nil).
func (o *vectorStoreRepoOwnership) StoreOwnedBy(ctx context.Context, storeID string, tenantID uint64) (bool, error) {
	store, err := o.repo.GetByID(ctx, tenantID, storeID)
	if err != nil {
		return false, err
	}
	return store != nil, nil
}
