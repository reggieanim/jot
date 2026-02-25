package domain

import (
	"encoding/json"
	"time"
)

type PageID string

type BlockType string

const (
	BlockTypeParagraph BlockType = "paragraph"
	BlockTypeImage     BlockType = "image"
)

type Block struct {
	ID       string          `json:"id"`
	PageID   PageID          `json:"page_id,omitempty"`
	ParentID *string         `json:"parent_id,omitempty"`
	Type     BlockType       `json:"type"`
	Position int             `json:"position"`
	Data     json.RawMessage `json:"data"`
}

type Page struct {
	ID             PageID     `json:"id"`
	OwnerID        *string    `json:"owner_id,omitempty"`
	Title          string     `json:"title"`
	Cover          *string    `json:"cover,omitempty"`
	Published      bool       `json:"published"`
	Unlisted       bool       `json:"unlisted"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	DarkMode       bool       `json:"dark_mode"`
	Cinematic      bool       `json:"cinematic"`
	Mood           int        `json:"mood"`
	BgColor        string     `json:"bg_color"`
	Blocks         []Block    `json:"blocks"`
	ProofreadCount int        `json:"proofread_count"`
	BlockCount     int        `json:"block_count"`
	ReadCount      int        `json:"read_count"`
	HasShareLinks  bool       `json:"has_share_links"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// FeedPage extends Page with author info for the public feed.
type FeedPage struct {
	Page
	AuthorUsername    string `json:"author_username"`
	AuthorDisplayName string `json:"author_display_name"`
	AuthorAvatarURL   string `json:"author_avatar_url"`
}

// CollabUser represents a signed-in user who has accessed a page via share link.
type CollabUser struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Access      string    `json:"access"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
