package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jnats "github.com/nats-io/nats.go"
	"github.com/reggieanim/jot/internal/modules/pages/domain"
)

type PageEventsPublisher struct {
	jetstream jnats.JetStreamContext
	subject   string
}

type pageEvent struct {
	Type      string      `json:"type"`
	Page      domain.Page `json:"page"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewPageEventsPublisher(jetstream jnats.JetStreamContext, subject string) *PageEventsPublisher {
	return &PageEventsPublisher{jetstream: jetstream, subject: subject}
}

func (publisher *PageEventsPublisher) PageCreated(_ context.Context, page domain.Page) error {
	return publisher.publish("page.created", page)
}

func (publisher *PageEventsPublisher) BlocksUpdated(_ context.Context, page domain.Page) error {
	return publisher.publish("page.blocks.updated", page)
}

func (publisher *PageEventsPublisher) publish(eventType string, page domain.Page) error {
	payload, err := json.Marshal(pageEvent{Type: eventType, Page: page, Timestamp: time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("marshal page event: %w", err)
	}
	if _, err := publisher.jetstream.Publish(publisher.subject, payload); err != nil {
		return fmt.Errorf("publish page event: %w", err)
	}
	return nil
}
