package event

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"laile/internal"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/forwarders"
	"laile/internal/logger"
	dbmodels "laile/internal/postgresql"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lithammer/shortuuid/v4"
)

func NewWebhookServiceFromConfigWebhookService(name string, configWS *config.WebhookService) *WebhookService {
	return &WebhookService{
		ID:     name,
		Name:   name,
		Path:   configWS.Path,
		Config: configWS,
	}
}

// GetWebhookServiceByPath returns the webhook service for the given path
// It first checks if the path matches a service name directly
// Then it checks if the path matches an explicit path override
// If neither are found, it returns false
func GetWebhookServiceByPath(currentConfig *config.Config, path string) (*WebhookService, bool) {
	// First check if path matches a service name directly
	if service, exists := currentConfig.WebhookServices[path]; exists {
		return NewWebhookServiceFromConfigWebhookService(path, &service), true
	}

	// Then check for explicit path overrides
	for name, service := range currentConfig.WebhookServices {
		if service.Path != "" && service.Path == path {
			return NewWebhookServiceFromConfigWebhookService(name, &service), true
		}
	}
	// TODO, make sure to validate that a path can't match multiple services
	return nil, false
}

func GetWebhookServiceByID(config *config.Config, id string) (*WebhookService, bool) {
	if service, exists := config.WebhookServices[id]; exists {
		return NewWebhookServiceFromConfigWebhookService(id, &service), true
	}
	return nil, false
}

func (ws *WebhookService) GetForwarderByID(id string) (*config.Forwarder, bool) {
	forwarderConfig, exists := ws.Config.Forwarders[id]
	if !exists {
		return nil, false
	}

	return &forwarderConfig, true

}

func HandleEvent(dbService database.Service, listener string, request *http.Request, serverConfig *config.Config) error {
	ctx := context.Background()
	tx, err := dbService.BeginTx(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to begin transaction", err)
		return err
	}
	queries := tx.Queries()
	defer tx.Rollback(ctx)

	configService, exists := GetWebhookServiceByPath(serverConfig, listener)
	if !exists {
		return errors.New("webhook service not found")
	}

	payloadBytes, err := io.ReadAll(request.Body)
	if err != nil {
		logger.Error(ctx, "Failed to read request body", err)
		return err
	}

	payload := string(payloadBytes)
	headersJson := internal.HeadersToJson(request.Header)
	headersJsonBytes, err := json.Marshal(headersJson)
	if err != nil {
		logger.Error(ctx, "Failed to marshal headers", err)
		return errors.New("failed to parse request headers")
	}

	queryParamsJson := internal.QueryParamsToJson(request.URL.Query())
	queryParamsJsonBytes, err := json.Marshal(queryParamsJson)
	if err != nil {
		logger.Error(ctx, "Failed to marshal query params", err)
		return errors.New("failed to parse request query params")
	}

	webhookRecord, err := queries.InsertWebhookEvent(ctx, dbmodels.InsertWebhookEventParams{
		Name:             shortuuid.New(),
		Url:              request.URL.String(),
		Body:             payload,
		Headers:          headersJsonBytes,
		QueryParams:      queryParamsJsonBytes,
		WebhookServiceID: configService.ID,
	})
	if err != nil {
		logger.Error(ctx, "Failed to insert webhook event", err)
		return err
	}

	logger.Info(ctx, "Webhook event recorded",
		"event_id", webhookRecord.ID,
		"service_id", configService.ID)

	idempotencyKey := NewIdempotencyKey(webhookRecord.ID, listener)
	err = queries.SetWebhookIdempotencyKey(ctx, dbmodels.SetWebhookIdempotencyKeyParams{
		ID: webhookRecord.ID,
		IdempotencyKey: pgtype.Text{
			String: idempotencyKey,
			Valid:  true,
		},
	})
	if err != nil {
		logger.Error(ctx, "Failed to set idempotency key", err,
			"event_id", webhookRecord.ID,
			"key", idempotencyKey)
		return err
	}

	logger.Debug(ctx, "Set idempotency key",
		"event_id", webhookRecord.ID,
		"key", idempotencyKey)

	return tx.Commit(ctx)
}

func EventProcessor(ctx context.Context, eventChan chan string, db database.Service, currentConfig *config.Config) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	logger.Info(ctx, "Event processor started")

	for {
		select {
		case <-ticker.C:
			tickerEnabled := currentConfig.Settings.TickerEnabled
			if !tickerEnabled {
				continue
			}
			logger.Debug(ctx, "Processing scheduled events")
			if err := processEvents(db, currentConfig); err != nil {
				logger.Error(ctx, "Failed to process scheduled events", err)
			}
		case _ = <-eventChan:
			logger.Debug(ctx, "Processing event from channel")
			if err := processEvents(db, currentConfig); err != nil {
				logger.Error(ctx, "Failed to process events from channel", err)
			}
		}
	}
}

func processEvents(db database.Service, currentConfig *config.Config) error {
	queries := db.Queries()
	ctx := context.Background()
	now := time.Now()

	if err := scheduleDeliveryAttemptsForWebhooks(ctx, queries, now, currentConfig); err != nil {
		logger.Error(ctx, "Failed to schedule delivery attempts", err)
		return err
	}

	count, err := queries.CountDueDeliveryAttempts(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		logger.Error(ctx, "Failed to count due delivery attempts", err)
		return err
	}

	logger.Info(ctx, "Found events to deliver", "count", count)

	events, err := queries.GetDueDeliveryAttempts(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		logger.Error(ctx, "Failed to get due delivery attempts", err)
		return err
	}

	for _, event := range events {
		logger.Debug(ctx, "Processing event", "event_id", event.ID)

		webhookServiceConfig, exists := currentConfig.WebhookServices[event.WebhookServiceID]
		if !exists {
			logger.Error(ctx, "Webhook service not found",
				errors.New("webhook service not found"),
				"service_id", event.WebhookServiceID,
				"event_id", event.ID)
			continue
		}
		forwarderConfig, exists := webhookServiceConfig.Forwarders[event.ForwarderID]
		if !exists {
			logger.Error(ctx, "Forwarder not found",
				errors.New("forwarder not found"),
				"service_id", event.WebhookServiceID,
				"forwarder_id", event.ForwarderID,
				"event_id", event.ID)
			continue
		}
		if err := deliverEvent(event, db, &forwarderConfig); err != nil {
			logger.Error(ctx, "Failed to deliver event", err,
				"event_id", event.ID,
				"forwarder_id", event.ForwarderID)

			if err := rescheduleEvent(queries, event, err); err != nil {
				logger.Error(ctx, "Failed to reschedule event", err,
					"event_id", event.ID)
			}
		}
	}
	return nil
}

func scheduleDeliveryAttemptsForWebhooks(ctx context.Context, queries *dbmodels.Queries, now time.Time, config *config.Config) error {
	// TODO this whole operation could be a transaction
	webhooks, err := queries.GetUnprocessedWebhooks(ctx)
	if err != nil {
		return err
	}
	// TODO schedule a delivery attempt for each forwarder type
	for _, webhook := range webhooks {
		webhookService, exists := GetWebhookServiceByID(config, webhook.WebhookServiceID)
		if !exists {
			logger.Error(ctx, "Webhook service not found",
				errors.New("webhook service not found"),
				"service_id", webhook.WebhookServiceID,
				"event_id", webhook.ID)
			continue
		}
		forwarderConfigs := webhookService.Config.Forwarders
		for name, _ := range forwarderConfigs {
			webhookTargetRecord, err := queries.InsertWebhookTarget(ctx, dbmodels.InsertWebhookTargetParams{
				WebhookID: pgtype.Int8{
					Int64: webhook.ID,
					Valid: true,
				},
				ForwarderID: name,
			})
			_, err = queries.ScheduleDeliveryAttempt(ctx, dbmodels.ScheduleDeliveryAttemptParams{
				TargetID: pgtype.Int8{
					Int64: webhookTargetRecord.ID,
					Valid: true,
				},
				ScheduledFor: pgtype.Timestamptz{Time: now, Valid: true},
				Status:       dbmodels.DeliveryStatusScheduled,
			})
			if err != nil {
				logger.Error(ctx, "Failed to reschedule event", err)
				return err
			}
		}

		err = queries.MarkWebhookAsScheduled(ctx, webhook.ID)
		if err != nil {
			logger.Error(ctx, "Failed to mark webhook as scheduled", err)
			return err
		}
	}

	return nil
}

func getNextExponentialBackoffTime(deliveryAttemptCount int64) time.Duration {
	// 2^deliveryAttemptCount * 1 seconds
	return time.Duration((1 << deliveryAttemptCount) * 1 * time.Second)
}

func rescheduleEvent(queries *dbmodels.Queries, event dbmodels.GetDueDeliveryAttemptsRow, error_message error) error {
	deliveryAttemptCount, err := queries.GetDeliveryAttemptCount(context.Background(), pgtype.Int8{Int64: event.ID})
	if err != nil {
		return errors.New("failed to get delivery attempt count")
	}
	nextAttemptTime := time.Now().Add(getNextExponentialBackoffTime(deliveryAttemptCount))
	queryParams := dbmodels.MarkDeliveryAttemptAsFailedParams{
		ID:           event.ID,
		ErrorMessage: pgtype.Text{String: error_message.Error(), Valid: true},
	}
	err = queries.MarkDeliveryAttemptAsFailed(context.Background(), queryParams)
	if err != nil {
		return errors.New("failed to mark delivery attempt as failed")

	}
	_, err = queries.ScheduleDeliveryAttempt(context.Background(), dbmodels.ScheduleDeliveryAttemptParams{
		TargetID:     event.TargetID,
		ScheduledFor: pgtype.Timestamptz{Time: nextAttemptTime, Valid: true},
		Status:       dbmodels.DeliveryStatusScheduled,
	})
	return nil
}

func deliverEvent(event dbmodels.GetDueDeliveryAttemptsRow, db database.Service, forwarderConfig *config.Forwarder) error {
	// Currently there's only one delivery method, HTTP, so we'll just use that
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx)
	defer tx.Rollback(ctx)
	if err != nil {
		return err
	}
	queries := db.Queries()
	queries = queries.WithTx(tx.RawTx())

	eventForwarder, err := forwarders.NewForwarder(forwarderConfig)
	if err != nil {
		return err
	}
	logger.Info(ctx, "Body: %s", event.Body)
	bodyBytes := []byte(event.Body)
	logger.Info(ctx, "Body bytes: %s", bodyBytes)
	deliveryAttempt := &forwarders.DeliveryAttempt{
		Headers:     event.Headers,
		Body:        &bodyBytes,
		QueryParams: event.QueryParams,
		Method:      event.Method,
		Url:         forwarderConfig.URL,
	}
	deliveryResult, err := eventForwarder.Forward(ctx, deliveryAttempt)
	if err != nil {
		return err
	}

	headersBytes, err := json.Marshal(deliveryResult.Headers)
	if err != nil {
		return err
	}

	var bodyResultSQL pgtype.Text
	if deliveryResult.Body == nil {
		bodyResultSQL = pgtype.Text{String: "", Valid: false}
	} else {
		bodyResultSQL = pgtype.Text{String: string(*deliveryResult.Body), Valid: true}
	}

	err = queries.MarkDeliveryAttemptAsSuccess(context.Background(), dbmodels.MarkDeliveryAttemptAsSuccessParams{
		ID:              event.ID,
		ResponseCode:    pgtype.Int4{Int32: int32(deliveryResult.StatusCode), Valid: true},
		ResponseBody:    bodyResultSQL,
		ResponseHeaders: headersBytes,
	})
	if err != nil {
		return err
	}
	err = queries.UpdateWebhookDeliveryStatus(context.Background(), dbmodels.UpdateWebhookDeliveryStatusParams{
		ID:             event.WebhookID.Int64,
		DeliveryStatus: dbmodels.DeliveryStatusSuccess,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
