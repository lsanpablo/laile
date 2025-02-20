package forwarders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"laile/internal/config"
)

type RMQForwarder struct {
	Session    *session
	Config     *config.Forwarder
	ErrorCount int
	mu         sync.Mutex // Add mutex for connection management
}

type message []byte
type session struct {
	*amqp091.Connection
	*amqp091.Channel
}

// Close tears the connection down, taking the channel with it.
func (s *session) Close() error {
	if s.Connection == nil {
		return nil
	}
	return s.Connection.Close()
}

func NewRMQForwarder(config *config.Forwarder) *RMQForwarder {
	return &RMQForwarder{
		Config: config,
	}
}

func (f *RMQForwarder) declareExchange() error {
	session, err := f.getSession()
	if err != nil {
		return err
	}

	return session.Channel.ExchangeDeclare(
		f.Config.Exchange,     // name
		f.Config.ExchangeType, // type
		f.Config.Durable,      // durable
		f.Config.AutoDelete,   // auto-deleted
		f.Config.Internal,     // internal
		f.Config.NoWait,       // no-wait
		nil,                   // arguments
	)
}

func (f *RMQForwarder) Forward(ctx context.Context, deliveryAttempt *DeliveryAttempt) (*DeliveryResult, error) {
	payload, err := webhookToAMQPBody(deliveryAttempt, f.Config.Persistent)
	if err != nil {
		return nil, err
	}

	timeoutContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Single attempt to publish with proper error handling
	err = f.publishToRMQ(timeoutContext, payload)
	if err != nil {
		slog.Error("producer: error publishing message", "error", err)
		return nil, err
	}

	return &DeliveryResult{
		StatusCode: 200,
		Headers:    map[string][]string{},
		Body:       nil,
	}, nil
}

type publishConfig struct {
	ExchangeName string
	RoutingKey   string
}

func (f *RMQForwarder) publishToRMQ(ctx context.Context, payload message) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	rmqSession, err := f.getSession()
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	err = rmqSession.publish(ctx, payload, &publishConfig{
		ExchangeName: f.Config.Exchange,
		RoutingKey:   f.Config.RoutingKey,
	})
	if err != nil {
		// Only dispose session on connection errors
		if amqpErr, ok := err.(*amqp091.Error); ok && amqpErr.Code >= 300 {
			_ = f.disposeSession()
		}
		return err
	}
	return nil
}

type AMQPBody struct {
	Headers        map[string][]string `json:"headers"`
	Body           json.RawMessage     `json:"body,omitempty"`
	QueryParams    map[string][]string `json:"query_params"`
	Method         string              `json:"method"`
	Url            string              `json:"url"`
	IdempotencyKey string              `json:"idempotency_key"`
	HelpText       string              `json:"help_text,omitempty"`
}

// webhookToAMQPBody marshals the DeliveryAttempt into a JSON byte slice.
func webhookToAMQPBody(attempt *DeliveryAttempt, includeHelpText bool) (message, error) {
	var headers map[string][]string
	if err := json.Unmarshal(attempt.Headers, &headers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
	}

	var queryParams map[string][]string
	if err := json.Unmarshal(attempt.QueryParams, &queryParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query params: %w", err)
	}

	amqpBody := AMQPBody{
		Headers:        headers,
		Body:           *attempt.Body,
		QueryParams:    queryParams,
		Method:         attempt.Method,
		Url:            attempt.Url,
		IdempotencyKey: attempt.IdempotencyKey,
	}

	if includeHelpText {
		amqpBody.HelpText = "body is a raw JSON message, headers and query parameters are key-value maps"
	}

	return json.Marshal(amqpBody)
}

func (f *RMQForwarder) getSession() (*session, error) {
	if f.Session != nil && !f.Session.IsClosed() {
		return f.Session, nil
	}

	if err := f.createSession(); err != nil {
		return nil, err
	}
	return f.Session, nil
}

func (s *session) IsClosed() bool {
	return s.Connection == nil || s.Connection.IsClosed()
}

func (f *RMQForwarder) createSession() error {
	conn, err := amqp091.Dial(f.Config.ConnectionURL)
	slog.Info("dialing", "url", f.Config.ConnectionURL)
	if err != nil {
		slog.Error("cannot dial", "error", err, "url", f.Config.ConnectionURL)
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		slog.Error("cannot create channel", "error", err)
		return err
	}
	f.Session = &session{conn, ch}

	if err := f.declareExchange(); err != nil {
		slog.Error("cannot declare exchange", "error", err)
		return err
	}

	return nil
}

func (f *RMQForwarder) disposeSession() error {
	if f.Session != nil {
		_ = f.Session.Close()
		f.Session = nil
	}
	return nil
}

// publish publishes messages to a reconnecting session to a configured exchange.
// It receives from the application specific source of messages.
func (s *session) publish(parentCtx context.Context, payload message, cfg *publishConfig) error {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel()

	confirm := make(chan amqp091.Confirmation, 1)
	forwarderChannel := s.Channel

	// publisher confirms for this channel/connection
	if err := forwarderChannel.Confirm(false); err != nil {
		log.Printf("publisher confirms not supported")
		close(confirm) // confirms not supported, simulate by always nacking
	} else {
		forwarderChannel.NotifyPublish(confirm)
	}

	err := forwarderChannel.PublishWithContext(
		ctx,
		cfg.ExchangeName,
		cfg.RoutingKey,
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			Headers:         amqp091.Table{},
			ContentType:     "application/json",
			ContentEncoding: "",
			DeliveryMode:    amqp091.Persistent,
			Priority:        0,
			AppId:           "laile-webhook-forwarder",
			Body:            payload,
		})
	if err != nil {
		close(confirm)
		return err
	}

	confirmed, ok := <-confirm
	if !ok {
		return errors.New("RMQ did not confirm the publish")
	}
	if !confirmed.Ack {
		log.Printf("RMQ nack'd message %d, body: %q", confirmed.DeliveryTag, string(payload))
		return errors.New("message was not acknowledged by RMQ")
	}
	return nil
}
