package ports

import (
	"context"

	"github.com/reggieanim/jot/internal/modules/pages/domain"
)

type PageEvents interface {
	PageCreated(ctx context.Context, page domain.Page) error
	BlocksUpdated(ctx context.Context, page domain.Page) error
}
