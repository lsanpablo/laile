package forwarders

import (
	"context"
	"fmt"
	"time"

	"laile/internal/config"
	"laile/internal/log"
	db_models "laile/internal/postgresql"
)

type DeliveryAttempt struct {
	Headers        []byte
	Body           *[]byte
	QueryParams    []byte
	Method         string
	URL            string
	IdempotencyKey string
}

func NewDeliveryAttempt(event db_models.GetDueDeliveryAttemptsRow, forwarder *config.Forwarder) *DeliveryAttempt {
	log.Logger.DebugContext(context.Background(), "Creating new delivery attempt",
		"event_id", event.ID,
		"body_length", len(event.Body))

	bodyBytes := []byte(event.Body)
	deliveryAttempt := &DeliveryAttempt{
		Headers:        event.Headers,
		Body:           &bodyBytes,
		QueryParams:    event.QueryParams,
		Method:         event.Method,
		URL:            forwarder.URL,
		IdempotencyKey: fmt.Sprintf("%s-%d-%d", event.ForwarderID, time.Now().Unix(), event.ID),
	}
	return deliveryAttempt
}

type DeliveryResult struct {
	// StatusCode is the HTTP status code
	StatusCode int
	Headers    map[string][]string
	Body       *[]byte

	// TODO: Add more fields later for RMQ
}

type DeliveryAttemptForwarder interface {
	Forward(ctx context.Context, deliveryAttempt *DeliveryAttempt) (*DeliveryResult, error)
}
