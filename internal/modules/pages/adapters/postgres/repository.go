package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggieanim/jot/internal/modules/pages/domain"
	"github.com/reggieanim/jot/internal/shared/errs"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Create(ctx context.Context, page domain.Page) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO pages (id, title, cover, published, unlisted, dark_mode, cinematic, mood, bg_color, owner_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, string(page.ID), page.Title, page.Cover, page.Published, page.Unlisted, page.DarkMode, page.Cinematic, page.Mood, page.BgColor, page.OwnerID, page.CreatedAt, page.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert page: %w", err)
	}
	if err := repository.insertBlocks(ctx, tx, page.ID, page.Blocks); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create page: %w", err)
	}
	return nil
}

func (repository *Repository) UpdateBlocks(ctx context.Context, pageID domain.PageID, blocks []domain.Block) error {
	return repository.UpdateBlocksOptimistic(ctx, pageID, blocks, nil)
}

func (repository *Repository) UpdatePageMetaOptimistic(ctx context.Context, pageID domain.PageID, title string, cover *string, darkMode bool, cinematic bool, mood int, bgColor string, expectedUpdatedAt *time.Time) error {
	if mood < 0 {
		mood = 0
	}
	if mood > 100 {
		mood = 100
	}

	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE pages
		SET title = $2, cover = $3, dark_mode = $4, cinematic = $5, mood = $6, bg_color = $7, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND ($8::timestamptz IS NULL OR updated_at = $8)
	`, string(pageID), title, cover, darkMode, cinematic, mood, bgColor, expectedUpdatedAt)
	if err != nil {
		return fmt.Errorf("update page meta: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		var exists bool
		if err := repository.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pages WHERE id = $1 AND deleted_at IS NULL)`, string(pageID)).Scan(&exists); err != nil {
			return fmt.Errorf("check page existence: %w", err)
		}
		if !exists {
			return errs.ErrNotFound
		}
		return errs.ErrConflict
	}

	return nil
}

func (repository *Repository) SetPublished(ctx context.Context, pageID domain.PageID, published bool, unlisted bool) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE pages
		SET published = $2,
		    unlisted = $3,
		    published_at = CASE WHEN $2 THEN now() ELSE NULL END,
		    updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, string(pageID), published, unlisted)
	if err != nil {
		return fmt.Errorf("set published: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return nil
}

func (repository *Repository) DeletePage(ctx context.Context, pageID domain.PageID) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM blocks WHERE page_id = $1`, string(pageID))
	if err != nil {
		return fmt.Errorf("delete blocks: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM proofreads WHERE page_id = $1`, string(pageID))
	if err != nil {
		return fmt.Errorf("delete proofreads: %w", err)
	}

	commandTag, err := tx.Exec(ctx, `DELETE FROM pages WHERE id = $1`, string(pageID))
	if err != nil {
		return fmt.Errorf("delete page: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete page: %w", err)
	}
	return nil
}

func (repository *Repository) ArchivePage(ctx context.Context, pageID domain.PageID) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE pages
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, string(pageID))
	if err != nil {
		return fmt.Errorf("archive page: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return nil
}

func (repository *Repository) RestorePage(ctx context.Context, pageID domain.PageID) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE pages
		SET deleted_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL
	`, string(pageID))
	if err != nil {
		return fmt.Errorf("restore page: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return nil
}

func (repository *Repository) ListArchivedPages(ctx context.Context, ownerID string) ([]domain.Page, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT
			p.id, p.title, p.cover, p.published, p.unlisted, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.owner_id, p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) AS proofread_count,
			(SELECT count(*) FROM blocks b WHERE b.page_id = p.id) AS block_count,
			(SELECT count(*) FROM page_reads r WHERE r.page_id = p.id) AS read_count
		FROM pages p
		WHERE p.deleted_at IS NOT NULL AND p.owner_id = $1
		ORDER BY p.deleted_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list archived pages: %w", err)
	}
	defer rows.Close()

	pages := make([]domain.Page, 0)
	for rows.Next() {
		var page domain.Page
		if err := rows.Scan(&page.ID, &page.Title, &page.Cover, &page.Published, &page.Unlisted, &page.PublishedAt, &page.DarkMode, &page.Cinematic, &page.Mood, &page.BgColor, &page.OwnerID, &page.CreatedAt, &page.UpdatedAt, &page.DeletedAt, &page.ProofreadCount, &page.BlockCount, &page.ReadCount); err != nil {
			return nil, fmt.Errorf("scan archived page row: %w", err)
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate archived pages rows: %w", err)
	}
	return pages, nil
}

func (repository *Repository) ListPublishedPagesByOwner(ctx context.Context, ownerID string) ([]domain.Page, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT
			p.id, p.title, p.cover, p.published, p.unlisted, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.owner_id, p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) AS proofread_count,
			(SELECT count(*) FROM blocks b WHERE b.page_id = p.id) AS block_count,
			(SELECT count(*) FROM page_reads r WHERE r.page_id = p.id) AS read_count,
			EXISTS(SELECT 1 FROM page_share_links s WHERE s.page_id = p.id AND s.revoked = false) AS has_share_links
		FROM pages p
		WHERE p.deleted_at IS NULL AND p.published = true AND p.unlisted = false AND p.owner_id = $1
		ORDER BY p.published_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list published pages by owner: %w", err)
	}
	defer rows.Close()

	pages := make([]domain.Page, 0)
	for rows.Next() {
		var page domain.Page
		if err := rows.Scan(&page.ID, &page.Title, &page.Cover, &page.Published, &page.Unlisted, &page.PublishedAt, &page.DarkMode, &page.Cinematic, &page.Mood, &page.BgColor, &page.OwnerID, &page.CreatedAt, &page.UpdatedAt, &page.DeletedAt, &page.ProofreadCount, &page.BlockCount, &page.ReadCount, &page.HasShareLinks); err != nil {
			return nil, fmt.Errorf("scan published page row: %w", err)
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published pages rows: %w", err)
	}

	// Fetch preview blocks
	if len(pages) > 0 {
		pageIDs := make([]string, len(pages))
		pageMap := make(map[string]*domain.Page, len(pages))
		for i := range pages {
			pageIDs[i] = string(pages[i].ID)
			pageMap[string(pages[i].ID)] = &pages[i]
		}

		blockRows, err := repository.pool.Query(ctx, `
			SELECT DISTINCT ON (page_id) id, page_id, parent_id, type, position, data
			FROM blocks
			WHERE page_id = ANY($1) AND type IN ('image', 'embed', 'gallery', 'music')
			ORDER BY page_id, position
		`, pageIDs)
		if err != nil {
			return nil, fmt.Errorf("query preview blocks: %w", err)
		}
		defer blockRows.Close()

		for blockRows.Next() {
			var block domain.Block
			var blockType string
			var data []byte
			if err := blockRows.Scan(&block.ID, &block.PageID, &block.ParentID, &blockType, &block.Position, &data); err != nil {
				return nil, fmt.Errorf("scan preview block: %w", err)
			}
			block.Type = domain.BlockType(blockType)
			block.Data = json.RawMessage(data)
			if p, ok := pageMap[string(block.PageID)]; ok {
				p.Blocks = []domain.Block{block}
			}
		}
		if err := blockRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate preview blocks: %w", err)
		}
	}

	return pages, nil
}

func (repository *Repository) ListPublishedFeed(ctx context.Context, limit, offset int, sort string, authorUserIDs []string) ([]domain.FeedPage, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	var orderClause string
	switch sort {
	case "top":
		orderClause = "ORDER BY (SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) DESC, p.published_at DESC"
	case "hot":
		// Hot = engagement weighted by recency (logarithmic decay over 48h)
		orderClause = "ORDER BY ((SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) + 1) / POWER(EXTRACT(EPOCH FROM (NOW() - COALESCE(p.published_at, p.created_at))) / 3600 + 2, 1.5) DESC"
	default: // "new"
		orderClause = "ORDER BY p.published_at DESC"
	}

	var whereClause string
	var args []interface{}
	args = append(args, limit, offset)

	if len(authorUserIDs) > 0 {
		placeholders := make([]string, len(authorUserIDs))
		for i, uid := range authorUserIDs {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, uid)
		}
		whereClause = fmt.Sprintf("AND p.owner_id IN (%s)", strings.Join(placeholders, ","))
	}

	query := fmt.Sprintf(`
		SELECT
			p.id, p.title, p.cover, p.published, p.unlisted, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.owner_id,
			p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) AS proofread_count,
			(SELECT count(*) FROM blocks b WHERE b.page_id = p.id) AS block_count,
			(SELECT count(*) FROM page_reads r WHERE r.page_id = p.id) AS read_count,
			EXISTS(SELECT 1 FROM page_share_links s WHERE s.page_id = p.id AND s.revoked = false) AS has_share_links,
			COALESCE(u.username, '') AS author_username,
			COALESCE(u.display_name, '') AS author_display_name,
			COALESCE(u.avatar_url, '') AS author_avatar_url
		FROM pages p
		LEFT JOIN users u ON u.id = p.owner_id
		WHERE p.deleted_at IS NULL AND p.published = true AND p.unlisted = false
		%s
		%s
		LIMIT $1 OFFSET $2
	`, whereClause, orderClause)

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list published feed: %w", err)
	}
	defer rows.Close()

	pages := make([]domain.FeedPage, 0)
	for rows.Next() {
		var fp domain.FeedPage
		if err := rows.Scan(
			&fp.ID, &fp.Title, &fp.Cover, &fp.Published, &fp.Unlisted, &fp.PublishedAt,
			&fp.DarkMode, &fp.Cinematic, &fp.Mood, &fp.BgColor, &fp.OwnerID,
			&fp.CreatedAt, &fp.UpdatedAt, &fp.DeletedAt,
			&fp.ProofreadCount, &fp.BlockCount, &fp.ReadCount, &fp.HasShareLinks,
			&fp.AuthorUsername, &fp.AuthorDisplayName, &fp.AuthorAvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scan feed page row: %w", err)
		}
		pages = append(pages, fp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feed pages rows: %w", err)
	}

	// Fetch preview blocks
	if len(pages) > 0 {
		pageIDs := make([]string, len(pages))
		pageMap := make(map[string]*domain.FeedPage, len(pages))
		for i := range pages {
			pageIDs[i] = string(pages[i].ID)
			pageMap[string(pages[i].ID)] = &pages[i]
		}

		blockRows, err := repository.pool.Query(ctx, `
			SELECT DISTINCT ON (page_id) id, page_id, parent_id, type, position, data
			FROM blocks
			WHERE page_id = ANY($1) AND type IN ('image', 'embed', 'gallery', 'music')
			ORDER BY page_id, position
		`, pageIDs)
		if err != nil {
			return nil, fmt.Errorf("query feed preview blocks: %w", err)
		}
		defer blockRows.Close()

		for blockRows.Next() {
			var block domain.Block
			var blockType string
			var data []byte
			if err := blockRows.Scan(&block.ID, &block.PageID, &block.ParentID, &blockType, &block.Position, &data); err != nil {
				return nil, fmt.Errorf("scan feed preview block: %w", err)
			}
			block.Type = domain.BlockType(blockType)
			block.Data = json.RawMessage(data)
			if p, ok := pageMap[string(block.PageID)]; ok {
				p.Blocks = []domain.Block{block}
			}
		}
		if err := blockRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate feed preview blocks: %w", err)
		}
	}

	return pages, nil
}

func (repository *Repository) CreateShareLink(ctx context.Context, share domain.PageShareLink) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO page_share_links (token, page_id, access, created_by, revoked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, share.Token, string(share.PageID), string(share.Access), share.CreatedBy, share.Revoked, share.CreatedAt)
	if err != nil {
		return fmt.Errorf("create share link: %w", err)
	}
	return nil
}

func (repository *Repository) GetShareLinkByToken(ctx context.Context, token string) (domain.PageShareLink, error) {
	var share domain.PageShareLink
	err := repository.pool.QueryRow(ctx, `
		SELECT token, page_id, access, created_by, revoked, created_at
		FROM page_share_links
		WHERE token = $1
	`, token).Scan(&share.Token, &share.PageID, &share.Access, &share.CreatedBy, &share.Revoked, &share.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PageShareLink{}, errs.ErrNotFound
		}
		return domain.PageShareLink{}, fmt.Errorf("get share link by token: %w", err)
	}
	return share, nil
}

func (repository *Repository) RevokeShareLinksByAccess(ctx context.Context, pageID domain.PageID, ownerID string, access domain.ShareAccess) error {
	_, err := repository.pool.Exec(ctx, `
		UPDATE page_share_links
		SET revoked = true
		WHERE page_id = $1 AND created_by = $2 AND access = $3 AND revoked = false
	`, string(pageID), ownerID, string(access))
	if err != nil {
		return fmt.Errorf("revoke share links: %w", err)
	}
	return nil
}

func (repository *Repository) UpdateBlocksOptimistic(ctx context.Context, pageID domain.PageID, blocks []domain.Block, expectedUpdatedAt *time.Time) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	commandTag, err := tx.Exec(ctx, `
		UPDATE pages
		SET updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND ($2::timestamptz IS NULL OR updated_at = $2)
	`, string(pageID), expectedUpdatedAt)
	if err != nil {
		return fmt.Errorf("touch page: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pages WHERE id = $1 AND deleted_at IS NULL)`, string(pageID)).Scan(&exists); err != nil {
			return fmt.Errorf("check page existence: %w", err)
		}
		if !exists {
			return errs.ErrNotFound
		}
		return errs.ErrConflict
	}

	_, err = tx.Exec(ctx, `DELETE FROM blocks WHERE page_id = $1`, string(pageID))
	if err != nil {
		return fmt.Errorf("clear blocks: %w", err)
	}

	if err := repository.insertBlocks(ctx, tx, pageID, blocks); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update blocks: %w", err)
	}
	return nil
}

func (repository *Repository) GetByID(ctx context.Context, pageID domain.PageID) (domain.Page, error) {
	var page domain.Page
	err := repository.pool.QueryRow(ctx, `
		SELECT
			p.id, p.title, p.cover, p.published, p.unlisted, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.owner_id,
			p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM page_reads r WHERE r.page_id = p.id) AS read_count,
			EXISTS(SELECT 1 FROM page_share_links s WHERE s.page_id = p.id AND s.revoked = false) AS has_share_links
		FROM pages p
		WHERE p.id = $1
	`, string(pageID)).Scan(&page.ID, &page.Title, &page.Cover, &page.Published, &page.Unlisted, &page.PublishedAt, &page.DarkMode, &page.Cinematic, &page.Mood, &page.BgColor, &page.OwnerID, &page.CreatedAt, &page.UpdatedAt, &page.DeletedAt, &page.ReadCount, &page.HasShareLinks)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Page{}, errs.ErrNotFound
		}
		return domain.Page{}, fmt.Errorf("get page by id: %w", err)
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT id, page_id, parent_id, type, position, data
		FROM blocks
		WHERE page_id = $1
		ORDER BY position
	`, string(pageID))
	if err != nil {
		return domain.Page{}, fmt.Errorf("query blocks: %w", err)
	}
	defer rows.Close()

	blocks := make([]domain.Block, 0)
	for rows.Next() {
		var block domain.Block
		var blockType string
		var data []byte
		if err := rows.Scan(&block.ID, &block.PageID, &block.ParentID, &blockType, &block.Position, &data); err != nil {
			return domain.Page{}, fmt.Errorf("scan block row: %w", err)
		}
		block.Type = domain.BlockType(blockType)
		block.Data = json.RawMessage(data)
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, fmt.Errorf("iterate blocks rows: %w", err)
	}
	page.Blocks = blocks
	return page, nil
}

func (repository *Repository) GetByIDWithAuthor(ctx context.Context, pageID domain.PageID) (domain.FeedPage, error) {
	var fp domain.FeedPage
	err := repository.pool.QueryRow(ctx, `
		SELECT
			p.id, p.title, p.cover, p.published, p.unlisted, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.owner_id,
			p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM page_reads r WHERE r.page_id = p.id) AS read_count,
			EXISTS(SELECT 1 FROM page_share_links s WHERE s.page_id = p.id AND s.revoked = false) AS has_share_links,
			COALESCE(u.username, '') AS author_username,
			COALESCE(u.display_name, '') AS author_display_name,
			COALESCE(u.avatar_url, '') AS author_avatar_url
		FROM pages p
		LEFT JOIN users u ON u.id = p.owner_id
		WHERE p.id = $1
	`, string(pageID)).Scan(
		&fp.ID, &fp.Title, &fp.Cover, &fp.Published, &fp.Unlisted, &fp.PublishedAt,
		&fp.DarkMode, &fp.Cinematic, &fp.Mood, &fp.BgColor, &fp.OwnerID,
		&fp.CreatedAt, &fp.UpdatedAt, &fp.DeletedAt,
		&fp.ReadCount, &fp.HasShareLinks,
		&fp.AuthorUsername, &fp.AuthorDisplayName, &fp.AuthorAvatarURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FeedPage{}, errs.ErrNotFound
		}
		return domain.FeedPage{}, fmt.Errorf("get page with author by id: %w", err)
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT id, page_id, parent_id, type, position, data
		FROM blocks
		WHERE page_id = $1
		ORDER BY position
	`, string(pageID))
	if err != nil {
		return domain.FeedPage{}, fmt.Errorf("query blocks: %w", err)
	}
	defer rows.Close()

	blocks := make([]domain.Block, 0)
	for rows.Next() {
		var block domain.Block
		var blockType string
		var data []byte
		if err := rows.Scan(&block.ID, &block.PageID, &block.ParentID, &blockType, &block.Position, &data); err != nil {
			return domain.FeedPage{}, fmt.Errorf("scan block row: %w", err)
		}
		block.Type = domain.BlockType(blockType)
		block.Data = json.RawMessage(data)
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return domain.FeedPage{}, fmt.Errorf("iterate blocks rows: %w", err)
	}
	fp.Blocks = blocks
	return fp, nil
}

func (repository *Repository) ListPages(ctx context.Context, ownerID string) ([]domain.Page, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT
			p.id, p.title, p.cover, p.published, p.unlisted, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.owner_id, p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) AS proofread_count,
			(SELECT count(*) FROM blocks b WHERE b.page_id = p.id) AS block_count,
			(SELECT count(*) FROM page_reads r WHERE r.page_id = p.id) AS read_count,
			EXISTS(SELECT 1 FROM page_share_links s WHERE s.page_id = p.id AND s.revoked = false) AS has_share_links
		FROM pages p
		WHERE p.deleted_at IS NULL AND p.owner_id = $1
		ORDER BY p.updated_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	pages := make([]domain.Page, 0)
	for rows.Next() {
		var page domain.Page
		if err := rows.Scan(&page.ID, &page.Title, &page.Cover, &page.Published, &page.Unlisted, &page.PublishedAt, &page.DarkMode, &page.Cinematic, &page.Mood, &page.BgColor, &page.OwnerID, &page.CreatedAt, &page.UpdatedAt, &page.DeletedAt, &page.ProofreadCount, &page.BlockCount, &page.ReadCount, &page.HasShareLinks); err != nil {
			return nil, fmt.Errorf("scan page row: %w", err)
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages rows: %w", err)
	}

	// Fetch preview blocks (first image, embed, or gallery block per page)
	if len(pages) > 0 {
		pageIDs := make([]string, len(pages))
		pageMap := make(map[string]*domain.Page, len(pages))
		for i := range pages {
			pageIDs[i] = string(pages[i].ID)
			pageMap[string(pages[i].ID)] = &pages[i]
		}

		blockRows, err := repository.pool.Query(ctx, `
			SELECT DISTINCT ON (page_id) id, page_id, parent_id, type, position, data
			FROM blocks
			WHERE page_id = ANY($1) AND type IN ('image', 'embed', 'gallery', 'music')
			ORDER BY page_id, position
		`, pageIDs)
		if err != nil {
			return nil, fmt.Errorf("query preview blocks: %w", err)
		}
		defer blockRows.Close()

		for blockRows.Next() {
			var block domain.Block
			var blockType string
			var data []byte
			if err := blockRows.Scan(&block.ID, &block.PageID, &block.ParentID, &blockType, &block.Position, &data); err != nil {
				return nil, fmt.Errorf("scan preview block: %w", err)
			}
			block.Type = domain.BlockType(blockType)
			block.Data = json.RawMessage(data)
			if p, ok := pageMap[string(block.PageID)]; ok {
				p.Blocks = []domain.Block{block}
			}
		}
		if err := blockRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate preview blocks: %w", err)
		}
	}

	return pages, nil
}

func (repository *Repository) CreateProofread(ctx context.Context, proofread domain.Proofread) error {
	annotations, err := json.Marshal(proofread.Annotations)
	if err != nil {
		return fmt.Errorf("marshal annotations: %w", err)
	}

	_, err = repository.pool.Exec(ctx, `
		INSERT INTO proofreads (id, page_id, author_name, title, summary, stance, annotations, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)
	`, string(proofread.ID), string(proofread.PageID), proofread.AuthorName, proofread.Title, proofread.Summary, proofread.Stance, annotations, proofread.CreatedAt, proofread.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert proofread: %w", err)
	}
	return nil
}

func (repository *Repository) ListProofreadsByPageID(ctx context.Context, pageID domain.PageID) ([]domain.Proofread, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id, page_id, author_name, title, summary, stance, annotations, created_at, updated_at
		FROM proofreads
		WHERE page_id = $1
		ORDER BY created_at DESC
	`, string(pageID))
	if err != nil {
		return nil, fmt.Errorf("query proofreads: %w", err)
	}
	defer rows.Close()

	proofreads := make([]domain.Proofread, 0)
	for rows.Next() {
		proofread, err := scanProofread(rows)
		if err != nil {
			return nil, err
		}
		proofreads = append(proofreads, proofread)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proofreads rows: %w", err)
	}

	return proofreads, nil
}

func (repository *Repository) GetProofreadByID(ctx context.Context, proofreadID domain.ProofreadID) (domain.Proofread, error) {
	row := repository.pool.QueryRow(ctx, `
		SELECT id, page_id, author_name, title, summary, stance, annotations, created_at, updated_at
		FROM proofreads
		WHERE id = $1
	`, string(proofreadID))

	proofread, err := scanProofread(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Proofread{}, errs.ErrNotFound
		}
		return domain.Proofread{}, err
	}

	return proofread, nil
}

func (repository *Repository) RecordOrganicRead(ctx context.Context, pageID domain.PageID, readerKey string) (bool, error) {
	if readerKey == "" {
		return false, nil
	}
	var inserted bool
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO page_reads (page_id, reader_key, read_count, first_read_at, last_read_at)
		VALUES ($1, $2, 1, now(), now())
		ON CONFLICT (page_id, reader_key)
		DO UPDATE SET
			read_count = page_reads.read_count + 1,
			last_read_at = now()
		RETURNING (xmax = 0) AS inserted
	`, string(pageID), readerKey).Scan(&inserted)
	if err != nil {
		return false, fmt.Errorf("record organic read: %w", err)
	}
	return inserted, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProofread(scanner rowScanner) (domain.Proofread, error) {
	var proofread domain.Proofread
	var annotationsRaw []byte
	if err := scanner.Scan(
		&proofread.ID,
		&proofread.PageID,
		&proofread.AuthorName,
		&proofread.Title,
		&proofread.Summary,
		&proofread.Stance,
		&annotationsRaw,
		&proofread.CreatedAt,
		&proofread.UpdatedAt,
	); err != nil {
		return domain.Proofread{}, fmt.Errorf("scan proofread row: %w", err)
	}

	if len(annotationsRaw) == 0 {
		proofread.Annotations = []domain.ProofreadAnnotation{}
		return proofread, nil
	}

	if err := json.Unmarshal(annotationsRaw, &proofread.Annotations); err != nil {
		return domain.Proofread{}, fmt.Errorf("unmarshal proofread annotations: %w", err)
	}

	return proofread, nil
}

func (repository *Repository) insertBlocks(ctx context.Context, tx pgx.Tx, pageID domain.PageID, blocks []domain.Block) error {
	for index, block := range blocks {
		blockID := block.ID
		if blockID == "" {
			blockID = uuid.NewString()
		}

		for {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocks WHERE id = $1)`, blockID).Scan(&exists); err != nil {
				return fmt.Errorf("check block id uniqueness %s: %w", blockID, err)
			}
			if !exists {
				break
			}
			blockID = uuid.NewString()
		}
		position := block.Position
		if position < 0 {
			position = index
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO blocks (id, page_id, parent_id, type, position, data, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, now(), now())
		`, blockID, string(pageID), block.ParentID, string(block.Type), position, block.Data)
		if err != nil {
			return fmt.Errorf("insert block %s: %w", blockID, err)
		}
	}
	return nil
}

func (repository *Repository) UpsertCollabUser(ctx context.Context, pageID domain.PageID, userID string, access string) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO page_collab_users (page_id, user_id, access, last_seen_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (page_id, user_id)
		DO UPDATE SET access = EXCLUDED.access, last_seen_at = now()
	`, string(pageID), userID, access)
	if err != nil {
		return fmt.Errorf("upsert collab user: %w", err)
	}
	return nil
}

func (repository *Repository) ListCollabUsers(ctx context.Context, pageID domain.PageID) ([]domain.CollabUser, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT u.id, u.username, u.display_name, u.avatar_url, pcu.access, pcu.last_seen_at
		FROM page_collab_users pcu
		JOIN users u ON u.id = pcu.user_id
		WHERE pcu.page_id = $1
		ORDER BY pcu.last_seen_at DESC
	`, string(pageID))
	if err != nil {
		return nil, fmt.Errorf("list collab users: %w", err)
	}
	defer rows.Close()

	users := make([]domain.CollabUser, 0)
	for rows.Next() {
		var cu domain.CollabUser
		if err := rows.Scan(&cu.UserID, &cu.Username, &cu.DisplayName, &cu.AvatarURL, &cu.Access, &cu.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan collab user: %w", err)
		}
		users = append(users, cu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collab users: %w", err)
	}
	return users, nil
}
