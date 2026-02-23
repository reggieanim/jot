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
	Title          string     `json:"title"`
	Cover          *string    `json:"cover,omitempty"`
	Published      bool       `json:"published"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	DarkMode       bool       `json:"dark_mode"`
	Cinematic      bool       `json:"cinematic"`
	Mood           int        `json:"mood"`
	BgColor        string     `json:"bg_color"`
	Blocks         []Block    `json:"blocks"`
	ProofreadCount int        `json:"proofread_count"`
	BlockCount     int        `json:"block_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}
