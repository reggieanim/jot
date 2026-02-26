package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reggieanim/jot/internal/modules/users/domain"
	"github.com/reggieanim/jot/internal/modules/users/ports"
	"github.com/reggieanim/jot/internal/shared/errs"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type Clock interface {
	Now() time.Time
}

// TokenIssuer abstracts JWT generation so the service stays decoupled.
type TokenIssuer interface {
	Issue(userID domain.UserID, email string) (string, error)
}

type Service struct {
	repo   ports.UserRepository
	tokens TokenIssuer
	clock  Clock
}

func NewService(repo ports.UserRepository, tokens TokenIssuer, clock Clock) *Service {
	return &Service{repo: repo, tokens: tokens, clock: clock}
}

// Signup creates a new user account.
func (s *Service) Signup(ctx context.Context, email, username, displayName, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(strings.ToLower(username))
	displayName = strings.TrimSpace(displayName)

	if email == "" || username == "" || password == "" {
		return domain.User{}, "", errs.ErrInvalidInput
	}
	if len(password) < 8 {
		return domain.User{}, "", fmt.Errorf("%w: password must be at least 8 characters", errs.ErrInvalidInput)
	}
	if len(username) < 3 {
		return domain.User{}, "", fmt.Errorf("%w: username must be at least 3 characters", errs.ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("hash password: %w", err)
	}

	now := s.clock.Now()
	user := domain.User{
		ID:           domain.UserID(uuid.NewString()),
		Email:        email,
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return domain.User{}, "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Issue(user.ID, user.Email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue token: %w", err)
	}

	return user, token, nil
}

// Login authenticates a user by email + password and returns a JWT.
func (s *Service) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", errs.ErrInvalidInput
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, "", errs.ErrInvalidInput
	}

	token, err := s.tokens.Issue(user.ID, user.Email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue token: %w", err)
	}

	return user, token, nil
}

// LoginOrSignupWithGoogle finds an existing user by email or creates a new one
// using identity data from a Google OAuth token. No password is required.
func (s *Service) LoginOrSignupWithGoogle(ctx context.Context, email, displayName, avatarURL string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return domain.User{}, "", errs.ErrInvalidInput
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err == nil {
		// Existing user — issue a new token.
		token, err := s.tokens.Issue(user.ID, user.Email)
		if err != nil {
			return domain.User{}, "", fmt.Errorf("issue token: %w", err)
		}
		return user, token, nil
	}

	// New Google user — derive a username and create the account.
	username := usernameFromEmail(email)
	now := s.clock.Now()
	newUser := domain.User{
		ID:          domain.UserID(uuid.NewString()),
		Email:       email,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if newUser.DisplayName == "" {
		newUser.DisplayName = username
	}

	if err := s.repo.Create(ctx, newUser); err != nil {
		return domain.User{}, "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Issue(newUser.ID, newUser.Email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue token: %w", err)
	}

	return newUser, token, nil
}

// usernameFromEmail derives a username from the email local part.
func usernameFromEmail(email string) string {
	for i, ch := range email {
		if ch == '@' {
			return email[:i]
		}
	}
	return email
}

// GetProfile returns the authenticated user's own profile.
func (s *Service) GetProfile(ctx context.Context, userID domain.UserID) (domain.User, error) {
	return s.repo.GetByID(ctx, userID)
}

// GetPublicProfile returns a user's public profile.
func (s *Service) GetPublicProfile(ctx context.Context, username string) (domain.PublicProfile, error) {
	return s.repo.GetPublicProfileByUsername(ctx, username)
}

// UpdateProfile updates the authenticated user's profile fields.
func (s *Service) UpdateProfile(ctx context.Context, userID domain.UserID, displayName, bio, avatarURL string) error {
	return s.repo.UpdateProfile(ctx, userID, displayName, bio, avatarURL)
}

// Follow makes followerID follow followeeID.
func (s *Service) Follow(ctx context.Context, followerID, followeeID domain.UserID) error {
	if followerID == followeeID {
		return fmt.Errorf("%w: cannot follow yourself", errs.ErrInvalidInput)
	}
	if _, err := s.repo.GetByID(ctx, followeeID); err != nil {
		return err
	}
	return s.repo.Follow(ctx, followerID, followeeID)
}

// Unfollow removes the follow relationship.
func (s *Service) Unfollow(ctx context.Context, followerID, followeeID domain.UserID) error {
	return s.repo.Unfollow(ctx, followerID, followeeID)
}

// IsFollowing checks if follower follows followee.
func (s *Service) IsFollowing(ctx context.Context, followerID, followeeID domain.UserID) (bool, error) {
	return s.repo.IsFollowing(ctx, followerID, followeeID)
}

// ListFollowers returns people who follow userID.
func (s *Service) ListFollowers(ctx context.Context, userID domain.UserID) ([]domain.PublicProfile, error) {
	return s.repo.ListFollowers(ctx, userID)
}

// ListFollowing returns people userID follows.
func (s *Service) ListFollowing(ctx context.Context, userID domain.UserID) ([]domain.PublicProfile, error) {
	return s.repo.ListFollowing(ctx, userID)
}
