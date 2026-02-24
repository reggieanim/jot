package domain

import "time"

type UserID string

type User struct {
	ID           UserID    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Bio          string    `json:"bio"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PublicProfile is the view of a user visible to others.
type PublicProfile struct {
	ID            UserID `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	Bio           string `json:"bio"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	FollowerCount int    `json:"follower_count"`
	FollowCount   int    `json:"follow_count"`
}

type Follow struct {
	FollowerID UserID    `json:"follower_id"`
	FolloweeID UserID    `json:"followee_id"`
	CreatedAt  time.Time `json:"created_at"`
}
