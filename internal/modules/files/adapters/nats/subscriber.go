package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	jnats "github.com/nats-io/nats.go"
	"github.com/reggieanim/jot/internal/modules/files/app"
	"go.uber.org/zap"
)

type pageDeletedEnvelope struct {
	Type string `json:"type"`
	Page struct {
		ID     string            `json:"id"`
		Cover  *string           `json:"cover,omitempty"`
		Blocks []json.RawMessage `json:"blocks"`
	} `json:"page"`
}

type Subscriber struct {
	service *app.Service
	conn    *jnats.Conn
	subject string
	logger  *zap.Logger
	sub     *jnats.Subscription
}

func NewSubscriber(service *app.Service, conn *jnats.Conn, subject string, logger *zap.Logger) *Subscriber {
	return &Subscriber{
		service: service,
		conn:    conn,
		subject: subject,
		logger:  logger,
	}
}

func (s *Subscriber) Start() error {
	sub, err := s.conn.Subscribe(s.subject, func(msg *jnats.Msg) {
		envelope, err := parsePageDeleted(msg.Data)
		if err != nil {
			return
		}

		s.logger.Info("received page.deleted event",
			zap.String("page_id", envelope.Page.ID),
		)

		s.service.HandlePageDeleted(context.Background(), envelope.Page.Cover, envelope.Page.Blocks)
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", s.subject, err)
	}
	s.sub = sub
	s.logger.Info("files subscriber started", zap.String("subject", s.subject))
	return nil
}

func (s *Subscriber) Stop() error {
	if s.sub != nil {
		return s.sub.Unsubscribe()
	}
	return nil
}

func parsePageDeleted(data []byte) (pageDeletedEnvelope, error) {
	var envelope pageDeletedEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return pageDeletedEnvelope{}, fmt.Errorf("unmarshal event: %w", err)
	}
	if !strings.HasPrefix(envelope.Type, "page.deleted") {
		return pageDeletedEnvelope{}, fmt.Errorf("not a page.deleted event: %s", envelope.Type)
	}
	return envelope, nil
}
