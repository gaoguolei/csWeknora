package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// StorageBackendRepository defines persistence for storage backend instances.
//
// Backend instances are platform-wide: tenant_id records the registering
// workspace, not who may see or use the instance. List still takes the caller's
// workspace because legacy/environment aliases stay per workspace; see the
// implementation for the exact predicate.
type StorageBackendRepository interface {
	Create(ctx context.Context, backend *types.StorageBackend) error
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.StorageBackend, error)
	List(ctx context.Context, tenantID uint64) ([]*types.StorageBackend, error)
	Update(ctx context.Context, backend *types.StorageBackend) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	// FindLegacyAlias intentionally stays workspace-scoped: an alias resolves the
	// process-wide environment storage for ONE workspace.
	FindLegacyAlias(ctx context.Context, tenantID uint64, provider string) (*types.StorageBackend, error)
}

type StorageBackendService interface {
	Create(ctx context.Context, backend *types.StorageBackend) error
	Update(ctx context.Context, backend *types.StorageBackend) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	SetDefault(ctx context.Context, tenantID uint64, id string) error
	Test(ctx context.Context, backend *types.StorageBackend) error
}

// StorageBackendResolver is the single runtime entry point for resolving one
// concrete storage instance. backendID wins; provider is a legacy fallback.
type StorageBackendResolver interface {
	ResolveFileService(ctx context.Context, tenant *types.Tenant, backendID, provider, localBaseDir string) (FileService, string, error)
	ResolveBackend(ctx context.Context, tenant *types.Tenant, backendID, provider string) (*types.StorageBackend, error)
}
