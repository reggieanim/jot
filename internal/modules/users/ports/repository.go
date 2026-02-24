package ports

import (
	"context"

	"github.com/reggieanim/jot/internal/modules/users/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	GetByID(ctx context.Context, id domain.UserID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, error)
	UpdateProfile(ctx context.Context, id domain.UserID, displayName, bio, avatarURL string) error

	Follow(ctx context.Context, followerID, followeeID domain.UserID) error
	Unfollow(ctx context.Context, followerID, followeeID domain.UserID) error
	IsFollowing(ctx context.Context, followerID, followeeID domain.UserID) (bool, error)
	ListFollowers(ctx context.Context, userID domain.UserID) ([]domain.PublicProfile, error)
	ListFollowing(ctx context.Context, userID domain.UserID) ([]domain.PublicProfile, error)
	GetPublicProfile(ctx context.Context, userID domain.UserID) (domain.PublicProfile, error)
	GetPublicProfileByUsername(ctx context.Context, username string) (domain.PublicProfile, error)
}
