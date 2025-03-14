package event

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgxlisten"
	"github.com/lithammer/shortuuid/v4"
	"laile/internal"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/forwarders"
	"laile/internal/hashing"
	"laile/internal/log"
	dbmodels "laile/internal/postgresql"
)

func NewWebhookServiceFromConfigWebhookService(name string, configWS *config.WebhookService) *WebhookService {
	return &WebhookService{
		ID:     name,
		Name:   name,
		Path:   configWS.Path,
		Config: configWS,
	}
}

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
		log.Logger.ErrorContext(ctx, "Failed to begin transaction", "error", err)
		return fmt.Errorf("webhook listener failed to begin database transaction: %w", err)
	}
	queries := tx.Queries()
	defer database.Rollback(ctx, tx)

	configService, exists := GetWebhookServiceByPath(serverConfig, listener)
	if !exists {
		return errors.New("webhook service not found")
	}

	payloadBytes, err := io.ReadAll(request.Body)
	if err != nil {
		log.Logger.ErrorContext(ctx, "failed to read request body", slog.Any("error", err))
		return fmt.Errorf("webhook listener failed to read request body: %w", err)
	}

	payload := string(payloadBytes)
	headersJSON := internal.HeadersToJSON(request.Header)
	headersJSONBytes, err := json.Marshal(headersJSON)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to marshal headers", slog.Any("error", err))
		return errors.New("failed to parse request headers")
	}

	queryParamsJSON := internal.QueryParamsToJSON(request.URL.Query())
	queryParamsJSONBytes, err := json.Marshal(queryParamsJSON)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to marshal query params", slog.Any("error", err))
		return errors.New("failed to parse request query params")
	}

	webhookRecord, err := queries.InsertWebhookEvent(ctx, dbmodels.InsertWebhookEventParams{
		Name:        shortuuid.New(),
		Url:         request.URL.String(),
		Method:      request.Method,
		Body:        payload,
		Headers:     headersJSONBytes,
		QueryParams: queryParamsJSONBytes,
	})
	if err != nil {
		log.Logger.ErrorContext(ctx, "failed to insert webhook event into database", slog.Any("error", err))
		return fmt.Errorf("failed to insert webhook event into database: %w", err)
	}

	log.Logger.InfoContext(ctx, "Webhook event recorded",
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
		log.Logger.ErrorContext(ctx, "failed to set idempotency key", slog.Any("error", err),
			slog.Int64("event_id", webhookRecord.ID),
			slog.String("key", idempotencyKey))
		return fmt.Errorf("failed to set idempotency key: %w", err)
	}

	log.Logger.DebugContext(ctx, "Set idempotency key",
		"event_id", webhookRecord.ID,
		"key", idempotencyKey)

	// Create webhook targets for each forwarder immediately
	now := time.Now()
	forwarderConfigs := configService.Config.Forwarders
	for name := range forwarderConfigs {
		// Generate a hash value for this target for distributed processing
		hashValue := hashing.HashKey64Bit(fmt.Sprintf("%d%s", webhookRecord.ID, name))

		var webhookTargetRecord dbmodels.WebhookTarget
		webhookTargetRecord, err = queries.InsertWebhookTarget(ctx, dbmodels.InsertWebhookTargetParams{
			WebhookID: pgtype.Int8{
				Int64: webhookRecord.ID,
				Valid: true,
			},
			ForwarderID: name,
			HashValue:   int64(hashValue), // #nosec G115: only the bits are relevant when used in the hash ring context
		})
		if err != nil {
			log.Logger.ErrorContext(ctx, "Failed to insert webhook target", slog.Any("error", err),
				slog.Int64("event_id", webhookRecord.ID),
				slog.String("forwarder_id", name))
			return fmt.Errorf("failed to insert webhook target into database: %w", err)
		}

		_, err = queries.ScheduleDeliveryAttempt(ctx, dbmodels.ScheduleDeliveryAttemptParams{
			TargetID: pgtype.Int8{
				Int64: webhookTargetRecord.ID,
				Valid: true,
			},
			ScheduledFor: pgtype.Timestamptz{
				Time:             now,
				InfinityModifier: 0,
				Valid:            true,
			},
			Status: dbmodels.DeliveryStatusScheduled,
		})
		if err != nil {
			log.Logger.ErrorContext(ctx, "failed to schedule delivery attempt", slog.Any("error", err),
				slog.Int64("event_id", webhookRecord.ID),
				slog.Int64("target_id", webhookTargetRecord.ID))
			return fmt.Errorf("failed to schedule delivery attempt: %w", err)
		}
	}

	// Mark webhook as scheduled since we've created all the targets
	err = queries.MarkWebhookAsScheduled(ctx, webhookRecord.ID)
	if err != nil {
		log.Logger.ErrorContext(ctx, "failed to mark webhook as scheduled in database", slog.Any("error", err),
			slog.Int64("event_id", webhookRecord.ID))
		return fmt.Errorf("failed to mark webhook as scheduled in database: %w", err)
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		log.Logger.ErrorContext(ctx, "failed to commit transaction in webhook listener", slog.Any("error", err))
		return fmt.Errorf("failed to commit transaction in webhook listener: %w", err)
	}

	// After successful commit, notify the webhook_tasks_channel
	// This is done outside the transaction to ensure we only notify after successful commit
	conn, err := dbService.GetConn(ctx)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to get connection for notification", slog.Any("error", err))
		return nil // Don't fail the request if notification fails
	}
	defer conn.Release()
	rawConn := conn.RawConn()

	_, err = rawConn.Exec(ctx, "SELECT pg_notify('webhook_tasks_channel', $1)", strconv.FormatInt(webhookRecord.ID, 10))
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to send notification", slog.Any("error", err),
			slog.Int64("event_id", webhookRecord.ID))
	} else {
		log.Logger.DebugContext(ctx, "Sent notification for new webhook",
			slog.Int64("event_id", webhookRecord.ID))
	}

	return nil
}

// This is used for distributing work across multiple workers.
func generateHashValue(webhookID int64, forwarderID string) string {
	// Create a hash of the webhook ID and forwarder ID
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d:%s", webhookID, forwarderID)))
	return hex.EncodeToString(h.Sum(nil))
}

func ProcessEvents(ctx context.Context, db database.Service, currentConfig *config.Config) {
	ticker := time.NewTicker(config.DefaultTickerInterval)
	defer ticker.Stop()
	log.Logger.InfoContext(ctx, "Event processor started")
	eventChan := make(chan string)
	err := ListenForEvents(ctx, eventChan, db)
	if err != nil {

	}

	for {
		select {
		case <-ticker.C:
			tickerEnabled := currentConfig.Settings.TickerEnabled
			if !tickerEnabled {
				continue
			}
			log.Logger.DebugContext(ctx, "Processing scheduled events")
			if err = processEvents(db, currentConfig); err != nil {
				log.Logger.ErrorContext(ctx, "Failed to process scheduled events", slog.Any("error", err))
			}
		case <-eventChan:
			log.Logger.DebugContext(ctx, "Processing event from channel")
			if err = processEvents(db, currentConfig); err != nil {
				log.Logger.ErrorContext(ctx, "Failed to process events from channel", slog.Any("error", err))
			}
		}
	}
}

func ListenForEvents(ctx context.Context, eventChan chan string, db database.Service) error {
	const reconnectDelay = 3 * time.Second
	listener := &pgxlisten.Listener{
		Connect: func(ctx context.Context) (*pgx.Conn, error) {
			poolConn, err := db.GetConn(ctx)
			if err != nil {
				log.Logger.ErrorContext(ctx, "Failed to get listening connection", slog.Any("error", err))
				return nil, errors.New("failed to get a listening connection")
			}
			listeningConn := poolConn.RawConn()
			myListeningConn := listeningConn.Hijack()
			return myListeningConn, nil
		},
		LogError: func(ctx context.Context, err error) {
			log.Logger.ErrorContext(ctx, "Failed to listen for events", slog.Any("error", err))
		},
		ReconnectDelay: reconnectDelay,
	}
	listener.Handle("webhook_tasks_channel", pgxlisten.HandlerFunc(func(ctx context.Context, notification *pgconn.Notification, _ *pgx.Conn) error {
		select {
		case eventChan <- notification.Payload:
		case <-ctx.Done():
		}
		return nil
	}))
	go func() {
		err := listener.Listen(ctx)
		if err != nil {
			close(eventChan)
		}
	}()
	return nil
}

func processEvents(db database.Service, currentConfig *config.Config) error {
	queries := db.Queries()
	ctx := context.Background()
	now := time.Now()

	count, err := queries.CountDueDeliveryAttempts(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to count due delivery attempts", slog.Any("error", err))
		return fmt.Errorf("failed get number of due delivery attempts from database: %w", err)
	}

	log.Logger.InfoContext(ctx, "Found events to deliver", "count", count)

	events, err := queries.GetDueDeliveryAttempts(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		log.Logger.ErrorContext(ctx, "failed to get due delivery attempts", slog.Any("error", err))
		return fmt.Errorf("failed to get due delivery attempts from database: %w", err)
	}

	for _, event := range events {
		log.Logger.DebugContext(ctx, "Processing event", "event_id", event.ID)

		webhookServiceConfig, exists := currentConfig.WebhookServices[event.WebhookServiceID]
		if !exists {
			log.Logger.ErrorContext(ctx, "Webhook service not found",
				slog.Any("error", errors.New("webhook service not found")),
				slog.String("service_id", event.WebhookServiceID),
				slog.Int64("event_id", event.ID))
			continue
		}
		forwarderConfig, exists := webhookServiceConfig.Forwarders[event.ForwarderID]
		if !exists {
			log.Logger.ErrorContext(ctx, "Forwarder not found",
				slog.Any("error", errors.New("forwarder not found")),
				slog.String("service_id", event.WebhookServiceID),
				slog.String("forwarder_id", event.ForwarderID),
				slog.Int64("event_id", event.ID))
			continue
		}
		if err = deliverEvent(event, db, &forwarderConfig); err != nil {
			log.Logger.ErrorContext(ctx, "Failed to deliver event", slog.Any("error", err),
				slog.Int64("event_id", event.ID),
				slog.String("forwarder_id", event.ForwarderID))

			if err = rescheduleEvent(queries, event, err); err != nil {
				log.Logger.ErrorContext(ctx, "Failed to reschedule event", slog.Any("error", err),
					slog.Int64("event_id", event.ID))
			}
		}
	}
	return nil
}

func getNextExponentialBackoffTime(deliveryAttemptCount int64) time.Duration {
	// 2^deliveryAttemptCount * 1 seconds
	return (1 << deliveryAttemptCount) * 1 * time.Second
}

func rescheduleEvent(queries *dbmodels.Queries, event dbmodels.GetDueDeliveryAttemptsRow, errorMessage error) error {
	deliveryAttemptCount, err := queries.GetDeliveryAttemptCount(context.Background(), pgtype.Int8{Int64: event.ID})
	if err != nil {
		return errors.New("failed to get delivery attempt count")
	}
	nextAttemptTime := time.Now().Add(getNextExponentialBackoffTime(deliveryAttemptCount))
	queryParams := dbmodels.MarkDeliveryAttemptAsFailedParams{
		ID:           event.ID,
		ErrorMessage: pgtype.Text{String: errorMessage.Error(), Valid: true},
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
	if err != nil {
		return fmt.Errorf("failed to schedule delivery attempt: %w", err)
	}
	return nil
}

func deliverEvent(event dbmodels.GetDueDeliveryAttemptsRow, db database.Service, forwarderConfig *config.Forwarder) error {
	// Currently there's only one delivery method, HTTP, so we'll just use that
	const deliveryDuration = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), deliveryDuration)
	defer cancel()

	tx, err := db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer database.Rollback(ctx, tx)

	queries := db.Queries()
	queries = queries.WithTx(tx.RawTx())

	eventForwarder, err := forwarders.NewForwarder(forwarderConfig)
	if err != nil {
		return fmt.Errorf("failed to create event forwarder: %w", err)
	}
	log.Logger.InfoContext(ctx, "webhook to deliver", slog.String("body", event.Body))
	bodyBytes := []byte(event.Body)
	deliveryAttempt := &forwarders.DeliveryAttempt{
		Headers:     event.Headers,
		Body:        &bodyBytes,
		QueryParams: event.QueryParams,
		Method:      event.Method,
		URL:         forwarderConfig.URL,
	}
	deliveryResult, err := eventForwarder.Forward(ctx, deliveryAttempt)
	if err != nil {
		return fmt.Errorf("failed to forward event: %w", err)
	}

	headersBytes, err := json.Marshal(deliveryResult.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery result headers: %w", err)
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
		return fmt.Errorf("failed to mark delivery attempt as success: %w", err)
	}
	err = queries.UpdateWebhookDeliveryStatus(context.Background(), dbmodels.UpdateWebhookDeliveryStatusParams{
		ID:             event.WebhookID.Int64,
		DeliveryStatus: dbmodels.DeliveryStatusSuccess,
	})
	if err != nil {
		return fmt.Errorf("failed to update webhook delivery status: %w", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit event delivery transaction: %w", err)
	}
	return nil
}
