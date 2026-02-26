package ports

import (
	"context"
	"time"

	"github.com/reggieanim/jot/internal/modules/pages/domain"
)

type PageRepository interface {
	Create(ctx context.Context, page domain.Page) error
	UpdateBlocks(ctx context.Context, pageID domain.PageID, blocks []domain.Block) error
	UpdateBlocksOptimistic(ctx context.Context, pageID domain.PageID, blocks []domain.Block, expectedUpdatedAt *time.Time) error
	UpdatePageMetaOptimistic(ctx context.Context, pageID domain.PageID, title string, cover *string, darkMode bool, cinematic bool, mood int, bgColor string, expectedUpdatedAt *time.Time) error
	SetPublished(ctx context.Context, pageID domain.PageID, published bool, unlisted bool) error
	GetByID(ctx context.Context, pageID domain.PageID) (domain.Page, error)
	GetByIDWithAuthor(ctx context.Context, pageID domain.PageID) (domain.FeedPage, error)
	ListPages(ctx context.Context, ownerID string) ([]domain.Page, error)
	ListPublishedPagesByOwner(ctx context.Context, ownerID string) ([]domain.Page, error)
	ListPublishedFeed(ctx context.Context, limit, offset int, sort string, authorUserIDs []string) ([]domain.FeedPage, error)
	CreateShareLink(ctx context.Context, share domain.PageShareLink) error
	GetShareLinkByToken(ctx context.Context, token string) (domain.PageShareLink, error)
	RevokeShareLinksByAccess(ctx context.Context, pageID domain.PageID, ownerID string, access domain.ShareAccess) error
	DeletePage(ctx context.Context, pageID domain.PageID) error
	ArchivePage(ctx context.Context, pageID domain.PageID) error
	RestorePage(ctx context.Context, pageID domain.PageID) error
	ListArchivedPages(ctx context.Context, ownerID string) ([]domain.Page, error)
	RecordOrganicRead(ctx context.Context, pageID domain.PageID, readerKey string) (bool, error)
	CreateProofread(ctx context.Context, proofread domain.Proofread) error
	ListProofreadsByPageID(ctx context.Context, pageID domain.PageID) ([]domain.Proofread, error)
	GetProofreadByID(ctx context.Context, proofreadID domain.ProofreadID) (domain.Proofread, error)
	UpsertCollabUser(ctx context.Context, pageID domain.PageID, userID string, access string) error
	ListCollabUsers(ctx context.Context, pageID domain.PageID) ([]domain.CollabUser, error)
}
