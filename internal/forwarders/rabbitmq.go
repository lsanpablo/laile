package forwarders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"laile/internal/config"
	"laile/internal/log"
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
	err := s.Connection.Close()
	if err != nil {
		log.Logger.Error("failed to close RMQ session", slog.Any("error", err))
		return fmt.Errorf("failed to close RMQ session: %w", err)
	}
	return nil
}

func NewRMQForwarder(config *config.Forwarder) *RMQForwarder {
	return &RMQForwarder{
		Session:    nil,
		Config:     config,
		ErrorCount: 0,
		mu:         sync.Mutex{},
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
	payload, err := webhookToAMQPBody(deliveryAttempt)
	if err != nil {
		return nil, err
	}

	const forwardingTimeout = 5 * time.Second
	timeoutContext, cancel := context.WithTimeout(ctx, forwardingTimeout)
	defer cancel()

	// Single attempt to publish with proper error handling
	err = f.publishToRMQ(timeoutContext, payload)
	if err != nil {
		log.Logger.ErrorContext(ctx, "producer: error publishing message", "error", err)
		return nil, err
	}

	return &DeliveryResult{
		StatusCode: http.StatusOK,
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
		var amqpErr *amqp091.Error
		if errors.As(err, &amqpErr) && amqpErr.Code >= 300 {
			f.disposeSession()
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
	URL            string              `json:"url"`
	IdempotencyKey string              `json:"idempotency_key"`
	HelpText       string              `json:"help_text,omitempty"`
}

// webhookToAMQPBody marshals the DeliveryAttempt into a JSON byte slice.
func webhookToAMQPBody(attempt *DeliveryAttempt) (message, error) {
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
		URL:            attempt.URL,
		IdempotencyKey: attempt.IdempotencyKey,
		HelpText:       "body is a raw JSON message, headers and query parameters are key-value maps",
	}
	resp, err := json.Marshal(amqpBody)
	if err != nil {
		log.Logger.Error("failed to marshal AMQP body", slog.Any("error", err))
		return nil, fmt.Errorf("failed to marshal AMQP body: %w", err)
	}
	return resp, nil
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
	log.Logger.Info("dialing", "url", f.Config.ConnectionURL)
	if err != nil {
		log.Logger.Error("cannot dial RMQ event forwarder", "error", err, "url", f.Config.ConnectionURL)
		return fmt.Errorf("cannot dial RMQ event forwarder: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Logger.Error("cannot create channel", "error", err)
		return fmt.Errorf("cannot create channel: %w", err)
	}
	f.Session = &session{conn, ch}

	if err = f.declareExchange(); err != nil {
		log.Logger.Error("cannot declare exchange", "error", err)
		return fmt.Errorf("cannot declare exchange: %w", err)
	}

	return nil
}

func (f *RMQForwarder) disposeSession() {
	if f.Session != nil {
		err := f.Session.Close()
		if err != nil {
			log.Logger.Error("failed to close RMQ session", "error", err)
		}
		f.Session = nil
	}
}

// publish publishes messages to a reconnecting session to a configured exchange.
// It receives from the application specific source of messages.
func (s *session) publish(parentCtx context.Context, payload message, cfg *publishConfig) error {
	const publishTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(parentCtx, publishTimeout)
	defer cancel()

	confirm := make(chan amqp091.Confirmation, 1)
	forwarderChannel := s.Channel

	// publisher confirms for this channel/connection
	if err := forwarderChannel.Confirm(false); err != nil {
		log.Logger.ErrorContext(ctx, "publisher confirms not supported")
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

			// The following fields are not supported by the forwarder implementation,
			// and will be populated with the zero values.
			CorrelationId: "",
			ReplyTo:       "",
			Expiration:    "",
			MessageId:     "",
			Timestamp:     time.Time{},
			Type:          "",
			UserId:        "",
		})
	if err != nil {
		close(confirm)
		log.Logger.ErrorContext(ctx, "failed to publish message", slog.Any("error", err))
		return fmt.Errorf("failed to publish message: %w", err)
	}

	confirmed, ok := <-confirm
	if !ok {
		return errors.New("RMQ did not confirm the publish")
	}
	if !confirmed.Ack {
		log.Logger.ErrorContext(ctx, "RMQ nack'd message", slog.Uint64("deliveryTag", confirmed.DeliveryTag), slog.String("body", string(payload)))
		return errors.New("message was not acknowledged by RMQ")
	}
	return nil
}
