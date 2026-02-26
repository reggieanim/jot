package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/reggieanim/jot/internal/modules/pages/domain"
)

type fakeClock struct {
	now time.Time
}

func (clock fakeClock) Now() time.Time {
	return clock.now
}

type inMemoryRepo struct {
	store      map[domain.PageID]domain.Page
	proofreads map[domain.ProofreadID]domain.Proofread
	reads      map[domain.PageID]map[string]struct{}
	shares     map[string]domain.PageShareLink
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		store:      map[domain.PageID]domain.Page{},
		proofreads: map[domain.ProofreadID]domain.Proofread{},
		reads:      map[domain.PageID]map[string]struct{}{},
		shares:     map[string]domain.PageShareLink{},
	}
}

func (repo *inMemoryRepo) Create(_ context.Context, page domain.Page) error {
	repo.store[page.ID] = page
	return nil
}

func (repo *inMemoryRepo) UpdateBlocks(_ context.Context, pageID domain.PageID, blocks []domain.Block) error {
	page := repo.store[pageID]
	page.Blocks = blocks
	repo.store[pageID] = page
	return nil
}

func (repo *inMemoryRepo) UpdateBlocksOptimistic(_ context.Context, pageID domain.PageID, blocks []domain.Block, _ *time.Time) error {
	return repo.UpdateBlocks(context.Background(), pageID, blocks)
}

func (repo *inMemoryRepo) UpdatePageMetaOptimistic(_ context.Context, pageID domain.PageID, title string, cover *string, darkMode bool, cinematic bool, mood int, bgColor string, _ *time.Time) error {
	page := repo.store[pageID]
	page.Title = title
	page.Cover = cover
	page.DarkMode = darkMode
	page.Cinematic = cinematic
	page.Mood = mood
	page.BgColor = bgColor
	repo.store[pageID] = page
	return nil
}

func (repo *inMemoryRepo) GetByID(_ context.Context, pageID domain.PageID) (domain.Page, error) {
	return repo.store[pageID], nil
}

func (repo *inMemoryRepo) GetByIDWithAuthor(_ context.Context, pageID domain.PageID) (domain.FeedPage, error) {
	page := repo.store[pageID]
	return domain.FeedPage{Page: page}, nil
}

func (repo *inMemoryRepo) SetPublished(_ context.Context, pageID domain.PageID, published bool, unlisted bool) error {
	page := repo.store[pageID]
	page.Published = published
	page.Unlisted = unlisted
	if published {
		now := time.Now().UTC()
		page.PublishedAt = &now
	} else {
		page.PublishedAt = nil
	}
	repo.store[pageID] = page
	return nil
}

func (repo *inMemoryRepo) CreateProofread(_ context.Context, proofread domain.Proofread) error {
	repo.proofreads[proofread.ID] = proofread
	return nil
}

func (repo *inMemoryRepo) ListProofreadsByPageID(_ context.Context, pageID domain.PageID) ([]domain.Proofread, error) {
	items := make([]domain.Proofread, 0)
	for _, proofread := range repo.proofreads {
		if proofread.PageID == pageID {
			items = append(items, proofread)
		}
	}
	return items, nil
}

func (repo *inMemoryRepo) GetProofreadByID(_ context.Context, proofreadID domain.ProofreadID) (domain.Proofread, error) {
	return repo.proofreads[proofreadID], nil
}

func (repo *inMemoryRepo) ListPages(_ context.Context, ownerID string) ([]domain.Page, error) {
	pages := make([]domain.Page, 0, len(repo.store))
	for _, page := range repo.store {
		if page.DeletedAt == nil && page.OwnerID != nil && *page.OwnerID == ownerID {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func (repo *inMemoryRepo) DeletePage(_ context.Context, pageID domain.PageID) error {
	delete(repo.store, pageID)
	return nil
}

func (repo *inMemoryRepo) ArchivePage(_ context.Context, pageID domain.PageID) error {
	page := repo.store[pageID]
	now := time.Now().UTC()
	page.DeletedAt = &now
	repo.store[pageID] = page
	return nil
}

func (repo *inMemoryRepo) RestorePage(_ context.Context, pageID domain.PageID) error {
	page := repo.store[pageID]
	page.DeletedAt = nil
	repo.store[pageID] = page
	return nil
}

func (repo *inMemoryRepo) ListArchivedPages(_ context.Context, ownerID string) ([]domain.Page, error) {
	pages := make([]domain.Page, 0)
	for _, page := range repo.store {
		if page.DeletedAt != nil && page.OwnerID != nil && *page.OwnerID == ownerID {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func (repo *inMemoryRepo) ListPublishedPagesByOwner(_ context.Context, ownerID string) ([]domain.Page, error) {
	pages := make([]domain.Page, 0)
	for _, page := range repo.store {
		if page.DeletedAt == nil && page.Published && !page.Unlisted && page.OwnerID != nil && *page.OwnerID == ownerID {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func (repo *inMemoryRepo) ListPublishedFeed(_ context.Context, limit, offset int, _ string, authorUserIDs []string) ([]domain.FeedPage, error) {
	all := make([]domain.FeedPage, 0)
	for _, page := range repo.store {
		if page.DeletedAt == nil && page.Published && !page.Unlisted {
			// Filter by author user IDs if specified
			if len(authorUserIDs) > 0 {
				found := false
				for _, uid := range authorUserIDs {
					if page.OwnerID != nil && *page.OwnerID == uid {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			all = append(all, domain.FeedPage{Page: page})
		}
	}
	if offset >= len(all) {
		return []domain.FeedPage{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (repo *inMemoryRepo) CreateShareLink(_ context.Context, share domain.PageShareLink) error {
	repo.shares[share.Token] = share
	return nil
}

func (repo *inMemoryRepo) GetShareLinkByToken(_ context.Context, token string) (domain.PageShareLink, error) {
	if share, ok := repo.shares[token]; ok {
		return share, nil
	}
	return domain.PageShareLink{}, nil
}

func (repo *inMemoryRepo) RevokeShareLinksByAccess(_ context.Context, pageID domain.PageID, ownerID string, access domain.ShareAccess) error {
	for token, share := range repo.shares {
		if share.PageID == pageID && share.CreatedBy == ownerID && share.Access == access {
			share.Revoked = true
			repo.shares[token] = share
		}
	}
	return nil
}

func (repo *inMemoryRepo) RecordOrganicRead(_ context.Context, pageID domain.PageID, readerKey string) (bool, error) {
	if _, ok := repo.reads[pageID]; !ok {
		repo.reads[pageID] = map[string]struct{}{}
	}
	if _, exists := repo.reads[pageID][readerKey]; exists {
		return false, nil
	}
	repo.reads[pageID][readerKey] = struct{}{}
	page := repo.store[pageID]
	page.ReadCount++
	repo.store[pageID] = page
	return true, nil
}

func (repo *inMemoryRepo) UpsertCollabUser(_ context.Context, _ domain.PageID, _ string, _ string) error {
	return nil
}

func (repo *inMemoryRepo) ListCollabUsers(_ context.Context, _ domain.PageID) ([]domain.CollabUser, error) {
	return []domain.CollabUser{}, nil
}

type noOpEvents struct{}

func (noOpEvents) PageCreated(_ context.Context, _ domain.Page) error   { return nil }
func (noOpEvents) BlocksUpdated(_ context.Context, _ domain.Page) error { return nil }
func (noOpEvents) PageDeleted(_ context.Context, _ domain.Page) error   { return nil }

func TestCreateAndGetPage(t *testing.T) {
	service := NewService(newInMemoryRepo(), noOpEvents{}, fakeClock{now: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)})
	blocks := []domain.Block{{
		ID:       "b1",
		Type:     domain.BlockTypeParagraph,
		Position: 0,
		Data:     json.RawMessage(`{"text":"hello"}`),
	}}

	page, err := service.CreatePage(context.Background(), "owner-1", "Welcome", nil, blocks)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := service.GetPage(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Title != "Welcome" {
		t.Fatalf("expected title Welcome, got %s", got.Title)
	}

	if len(got.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(got.Blocks))
	}

	if got.OwnerID == nil || *got.OwnerID != "owner-1" {
		t.Fatalf("expected owner_id owner-1, got %v", got.OwnerID)
	}
}
