package app

import (
	"context"
	"encoding/json"

	"github.com/reggieanim/jot/internal/modules/files/domain"
	"github.com/reggieanim/jot/internal/modules/files/ports"
	"go.uber.org/zap"
)

type Service struct {
	media  ports.MediaStore
	logger *zap.Logger
}

func NewService(media ports.MediaStore, logger *zap.Logger) *Service {
	return &Service{media: media, logger: logger}
}

func (s *Service) HandlePageDeleted(ctx context.Context, cover *string, rawBlocks []json.RawMessage) {
	refs := s.extractRefs(cover, rawBlocks)
	if len(refs) == 0 {
		return
	}

	s.logger.Info("cleaning up media for deleted page",
		zap.Int("object_count", len(refs)),
	)

	for _, ref := range refs {
		if err := s.media.DeleteObject(ctx, ref.ObjectKey); err != nil {
			s.logger.Warn("failed to delete stored object",
				zap.String("key", ref.ObjectKey),
				zap.Error(err),
			)
		}
	}
}

func (s *Service) extractRefs(cover *string, rawBlocks []json.RawMessage) []domain.MediaRef {
	var refs []domain.MediaRef

	if cover != nil && *cover != "" {
		if key := s.media.ObjectKeyFromURL(*cover); key != "" {
			refs = append(refs, domain.MediaRef{ObjectKey: key})
		}
	}

	for _, raw := range rawBlocks {
		refs = append(refs, s.extractBlockRefs(raw)...)
	}

	return refs
}

func (s *Service) extractBlockRefs(blockJSON json.RawMessage) []domain.MediaRef {
	var block struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(blockJSON, &block); err != nil || block.Data == nil {
		return nil
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(block.Data, &data); err != nil {
		return nil
	}

	var refs []domain.MediaRef

	if raw, ok := data["url"]; ok {
		var u string
		if json.Unmarshal(raw, &u) == nil && u != "" {
			if key := s.media.ObjectKeyFromURL(u); key != "" {
				refs = append(refs, domain.MediaRef{ObjectKey: key})
			}
		}
	}

	if raw, ok := data["items"]; ok {
		var items []struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		}
		if json.Unmarshal(raw, &items) == nil {
			for _, item := range items {
				if item.Kind == "image" && item.Value != "" {
					if key := s.media.ObjectKeyFromURL(item.Value); key != "" {
						refs = append(refs, domain.MediaRef{ObjectKey: key})
					}
				}
			}
		}
	}

	if raw, ok := data["images"]; ok {
		var urls []string
		if json.Unmarshal(raw, &urls) == nil {
			for _, u := range urls {
				if u != "" {
					if key := s.media.ObjectKeyFromURL(u); key != "" {
						refs = append(refs, domain.MediaRef{ObjectKey: key})
					}
				}
			}
		}
	}

	return refs
}
