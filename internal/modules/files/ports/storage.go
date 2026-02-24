package ports

import (
	"context"
)

// MediaStore abstracts object storage operations needed by the files module.
type MediaStore interface {
	// DeleteObject removes an object by its storage key.
	DeleteObject(ctx context.Context, objectKey string) error
	// ObjectKeyFromURL extracts the storage key from a public URL.
	// Returns empty string if the URL doesn't belong to this store.
	ObjectKeyFromURL(rawURL string) string
}
