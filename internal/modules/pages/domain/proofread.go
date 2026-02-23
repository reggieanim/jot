package domain

import "time"

type ProofreadID string

type ProofreadAnnotation struct {
	ID      string `json:"id"`
	BlockID string `json:"block_id"`
	Kind    string `json:"kind"`
	Quote   string `json:"quote"`
	Text    string `json:"text"`
}

type Proofread struct {
	ID          ProofreadID           `json:"id"`
	PageID      PageID                `json:"page_id"`
	AuthorName  string                `json:"author_name"`
	Title       string                `json:"title"`
	Summary     string                `json:"summary"`
	Stance      string                `json:"stance"`
	Annotations []ProofreadAnnotation `json:"annotations"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}
