package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggieanim/jot/internal/modules/users/domain"
	"github.com/reggieanim/jot/internal/shared/errs"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, user domain.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, username, display_name, bio, avatar_url, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, string(user.ID), user.Email, user.Username, user.DisplayName, user.Bio, user.AvatarURL, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, username, display_name, bio, avatar_url, password_hash, created_at, updated_at
		FROM users WHERE id = $1
	`, string(id))
	return r.scanUser(row)
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, username, display_name, bio, avatar_url, password_hash, created_at, updated_at
		FROM users WHERE email = $1
	`, email)
	return r.scanUser(row)
}

func (r *Repository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, username, display_name, bio, avatar_url, password_hash, created_at, updated_at
		FROM users WHERE username = $1
	`, username)
	return r.scanUser(row)
}

func (r *Repository) UpdateProfile(ctx context.Context, id domain.UserID, displayName, bio, avatarURL string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE users SET display_name = $2, bio = $3, avatar_url = $4, updated_at = now()
		WHERE id = $1
	`, string(id), displayName, bio, avatarURL)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return nil
}

func (r *Repository) Follow(ctx context.Context, followerID, followeeID domain.UserID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, string(followerID), string(followeeID))
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	return nil
}

func (r *Repository) Unfollow(ctx context.Context, followerID, followeeID domain.UserID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2
	`, string(followerID), string(followeeID))
	if err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}
	return nil
}

func (r *Repository) IsFollowing(ctx context.Context, followerID, followeeID domain.UserID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)
	`, string(followerID), string(followeeID)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is following: %w", err)
	}
	return exists, nil
}

func (r *Repository) ListFollowers(ctx context.Context, userID domain.UserID) ([]domain.PublicProfile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url,
		       (SELECT COUNT(*) FROM follows WHERE followee_id = u.id) AS follower_count,
		       (SELECT COUNT(*) FROM follows WHERE follower_id = u.id) AS follow_count
		FROM follows f
		JOIN users u ON u.id = f.follower_id
		WHERE f.followee_id = $1
		ORDER BY f.created_at DESC
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("list followers: %w", err)
	}
	defer rows.Close()
	return r.scanProfiles(rows)
}

func (r *Repository) ListFollowing(ctx context.Context, userID domain.UserID) ([]domain.PublicProfile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url,
		       (SELECT COUNT(*) FROM follows WHERE followee_id = u.id) AS follower_count,
		       (SELECT COUNT(*) FROM follows WHERE follower_id = u.id) AS follow_count
		FROM follows f
		JOIN users u ON u.id = f.followee_id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("list following: %w", err)
	}
	defer rows.Close()
	return r.scanProfiles(rows)
}

func (r *Repository) GetPublicProfile(ctx context.Context, userID domain.UserID) (domain.PublicProfile, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url,
		       (SELECT COUNT(*) FROM follows WHERE followee_id = u.id) AS follower_count,
		       (SELECT COUNT(*) FROM follows WHERE follower_id = u.id) AS follow_count
		FROM users u
		WHERE u.id = $1
	`, string(userID))
	var p domain.PublicProfile
	err := row.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.FollowerCount, &p.FollowCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PublicProfile{}, errs.ErrNotFound
		}
		return domain.PublicProfile{}, fmt.Errorf("get public profile: %w", err)
	}
	return p, nil
}

func (r *Repository) GetPublicProfileByUsername(ctx context.Context, username string) (domain.PublicProfile, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.display_name, u.bio, u.avatar_url,
		       (SELECT COUNT(*) FROM follows WHERE followee_id = u.id) AS follower_count,
		       (SELECT COUNT(*) FROM follows WHERE follower_id = u.id) AS follow_count
		FROM users u
		WHERE u.username = $1
	`, username)
	var p domain.PublicProfile
	err := row.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.FollowerCount, &p.FollowCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PublicProfile{}, errs.ErrNotFound
		}
		return domain.PublicProfile{}, fmt.Errorf("get public profile by username: %w", err)
	}
	return p, nil
}

func (r *Repository) scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.Bio, &u.AvatarURL, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, errs.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

func (r *Repository) scanProfiles(rows pgx.Rows) ([]domain.PublicProfile, error) {
	var profiles []domain.PublicProfile
	for rows.Next() {
		var p domain.PublicProfile
		if err := rows.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Bio, &p.AvatarURL, &p.FollowerCount, &p.FollowCount); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}
