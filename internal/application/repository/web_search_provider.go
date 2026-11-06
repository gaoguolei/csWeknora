package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// webSearchProviderRepository implements the WebSearchProviderRepository interface
type webSearchProviderRepository struct {
	db *gorm.DB
}

// NewWebSearchProviderRepository creates a new web search provider repository
func NewWebSearchProviderRepository(db *gorm.DB) interfaces.WebSearchProviderRepository {
	return &webSearchProviderRepository{db: db}
}

// Create creates a new web search provider
func (r *webSearchProviderRepository) Create(ctx context.Context, provider *types.WebSearchProviderEntity) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// GetByID retrieves a web search provider by ID.
//
// The provider catalogue is platform-wide: every workspace manages and
// consumes the same rows, so the lookup must not be narrowed by workspace.
// tenant_id records who registered the row, not who may see it. The tenantID
// argument stays only for call-site compatibility and is ignored.
func (r *webSearchProviderRepository) GetByID(ctx context.Context, _ uint64, id string) (*types.WebSearchProviderEntity, error) {
	var provider types.WebSearchProviderEntity
	if err := r.db.WithContext(ctx).Where(
		"id = ?", id,
	).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// GetDefault retrieves the platform default provider (is_default=true), or nil
// when none is marked. There is exactly one default for the whole deployment;
// rows left flagged by the pre-platform-shared era are ordered so the most
// recently updated one wins instead of failing on multiple matches.
func (r *webSearchProviderRepository) GetDefault(ctx context.Context, _ uint64) (*types.WebSearchProviderEntity, error) {
	var provider types.WebSearchProviderEntity
	if err := r.db.WithContext(ctx).Where(
		"is_default = ?", true,
	).Order("updated_at DESC").First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// List lists every web search provider on the platform.
func (r *webSearchProviderRepository) List(ctx context.Context, _ uint64) ([]*types.WebSearchProviderEntity, error) {
	var providers []*types.WebSearchProviderEntity
	if err := r.db.WithContext(ctx).
		Order("created_at ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// Update updates a web search provider.
//
// tenant_id is omitted from the write: it records the registering workspace and
// must not drift to whoever edited the row last.
func (r *webSearchProviderRepository) Update(ctx context.Context, provider *types.WebSearchProviderEntity) error {
	return r.db.WithContext(ctx).Model(&types.WebSearchProviderEntity{}).Where(
		"id = ?", provider.ID,
	).Select("*").Omit("tenant_id", "created_at").Updates(provider).Error
}

// Delete soft-deletes a web search provider.
func (r *webSearchProviderRepository) Delete(ctx context.Context, _ uint64, id string) error {
	return r.db.WithContext(ctx).Where(
		"id = ?", id,
	).Delete(&types.WebSearchProviderEntity{}).Error
}

// ClearDefault clears the default flag on every provider, optionally excluding
// one. The default is a platform-wide choice, so the sweep cannot be scoped to a
// workspace: two defaults would make GetDefault's answer depend on row order.
func (r *webSearchProviderRepository) ClearDefault(ctx context.Context, _ uint64, excludeID string) error {
	query := r.db.WithContext(ctx).Model(&types.WebSearchProviderEntity{}).Where(
		"is_default = ?", true,
	)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	return query.Update("is_default", false).Error
}
