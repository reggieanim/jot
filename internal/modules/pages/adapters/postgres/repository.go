package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		INSERT INTO pages (id, title, cover, dark_mode, cinematic, mood, bg_color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, string(page.ID), page.Title, page.Cover, page.DarkMode, page.Cinematic, page.Mood, page.BgColor, page.CreatedAt, page.UpdatedAt)
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

func (repository *Repository) SetPublished(ctx context.Context, pageID domain.PageID, published bool) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE pages
		SET published = $2,
		    published_at = CASE WHEN $2 THEN now() ELSE NULL END,
		    updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, string(pageID), published)
	if err != nil {
		return fmt.Errorf("set published: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return errs.ErrNotFound
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
		SELECT id, title, cover, published, published_at, dark_mode, cinematic, mood, bg_color, created_at, updated_at, deleted_at
		FROM pages
		WHERE id = $1
	`, string(pageID)).Scan(&page.ID, &page.Title, &page.Cover, &page.Published, &page.PublishedAt, &page.DarkMode, &page.Cinematic, &page.Mood, &page.BgColor, &page.CreatedAt, &page.UpdatedAt, &page.DeletedAt)
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

func (repository *Repository) ListPages(ctx context.Context) ([]domain.Page, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT
			p.id, p.title, p.cover, p.published, p.published_at,
			p.dark_mode, p.cinematic, p.mood, p.bg_color, p.created_at, p.updated_at, p.deleted_at,
			(SELECT count(*) FROM proofreads pr WHERE pr.page_id = p.id) AS proofread_count,
			(SELECT count(*) FROM blocks b WHERE b.page_id = p.id) AS block_count
		FROM pages p
		WHERE p.deleted_at IS NULL
		ORDER BY p.updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	pages := make([]domain.Page, 0)
	for rows.Next() {
		var page domain.Page
		if err := rows.Scan(&page.ID, &page.Title, &page.Cover, &page.Published, &page.PublishedAt, &page.DarkMode, &page.Cinematic, &page.Mood, &page.BgColor, &page.CreatedAt, &page.UpdatedAt, &page.DeletedAt, &page.ProofreadCount, &page.BlockCount); err != nil {
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
			WHERE page_id = ANY($1) AND type IN ('image', 'embed', 'gallery')
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
