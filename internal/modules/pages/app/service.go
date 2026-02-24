package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reggieanim/jot/internal/modules/pages/domain"
	"github.com/reggieanim/jot/internal/modules/pages/ports"
	"github.com/reggieanim/jot/internal/shared/errs"
)

type Clock interface {
	Now() time.Time
}

type Service struct {
	repo   ports.PageRepository
	events ports.PageEvents
	clock  Clock
}

func NewService(repo ports.PageRepository, events ports.PageEvents, clock Clock) *Service {
	return &Service{repo: repo, events: events, clock: clock}
}

func (service *Service) CreatePage(ctx context.Context, ownerID string, title string, cover *string, blocks []domain.Block) (domain.Page, error) {
	return service.CreatePageWithSettings(ctx, ownerID, title, cover, blocks, false, true, 65, "")
}

func (service *Service) CreatePageWithSettings(
	ctx context.Context,
	ownerID string,
	title string,
	cover *string,
	blocks []domain.Block,
	darkMode bool,
	cinematic bool,
	mood int,
	bgColor string,
) (domain.Page, error) {
	if ownerID == "" || title == "" {
		return domain.Page{}, errs.ErrInvalidInput
	}
	if mood < 0 {
		mood = 0
	}
	if mood > 100 {
		mood = 100
	}
	now := service.clock.Now()
	page := domain.Page{
		ID:        domain.PageID(uuid.NewString()),
		OwnerID:   &ownerID,
		Title:     title,
		Cover:     cover,
		Published: false,
		DarkMode:  darkMode,
		Cinematic: cinematic,
		Mood:      mood,
		BgColor:   bgColor,
		Blocks:    blocks,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := service.repo.Create(ctx, page); err != nil {
		return domain.Page{}, fmt.Errorf("create page: %w", err)
	}
	persisted, err := service.repo.GetByID(ctx, page.ID)
	if err != nil {
		return domain.Page{}, fmt.Errorf("fetch created page: %w", err)
	}
	if err := service.events.PageCreated(ctx, persisted); err != nil {
		return domain.Page{}, fmt.Errorf("publish page created: %w", err)
	}
	return persisted, nil
}

func (service *Service) UpdateBlocks(ctx context.Context, ownerID string, pageID domain.PageID, blocks []domain.Block) error {
	_, err := service.UpdateBlocksRealtimeWithShare(ctx, ownerID, pageID, blocks, nil, "")
	return err
}

func (service *Service) UpdateBlocksRealtime(ctx context.Context, ownerID string, pageID domain.PageID, blocks []domain.Block, expectedUpdatedAt *time.Time) (domain.Page, error) {
	return service.UpdateBlocksRealtimeWithShare(ctx, ownerID, pageID, blocks, expectedUpdatedAt, "")
}

func (service *Service) UpdateBlocksRealtimeWithShare(ctx context.Context, actorID string, pageID domain.PageID, blocks []domain.Block, expectedUpdatedAt *time.Time, shareToken string) (domain.Page, error) {
	if pageID == "" {
		return domain.Page{}, errs.ErrInvalidInput
	}
	if _, _, err := service.ResolvePageAccess(ctx, actorID, pageID, shareToken, domain.ShareAccessEdit); err != nil {
		return domain.Page{}, err
	}
	if err := service.repo.UpdateBlocksOptimistic(ctx, pageID, blocks, expectedUpdatedAt); err != nil {
		return domain.Page{}, fmt.Errorf("update blocks: %w", err)
	}
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return domain.Page{}, fmt.Errorf("fetch updated page: %w", err)
	}
	if err := service.events.BlocksUpdated(ctx, page); err != nil {
		return domain.Page{}, fmt.Errorf("publish blocks updated: %w", err)
	}
	return page, nil
}

func (service *Service) UpdatePageMetaRealtime(ctx context.Context, ownerID string, pageID domain.PageID, title string, cover *string, darkMode bool, cinematic bool, mood int, bgColor string, expectedUpdatedAt *time.Time) (domain.Page, error) {
	return service.UpdatePageMetaRealtimeWithShare(ctx, ownerID, pageID, title, cover, darkMode, cinematic, mood, bgColor, expectedUpdatedAt, "")
}

func (service *Service) UpdatePageMetaRealtimeWithShare(ctx context.Context, actorID string, pageID domain.PageID, title string, cover *string, darkMode bool, cinematic bool, mood int, bgColor string, expectedUpdatedAt *time.Time, shareToken string) (domain.Page, error) {
	if pageID == "" || title == "" {
		return domain.Page{}, errs.ErrInvalidInput
	}
	if _, _, err := service.ResolvePageAccess(ctx, actorID, pageID, shareToken, domain.ShareAccessEdit); err != nil {
		return domain.Page{}, err
	}
	if mood < 0 {
		mood = 0
	}
	if mood > 100 {
		mood = 100
	}

	if err := service.repo.UpdatePageMetaOptimistic(ctx, pageID, title, cover, darkMode, cinematic, mood, bgColor, expectedUpdatedAt); err != nil {
		return domain.Page{}, fmt.Errorf("update page meta: %w", err)
	}

	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return domain.Page{}, fmt.Errorf("fetch updated page: %w", err)
	}
	if err := service.events.BlocksUpdated(ctx, page); err != nil {
		return domain.Page{}, fmt.Errorf("publish page updated: %w", err)
	}

	return page, nil
}

func (service *Service) GetPage(ctx context.Context, pageID domain.PageID) (domain.Page, error) {
	if pageID == "" {
		return domain.Page{}, errs.ErrInvalidInput
	}
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return domain.Page{}, fmt.Errorf("get page by id: %w", err)
	}
	return page, nil
}

func (service *Service) SetPagePublished(ctx context.Context, ownerID string, pageID domain.PageID, published bool) (domain.Page, error) {
	if pageID == "" {
		return domain.Page{}, errs.ErrInvalidInput
	}
	if err := service.checkOwnership(ctx, pageID, ownerID); err != nil {
		return domain.Page{}, err
	}
	if err := service.repo.SetPublished(ctx, pageID, published); err != nil {
		return domain.Page{}, fmt.Errorf("set page published: %w", err)
	}
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return domain.Page{}, fmt.Errorf("fetch published page: %w", err)
	}
	if err := service.events.BlocksUpdated(ctx, page); err != nil {
		return domain.Page{}, fmt.Errorf("publish page updated: %w", err)
	}
	return page, nil
}

func (service *Service) ListPages(ctx context.Context, ownerID string) ([]domain.Page, error) {
	pages, err := service.repo.ListPages(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	return pages, nil
}

func (service *Service) DeletePage(ctx context.Context, ownerID string, pageID domain.PageID) error {
	if pageID == "" {
		return errs.ErrInvalidInput
	}

	// Fetch full page (with blocks) before deletion so we can emit an event
	// carrying the media URLs for downstream cleanup.
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return fmt.Errorf("get page for delete: %w", err)
	}
	if page.OwnerID == nil || *page.OwnerID != ownerID {
		return errs.ErrForbidden
	}

	if err := service.repo.DeletePage(ctx, pageID); err != nil {
		return fmt.Errorf("delete page: %w", err)
	}

	// Best-effort: emit event so the files module can clean up S3 objects.
	_ = service.events.PageDeleted(ctx, page)

	return nil
}

func (service *Service) ArchivePage(ctx context.Context, ownerID string, pageID domain.PageID) error {
	if pageID == "" {
		return errs.ErrInvalidInput
	}
	if err := service.checkOwnership(ctx, pageID, ownerID); err != nil {
		return err
	}
	if err := service.repo.ArchivePage(ctx, pageID); err != nil {
		return fmt.Errorf("archive page: %w", err)
	}
	return nil
}

func (service *Service) RestorePage(ctx context.Context, ownerID string, pageID domain.PageID) error {
	if pageID == "" {
		return errs.ErrInvalidInput
	}
	// For restore, the page has deleted_at set, so we look it up and check owner
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return fmt.Errorf("get page for restore: %w", err)
	}
	if page.OwnerID == nil || *page.OwnerID != ownerID {
		return errs.ErrForbidden
	}
	if err := service.repo.RestorePage(ctx, pageID); err != nil {
		return fmt.Errorf("restore page: %w", err)
	}
	return nil
}

func (service *Service) ListArchivedPages(ctx context.Context, ownerID string) ([]domain.Page, error) {
	pages, err := service.repo.ListArchivedPages(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list archived pages: %w", err)
	}
	return pages, nil
}

func (service *Service) ListPublishedPagesByOwner(ctx context.Context, ownerID string) ([]domain.Page, error) {
	pages, err := service.repo.ListPublishedPagesByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list published pages by owner: %w", err)
	}
	return pages, nil
}

func (service *Service) ListPublishedFeed(ctx context.Context, limit, offset int, sort string) ([]domain.FeedPage, error) {
	pages, err := service.repo.ListPublishedFeed(ctx, limit, offset, sort)
	if err != nil {
		return nil, fmt.Errorf("list published feed: %w", err)
	}
	return pages, nil
}

func (service *Service) CreateShareLink(ctx context.Context, ownerID string, pageID domain.PageID, access domain.ShareAccess) (domain.PageShareLink, error) {
	if pageID == "" {
		return domain.PageShareLink{}, errs.ErrInvalidInput
	}
	if access != domain.ShareAccessView && access != domain.ShareAccessEdit {
		return domain.PageShareLink{}, errs.ErrInvalidInput
	}
	if err := service.checkOwnership(ctx, pageID, ownerID); err != nil {
		return domain.PageShareLink{}, err
	}
	share := domain.PageShareLink{
		Token:     uuid.NewString(),
		PageID:    pageID,
		Access:    access,
		CreatedBy: ownerID,
		Revoked:   false,
		CreatedAt: service.clock.Now(),
	}
	if err := service.repo.CreateShareLink(ctx, share); err != nil {
		return domain.PageShareLink{}, fmt.Errorf("create share link: %w", err)
	}
	return share, nil
}

func (service *Service) ResolvePageAccess(ctx context.Context, actorID string, pageID domain.PageID, shareToken string, required domain.ShareAccess) (domain.Page, string, error) {
	if pageID == "" {
		return domain.Page{}, "", errs.ErrInvalidInput
	}
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return domain.Page{}, "", fmt.Errorf("resolve page access: %w", err)
	}
	if actorID != "" && page.OwnerID != nil && *page.OwnerID == actorID {
		return page, "owner", nil
	}

	shareToken = strings.TrimSpace(shareToken)
	if shareToken == "" {
		return domain.Page{}, "", errs.ErrForbidden
	}

	share, err := service.repo.GetShareLinkByToken(ctx, shareToken)
	if err != nil {
		return domain.Page{}, "", errs.ErrForbidden
	}
	if share.Revoked || share.PageID != pageID {
		return domain.Page{}, "", errs.ErrForbidden
	}
	if required == domain.ShareAccessEdit && share.Access != domain.ShareAccessEdit {
		return domain.Page{}, "", errs.ErrForbidden
	}

	if share.Access == domain.ShareAccessEdit {
		return page, "edit", nil
	}
	return page, "view", nil
}

func (service *Service) checkOwnership(ctx context.Context, pageID domain.PageID, ownerID string) error {
	page, err := service.repo.GetByID(ctx, pageID)
	if err != nil {
		return fmt.Errorf("check ownership: %w", err)
	}
	if page.OwnerID == nil || *page.OwnerID != ownerID {
		return errs.ErrForbidden
	}
	return nil
}

func (service *Service) GetPublicPage(ctx context.Context, pageID domain.PageID) (domain.Page, error) {
	page, err := service.GetPage(ctx, pageID)
	if err != nil {
		return domain.Page{}, err
	}
	if !page.Published {
		return domain.Page{}, errs.ErrNotFound
	}
	return page, nil
}

func (service *Service) RecordPublicRead(ctx context.Context, pageID domain.PageID, readerKey string) (bool, error) {
	if pageID == "" || strings.TrimSpace(readerKey) == "" {
		return false, nil
	}
	unique, err := service.repo.RecordOrganicRead(ctx, pageID, readerKey)
	if err != nil {
		return false, fmt.Errorf("record organic read: %w", err)
	}
	return unique, nil
}

func (service *Service) GetPublicBlock(ctx context.Context, pageID domain.PageID, blockID string) (domain.Block, domain.Page, error) {
	if blockID == "" {
		return domain.Block{}, domain.Page{}, errs.ErrInvalidInput
	}
	page, err := service.GetPublicPage(ctx, pageID)
	if err != nil {
		return domain.Block{}, domain.Page{}, err
	}
	for _, block := range page.Blocks {
		if block.ID == blockID {
			return block, page, nil
		}
	}
	return domain.Block{}, domain.Page{}, errs.ErrNotFound
}

func (service *Service) CreateProofread(ctx context.Context, pageID domain.PageID, authorName, title, summary, stance string, annotations []domain.ProofreadAnnotation) (domain.Proofread, error) {
	if pageID == "" || strings.TrimSpace(authorName) == "" || strings.TrimSpace(title) == "" {
		return domain.Proofread{}, errs.ErrInvalidInput
	}

	page, err := service.GetPublicPage(ctx, pageID)
	if err != nil {
		return domain.Proofread{}, err
	}

	now := service.clock.Now()
	proofread := domain.Proofread{
		ID:          domain.ProofreadID(uuid.NewString()),
		PageID:      page.ID,
		AuthorName:  strings.TrimSpace(authorName),
		Title:       strings.TrimSpace(title),
		Summary:     strings.TrimSpace(summary),
		Stance:      strings.TrimSpace(stance),
		Annotations: annotations,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if proofread.Stance == "" {
		proofread.Stance = "review"
	}

	if err := service.repo.CreateProofread(ctx, proofread); err != nil {
		return domain.Proofread{}, fmt.Errorf("create proofread: %w", err)
	}
	return proofread, nil
}

func (service *Service) ListProofreads(ctx context.Context, pageID domain.PageID) ([]domain.Proofread, error) {
	if pageID == "" {
		return nil, errs.ErrInvalidInput
	}
	if _, err := service.GetPublicPage(ctx, pageID); err != nil {
		return nil, err
	}
	proofreads, err := service.repo.ListProofreadsByPageID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("list proofreads: %w", err)
	}
	return proofreads, nil
}

func (service *Service) GetProofread(ctx context.Context, proofreadID domain.ProofreadID) (domain.Proofread, domain.Page, error) {
	if proofreadID == "" {
		return domain.Proofread{}, domain.Page{}, errs.ErrInvalidInput
	}
	proofread, err := service.repo.GetProofreadByID(ctx, proofreadID)
	if err != nil {
		return domain.Proofread{}, domain.Page{}, fmt.Errorf("get proofread by id: %w", err)
	}
	page, err := service.GetPublicPage(ctx, proofread.PageID)
	if err != nil {
		return domain.Proofread{}, domain.Page{}, err
	}
	return proofread, page, nil
}
