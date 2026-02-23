package nats

import (
	"fmt"

	jnats "github.com/nats-io/nats.go"
)

func Connect(url string) (*jnats.Conn, jnats.JetStreamContext, error) {
	connection, err := jnats.Connect(url)
	if err != nil {
		return nil, nil, fmt.Errorf("connect nats: %w", err)
	}
	jetstream, err := connection.JetStream()
	if err != nil {
		connection.Close()
		return nil, nil, fmt.Errorf("create jetstream context: %w", err)
	}
	return connection, jetstream, nil
}

func EnsureStream(jetstream jnats.JetStreamContext, streamName, subject string) error {
	_, err := jetstream.StreamInfo(streamName)
	if err == nil {
		return nil
	}
	if _, err := jetstream.AddStream(&jnats.StreamConfig{Name: streamName, Subjects: []string{subject}}); err != nil {
		return fmt.Errorf("add stream: %w", err)
	}
	return nil
}
