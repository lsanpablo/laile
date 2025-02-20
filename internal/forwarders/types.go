package forwarders

import (
	"context"
	"fmt"
	"laile/internal/config"
	"laile/internal/logger"
	db_models "laile/internal/postgresql"
	"time"
)

type DeliveryAttempt struct {
	Headers        []byte
	Body           *[]byte
	QueryParams    []byte
	Method         string
	Url            string
	IdempotencyKey string
}

func NewDeliveryAttempt(event db_models.GetDueDeliveryAttemptsRow, forwarder *config.Forwarder) *DeliveryAttempt {
	logger.Debug(context.Background(), "Creating new delivery attempt",
		"event_id", event.ID,
		"body_length", len(event.Body))

	bodyBytes := []byte(event.Body)
	deliveryAttempt := &DeliveryAttempt{
		Headers:     event.Headers,
		Body:        &bodyBytes,
		QueryParams: event.QueryParams,
		Method:      event.Method,
		Url:         forwarder.URL,
	}
	deliveryAttempt.generateIdempotencyKey(event.ForwarderID, event.ID)
	return deliveryAttempt
}

func (a *DeliveryAttempt) generateIdempotencyKey(forwarderID string, eventID int64) {
	a.IdempotencyKey = fmt.Sprintf("%s-%d-%d", forwarderID, time.Now().Unix(), eventID)
}

var SampleBody = []byte(`{"message": "Hello, world!"}`)
var SampleDeliveryAttempt = DeliveryAttempt{
	Headers:        []byte(`{"Content-Type": ["application/json"]}`),
	Body:           &SampleBody,
	QueryParams:    []byte(`{"key": ["value"]}`),
	Method:         "POST",
	Url:            "http://example.com",
	IdempotencyKey: "some-key",
}

type DeliveryResult struct {
	// HTTP status code
	StatusCode int
	Headers    map[string][]string
	Body       *[]byte

	// TODO: Add more fields later for RMQ
}

type DeliveryAttemptForwarder interface {
	Forward(ctx context.Context, deliveryAttempt *DeliveryAttempt) (*DeliveryResult, error)
}
