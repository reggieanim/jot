package app

import (
	"context"
	"testing"
	"time"

	"github.com/reggieanim/jot/internal/modules/users/domain"
	"github.com/reggieanim/jot/internal/shared/errs"
)

// --- fakes ---

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeTokenIssuer struct{}

func (f fakeTokenIssuer) Issue(userID domain.UserID, email string) (string, error) {
	return "fake-jwt-" + string(userID), nil
}

type inMemoryUserRepo struct {
	users   []domain.User
	follows []domain.Follow
}

func (r *inMemoryUserRepo) Create(_ context.Context, user domain.User) error {
	for _, u := range r.users {
		if u.Email == user.Email {
			return errs.ErrConflict
		}
		if u.Username == user.Username {
			return errs.ErrConflict
		}
	}
	r.users = append(r.users, user)
	return nil
}

func (r *inMemoryUserRepo) GetByID(_ context.Context, id domain.UserID) (domain.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.User{}, errs.ErrNotFound
}

func (r *inMemoryUserRepo) GetByEmail(_ context.Context, email string) (domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return domain.User{}, errs.ErrNotFound
}

func (r *inMemoryUserRepo) GetByUsername(_ context.Context, username string) (domain.User, error) {
	for _, u := range r.users {
		if u.Username == username {
			return u, nil
		}
	}
	return domain.User{}, errs.ErrNotFound
}

func (r *inMemoryUserRepo) UpdateProfile(_ context.Context, id domain.UserID, displayName, bio, avatarURL string) error {
	for i, u := range r.users {
		if u.ID == id {
			r.users[i].DisplayName = displayName
			r.users[i].Bio = bio
			r.users[i].AvatarURL = avatarURL
			return nil
		}
	}
	return errs.ErrNotFound
}

func (r *inMemoryUserRepo) Follow(_ context.Context, followerID, followeeID domain.UserID) error {
	for _, f := range r.follows {
		if f.FollowerID == followerID && f.FolloweeID == followeeID {
			return nil
		}
	}
	r.follows = append(r.follows, domain.Follow{FollowerID: followerID, FolloweeID: followeeID, CreatedAt: time.Now()})
	return nil
}

func (r *inMemoryUserRepo) Unfollow(_ context.Context, followerID, followeeID domain.UserID) error {
	for i, f := range r.follows {
		if f.FollowerID == followerID && f.FolloweeID == followeeID {
			r.follows = append(r.follows[:i], r.follows[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *inMemoryUserRepo) IsFollowing(_ context.Context, followerID, followeeID domain.UserID) (bool, error) {
	for _, f := range r.follows {
		if f.FollowerID == followerID && f.FolloweeID == followeeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *inMemoryUserRepo) ListFollowers(_ context.Context, userID domain.UserID) ([]domain.PublicProfile, error) {
	var result []domain.PublicProfile
	for _, f := range r.follows {
		if f.FolloweeID == userID {
			for _, u := range r.users {
				if u.ID == f.FollowerID {
					result = append(result, domain.PublicProfile{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName})
				}
			}
		}
	}
	return result, nil
}

func (r *inMemoryUserRepo) ListFollowing(_ context.Context, userID domain.UserID) ([]domain.PublicProfile, error) {
	var result []domain.PublicProfile
	for _, f := range r.follows {
		if f.FollowerID == userID {
			for _, u := range r.users {
				if u.ID == f.FolloweeID {
					result = append(result, domain.PublicProfile{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName})
				}
			}
		}
	}
	return result, nil
}

func (r *inMemoryUserRepo) GetPublicProfile(_ context.Context, userID domain.UserID) (domain.PublicProfile, error) {
	for _, u := range r.users {
		if u.ID == userID {
			return domain.PublicProfile{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName}, nil
		}
	}
	return domain.PublicProfile{}, errs.ErrNotFound
}

func (r *inMemoryUserRepo) GetPublicProfileByUsername(_ context.Context, username string) (domain.PublicProfile, error) {
	for _, u := range r.users {
		if u.Username == username {
			return domain.PublicProfile{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName}, nil
		}
	}
	return domain.PublicProfile{}, errs.ErrNotFound
}

// --- tests ---

func newTestService() (*Service, *inMemoryUserRepo) {
	repo := &inMemoryUserRepo{}
	svc := NewService(repo, fakeTokenIssuer{}, fakeClock{now: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
	return svc, repo
}

func TestSignup_Success(t *testing.T) {
	svc, repo := newTestService()
	user, token, err := svc.Signup(context.Background(), "alice@example.com", "alice", "Alice", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", user.Email)
	}
	if user.Username != "alice" {
		t.Errorf("expected username alice, got %s", user.Username)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if len(repo.users) != 1 {
		t.Errorf("expected 1 user in repo, got %d", len(repo.users))
	}
}

func TestSignup_DefaultDisplayName(t *testing.T) {
	svc, _ := newTestService()
	user, _, err := svc.Signup(context.Background(), "bob@example.com", "bob", "", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.DisplayName != "bob" {
		t.Errorf("expected display name 'bob', got '%s'", user.DisplayName)
	}
}

func TestSignup_EmptyEmail(t *testing.T) {
	svc, _ := newTestService()
	_, _, err := svc.Signup(context.Background(), "", "alice", "Alice", "password123")
	if err == nil {
		t.Fatal("expected error for empty email")
	}
}

func TestSignup_ShortPassword(t *testing.T) {
	svc, _ := newTestService()
	_, _, err := svc.Signup(context.Background(), "alice@example.com", "alice", "Alice", "short")
	if err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestSignup_ShortUsername(t *testing.T) {
	svc, _ := newTestService()
	_, _, err := svc.Signup(context.Background(), "alice@example.com", "al", "Alice", "password123")
	if err == nil {
		t.Fatal("expected error for short username")
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _ := newTestService()
	_, _, err := svc.Signup(context.Background(), "alice@example.com", "alice", "Alice", "password123")
	if err != nil {
		t.Fatalf("signup error: %v", err)
	}

	user, token, err := svc.Login(context.Background(), "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", user.Email)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newTestService()
	_, _, _ = svc.Signup(context.Background(), "alice@example.com", "alice", "Alice", "password123")

	_, _, err := svc.Login(context.Background(), "alice@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLogin_NotFound(t *testing.T) {
	svc, _ := newTestService()
	_, _, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestFollow_And_Unfollow(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, _, _ = svc.Signup(ctx, "alice@example.com", "alice", "Alice", "password123")
	bob, _, _ := svc.Signup(ctx, "bob@example.com", "bob", "Bob", "password123")

	alice, _ := svc.GetPublicProfile(ctx, "alice")

	// Bob follows Alice
	err := svc.Follow(ctx, bob.ID, alice.ID)
	if err != nil {
		t.Fatalf("follow error: %v", err)
	}

	following, err := svc.IsFollowing(ctx, bob.ID, alice.ID)
	if err != nil {
		t.Fatalf("is following error: %v", err)
	}
	if !following {
		t.Error("expected Bob to be following Alice")
	}

	// List Alice's followers
	followers, err := svc.ListFollowers(ctx, alice.ID)
	if err != nil {
		t.Fatalf("list followers error: %v", err)
	}
	if len(followers) != 1 {
		t.Errorf("expected 1 follower, got %d", len(followers))
	}

	// Unfollow
	err = svc.Unfollow(ctx, bob.ID, alice.ID)
	if err != nil {
		t.Fatalf("unfollow error: %v", err)
	}

	following, _ = svc.IsFollowing(ctx, bob.ID, alice.ID)
	if following {
		t.Error("expected Bob to NOT be following Alice after unfollow")
	}
}

func TestFollow_Self(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	user, _, _ := svc.Signup(ctx, "alice@example.com", "alice", "Alice", "password123")

	err := svc.Follow(ctx, user.ID, user.ID)
	if err == nil {
		t.Fatal("expected error when following self")
	}
}

func TestGetPublicProfile(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	_, _, _ = svc.Signup(ctx, "alice@example.com", "alice", "Alice", "password123")

	profile, err := svc.GetPublicProfile(ctx, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Username != "alice" {
		t.Errorf("expected username alice, got %s", profile.Username)
	}
}

func TestGetPublicProfile_NotFound(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.GetPublicProfile(context.Background(), "nobody")
	if err == nil {
		t.Fatal("expected error for non-existent profile")
	}
}

func TestUpdateProfile(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	user, _, _ := svc.Signup(ctx, "alice@example.com", "alice", "Alice", "password123")

	err := svc.UpdateProfile(ctx, user.ID, "Alice W.", "Hello world", "https://example.com/avatar.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := svc.GetProfile(ctx, user.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.DisplayName != "Alice W." {
		t.Errorf("expected display name 'Alice W.', got '%s'", updated.DisplayName)
	}
	if updated.Bio != "Hello world" {
		t.Errorf("expected bio 'Hello world', got '%s'", updated.Bio)
	}
}
