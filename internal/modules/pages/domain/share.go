package domain

import "time"

type ShareAccess string

const (
	ShareAccessView ShareAccess = "view"
	ShareAccessEdit ShareAccess = "edit"
)

type PageShareLink struct {
	Token     string      `json:"token"`
	PageID    PageID      `json:"page_id"`
	Access    ShareAccess `json:"access"`
	CreatedBy string      `json:"created_by"`
	Revoked   bool        `json:"revoked"`
	CreatedAt time.Time   `json:"created_at"`
}
