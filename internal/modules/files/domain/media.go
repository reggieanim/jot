package domain

// MediaRef represents a reference to a stored media object.
type MediaRef struct {
	// ObjectKey is the storage-relative key (e.g. "images/abc-123.png").
	ObjectKey string
}
