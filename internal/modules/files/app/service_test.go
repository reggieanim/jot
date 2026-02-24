package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"go.uber.org/zap"
)

type mockMediaStore struct {
	mu           sync.Mutex
	deleted      []string
	urlToKey     map[string]string
	failOnDelete map[string]bool
}

func newMockMediaStore() *mockMediaStore {
	return &mockMediaStore{
		deleted:      make([]string, 0),
		urlToKey:     make(map[string]string),
		failOnDelete: make(map[string]bool),
	}
}

func (m *mockMediaStore) DeleteObject(_ context.Context, objectKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOnDelete[objectKey] {
		return fmt.Errorf("simulated delete failure for %s", objectKey)
	}
	m.deleted = append(m.deleted, objectKey)
	return nil
}

func (m *mockMediaStore) ObjectKeyFromURL(rawURL string) string {
	return m.urlToKey[rawURL]
}

func (m *mockMediaStore) addMapping(url, key string) {
	m.urlToKey[url] = key
}

func (m *mockMediaStore) deletedKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.deleted))
	copy(cp, m.deleted)
	return cp
}

func testLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

func TestHandlePageDeleted_ImageBlock(t *testing.T) {
	store := newMockMediaStore()
	store.addMapping("http://s3.local/bucket/images/abc.png", "images/abc.png")
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"image","data":{"url":"http://s3.local/bucket/images/abc.png"}}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 1 {
		t.Fatalf("expected 1 deletion, got %d", len(deleted))
	}
	if deleted[0] != "images/abc.png" {
		t.Fatalf("expected key images/abc.png, got %s", deleted[0])
	}
}

func TestHandlePageDeleted_CoverImage(t *testing.T) {
	store := newMockMediaStore()
	store.addMapping("http://s3.local/bucket/images/cover.jpg", "images/cover.jpg")
	svc := NewService(store, testLogger())

	cover := "http://s3.local/bucket/images/cover.jpg"
	svc.HandlePageDeleted(context.Background(), &cover, nil)

	deleted := store.deletedKeys()
	if len(deleted) != 1 {
		t.Fatalf("expected 1 deletion, got %d", len(deleted))
	}
	if deleted[0] != "images/cover.jpg" {
		t.Fatalf("expected key images/cover.jpg, got %s", deleted[0])
	}
}

func TestHandlePageDeleted_GalleryBlock(t *testing.T) {
	store := newMockMediaStore()
	store.addMapping("http://s3.local/bucket/images/g1.png", "images/g1.png")
	store.addMapping("http://s3.local/bucket/images/g2.png", "images/g2.png")
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"gallery","data":{"items":[{"kind":"image","value":"http://s3.local/bucket/images/g1.png"},{"kind":"image","value":"http://s3.local/bucket/images/g2.png"},{"kind":"embed","value":"https://youtube.com/watch?v=123"}]}}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deletions, got %d: %v", len(deleted), deleted)
	}
}

func TestHandlePageDeleted_LegacyImagesArray(t *testing.T) {
	store := newMockMediaStore()
	store.addMapping("http://s3.local/bucket/images/old1.png", "images/old1.png")
	store.addMapping("http://s3.local/bucket/images/old2.png", "images/old2.png")
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"image","data":{"images":["http://s3.local/bucket/images/old1.png","http://s3.local/bucket/images/old2.png"]}}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deletions, got %d: %v", len(deleted), deleted)
	}
}

func TestHandlePageDeleted_MixedBlocksAndCover(t *testing.T) {
	store := newMockMediaStore()
	store.addMapping("http://s3.local/bucket/images/cover.jpg", "images/cover.jpg")
	store.addMapping("http://s3.local/bucket/images/img1.png", "images/img1.png")
	store.addMapping("http://s3.local/bucket/images/img2.png", "images/img2.png")
	svc := NewService(store, testLogger())

	cover := "http://s3.local/bucket/images/cover.jpg"
	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"paragraph","data":{"text":"hello world"}}`),
		json.RawMessage(`{"type":"image","data":{"url":"http://s3.local/bucket/images/img1.png"}}`),
		json.RawMessage(`{"type":"gallery","data":{"items":[{"kind":"image","value":"http://s3.local/bucket/images/img2.png"}]}}`),
	}
	svc.HandlePageDeleted(context.Background(), &cover, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 3 {
		t.Fatalf("expected 3 deletions, got %d: %v", len(deleted), deleted)
	}
}

func TestHandlePageDeleted_NoMedia(t *testing.T) {
	store := newMockMediaStore()
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"paragraph","data":{"text":"just text"}}`),
		json.RawMessage(`{"type":"heading","data":{"text":"title"}}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deletions, got %d: %v", len(deleted), deleted)
	}
}

func TestHandlePageDeleted_ExternalURLsIgnored(t *testing.T) {
	store := newMockMediaStore()
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"image","data":{"url":"https://external.com/photo.jpg"}}`),
		json.RawMessage(`{"type":"embed","data":{"url":"https://youtube.com/watch?v=xyz"}}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deletions for external URLs, got %d: %v", len(deleted), deleted)
	}
}

func TestHandlePageDeleted_DeleteFailureContinues(t *testing.T) {
	store := newMockMediaStore()
	store.addMapping("http://s3.local/bucket/images/fail.png", "images/fail.png")
	store.addMapping("http://s3.local/bucket/images/ok.png", "images/ok.png")
	store.failOnDelete["images/fail.png"] = true
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"image","data":{"url":"http://s3.local/bucket/images/fail.png"}}`),
		json.RawMessage(`{"type":"image","data":{"url":"http://s3.local/bucket/images/ok.png"}}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 1 {
		t.Fatalf("expected 1 successful deletion, got %d: %v", len(deleted), deleted)
	}
	if deleted[0] != "images/ok.png" {
		t.Fatalf("expected images/ok.png, got %s", deleted[0])
	}
}

func TestHandlePageDeleted_MalformedBlocksIgnored(t *testing.T) {
	store := newMockMediaStore()
	svc := NewService(store, testLogger())

	blocks := []json.RawMessage{
		json.RawMessage(`not valid json`),
		json.RawMessage(`{"type":"image"}`),
		json.RawMessage(`{"type":"image","data":"not an object"}`),
	}
	svc.HandlePageDeleted(context.Background(), nil, blocks)

	deleted := store.deletedKeys()
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deletions for malformed blocks, got %d: %v", len(deleted), deleted)
	}
}

func TestHandlePageDeleted_EmptyCoverIgnored(t *testing.T) {
	store := newMockMediaStore()
	svc := NewService(store, testLogger())

	empty := ""
	svc.HandlePageDeleted(context.Background(), &empty, nil)

	deleted := store.deletedKeys()
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deletions for empty cover, got %d", len(deleted))
	}
}
