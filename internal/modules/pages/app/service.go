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

func (service *Service) CreatePage(ctx context.Context, title string, cover *string, blocks []domain.Block) (domain.Page, error) {
	if title == "" {
		return domain.Page{}, errs.ErrInvalidInput
	}
	now := service.clock.Now()
	page := domain.Page{
		ID:        domain.PageID(uuid.NewString()),
		Title:     title,
		Cover:     cover,
		Published: false,
		DarkMode:  false,
		Cinematic: true,
		Mood:      65,
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

func (service *Service) UpdateBlocks(ctx context.Context, pageID domain.PageID, blocks []domain.Block) error {
	_, err := service.UpdateBlocksRealtime(ctx, pageID, blocks, nil)
	return err
}

func (service *Service) UpdateBlocksRealtime(ctx context.Context, pageID domain.PageID, blocks []domain.Block, expectedUpdatedAt *time.Time) (domain.Page, error) {
	if pageID == "" {
		return domain.Page{}, errs.ErrInvalidInput
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

func (service *Service) UpdatePageMetaRealtime(ctx context.Context, pageID domain.PageID, title string, cover *string, darkMode bool, cinematic bool, mood int, bgColor string, expectedUpdatedAt *time.Time) (domain.Page, error) {
	if pageID == "" || title == "" {
		return domain.Page{}, errs.ErrInvalidInput
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

func (service *Service) SetPagePublished(ctx context.Context, pageID domain.PageID, published bool) (domain.Page, error) {
	if pageID == "" {
		return domain.Page{}, errs.ErrInvalidInput
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

func (service *Service) ListPages(ctx context.Context) ([]domain.Page, error) {
	pages, err := service.repo.ListPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	return pages, nil
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
