package interfaces

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
)

type WorkspaceTestRunRepository interface {
	Create(context.Context, *types.WorkspaceTestRun) error
	Save(context.Context, *types.WorkspaceTestRun) error
	Get(context.Context, uint64, string, string, string) (*types.WorkspaceTestRun, error)
	List(context.Context, uint64, string, string) ([]types.WorkspaceTestRun, error)
	Active(context.Context, uint64, string) (*types.WorkspaceTestRun, error)
	PublishReport(context.Context, *types.WorkspaceTestRun, types.MessageArtifacts) (string, bool, error)
	RequestCancel(context.Context, *types.WorkspaceTestRun) error
}
