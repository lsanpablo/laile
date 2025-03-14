// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: query.sql

package dbmodels

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const acquireTaskLock = `-- name: AcquireTaskLock :one
INSERT INTO task_locks (task_name, worker_name, acquired_at)
VALUES ($1, $2, NOW())
ON CONFLICT (task_name) DO NOTHING
RETURNING id, task_name, worker_name, acquired_at, touched_at
`

type AcquireTaskLockParams struct {
	TaskName   string
	WorkerName string
}

func (q *Queries) AcquireTaskLock(ctx context.Context, arg AcquireTaskLockParams) (TaskLock, error) {
	row := q.db.QueryRow(ctx, acquireTaskLock, arg.TaskName, arg.WorkerName)
	var i TaskLock
	err := row.Scan(
		&i.ID,
		&i.TaskName,
		&i.WorkerName,
		&i.AcquiredAt,
		&i.TouchedAt,
	)
	return i, err
}

const claimDeliveryAttempt = `-- name: ClaimDeliveryAttempt :one
UPDATE delivery_attempts
SET status = 'processing', worker_name = $1, executed_at = NOW()
WHERE id = (
    SELECT id FROM delivery_attempts
    WHERE status = 'scheduled'
      AND scheduled_for <= NOW()
      AND delivery_attempts.hash_value >= $2
      AND delivery_attempts.hash_value < $3
    ORDER BY scheduled_for, hash_value
        FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, target_id, status, scheduled_for, executed_at, response_code, response_body, response_headers, error_message, created_at, hash_value, worker_name
`

type ClaimDeliveryAttemptParams struct {
	WorkerName pgtype.Text
	HashStart  int64
	HashEnd    int64
}

func (q *Queries) ClaimDeliveryAttempt(ctx context.Context, arg ClaimDeliveryAttemptParams) (DeliveryAttempt, error) {
	row := q.db.QueryRow(ctx, claimDeliveryAttempt, arg.WorkerName, arg.HashStart, arg.HashEnd)
	var i DeliveryAttempt
	err := row.Scan(
		&i.ID,
		&i.TargetID,
		&i.Status,
		&i.ScheduledFor,
		&i.ExecutedAt,
		&i.ResponseCode,
		&i.ResponseBody,
		&i.ResponseHeaders,
		&i.ErrorMessage,
		&i.CreatedAt,
		&i.HashValue,
		&i.WorkerName,
	)
	return i, err
}

const claimDeliveryAttemptFromEnd = `-- name: ClaimDeliveryAttemptFromEnd :one
UPDATE delivery_attempts
SET status = 'processing', worker_name = $1, executed_at = NOW()
WHERE id = (
    SELECT id FROM delivery_attempts
    WHERE status = 'scheduled'
      AND scheduled_for <= NOW()
      AND delivery_attempts.hash_value >= $2
      OR delivery_attempts.hash_value < $2
    ORDER BY scheduled_for, hash_value
        FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, target_id, status, scheduled_for, executed_at, response_code, response_body, response_headers, error_message, created_at, hash_value, worker_name
`

type ClaimDeliveryAttemptFromEndParams struct {
	WorkerName pgtype.Text
	HashEnd    int64
}

// If a node is in charge of the end of the hash ring, it needs to go back and claim the tasks
// from the start of the hash ring.
func (q *Queries) ClaimDeliveryAttemptFromEnd(ctx context.Context, arg ClaimDeliveryAttemptFromEndParams) (DeliveryAttempt, error) {
	row := q.db.QueryRow(ctx, claimDeliveryAttemptFromEnd, arg.WorkerName, arg.HashEnd)
	var i DeliveryAttempt
	err := row.Scan(
		&i.ID,
		&i.TargetID,
		&i.Status,
		&i.ScheduledFor,
		&i.ExecutedAt,
		&i.ResponseCode,
		&i.ResponseBody,
		&i.ResponseHeaders,
		&i.ErrorMessage,
		&i.CreatedAt,
		&i.HashValue,
		&i.WorkerName,
	)
	return i, err
}

const countDueDeliveryAttempts = `-- name: CountDueDeliveryAttempts :one
SELECT count(*) FROM delivery_attempts ds
WHERE ds.status = 'scheduled' AND (ds.scheduled_for <= $1 OR ds.scheduled_for IS NULL)
`

func (q *Queries) CountDueDeliveryAttempts(ctx context.Context, scheduledFor pgtype.Timestamptz) (int64, error) {
	row := q.db.QueryRow(ctx, countDueDeliveryAttempts, scheduledFor)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getDeliveryAttemptCount = `-- name: GetDeliveryAttemptCount :one
SELECT count(*) FROM delivery_attempts
WHERE target_id = $1
`

func (q *Queries) GetDeliveryAttemptCount(ctx context.Context, targetID pgtype.Int8) (int64, error) {
	row := q.db.QueryRow(ctx, getDeliveryAttemptCount, targetID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getDeliveryAttemptsByTargetId = `-- name: GetDeliveryAttemptsByTargetId :many
SELECT
    id,
    scheduled_for,
    executed_at,
    created_at,
    response_body,
    status
FROM delivery_attempts
WHERE target_id = $1
ORDER BY created_at DESC
`

type GetDeliveryAttemptsByTargetIdRow struct {
	ID           int64
	ScheduledFor pgtype.Timestamptz
	ExecutedAt   pgtype.Timestamptz
	CreatedAt    pgtype.Timestamptz
	ResponseBody pgtype.Text
	Status       DeliveryStatus
}

func (q *Queries) GetDeliveryAttemptsByTargetId(ctx context.Context, targetID pgtype.Int8) ([]GetDeliveryAttemptsByTargetIdRow, error) {
	rows, err := q.db.Query(ctx, getDeliveryAttemptsByTargetId, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetDeliveryAttemptsByTargetIdRow
	for rows.Next() {
		var i GetDeliveryAttemptsByTargetIdRow
		if err := rows.Scan(
			&i.ID,
			&i.ScheduledFor,
			&i.ExecutedAt,
			&i.CreatedAt,
			&i.ResponseBody,
			&i.Status,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getDeliveryAttemptsList = `-- name: GetDeliveryAttemptsList :many
SELECT da.id, da.target_id, da.status, da.scheduled_for, da.executed_at, da.response_code, da.response_body, da.response_headers, da.error_message, da.created_at, da.hash_value, da.worker_name, wt.id, wt.webhook_id, wt.forwarder_id, wt.created_at, wt.hash_value, w.id, w.name, w.url, w.method, w.body, w.headers, w.query_params, w.webhook_service_id, w.delivery_status, w.created_at, w.idempotency_key
FROM delivery_attempts da
         JOIN webhook_targets wt ON da.target_id = wt.id
         JOIN webhooks w ON wt.webhook_id = w.id
WHERE
    ($1::text = '' OR w.webhook_service_id = $1) AND
    ($2::text = '' OR wt.forwarder_id = $2) AND
    ($3::text = '' OR da.status = $3::delivery_status) AND
    ($4::bigint = 0 OR da.id < $4)
ORDER BY da.id DESC
LIMIT $5
`

type GetDeliveryAttemptsListParams struct {
	ServiceID   string
	ForwarderID string
	Status      string
	Cursor      int64
	PageSize    int32
}

type GetDeliveryAttemptsListRow struct {
	ID               int64
	TargetID         pgtype.Int8
	Status           DeliveryStatus
	ScheduledFor     pgtype.Timestamptz
	ExecutedAt       pgtype.Timestamptz
	ResponseCode     pgtype.Int4
	ResponseBody     pgtype.Text
	ResponseHeaders  []byte
	ErrorMessage     pgtype.Text
	CreatedAt        pgtype.Timestamptz
	HashValue        int64
	WorkerName       pgtype.Text
	ID_2             int64
	WebhookID        pgtype.Int8
	ForwarderID      string
	CreatedAt_2      pgtype.Timestamptz
	HashValue_2      int64
	ID_3             int64
	Name             string
	Url              string
	Method           string
	Body             string
	Headers          []byte
	QueryParams      []byte
	WebhookServiceID string
	DeliveryStatus   DeliveryStatus
	CreatedAt_3      pgtype.Timestamptz
	IdempotencyKey   pgtype.Text
}

func (q *Queries) GetDeliveryAttemptsList(ctx context.Context, arg GetDeliveryAttemptsListParams) ([]GetDeliveryAttemptsListRow, error) {
	rows, err := q.db.Query(ctx, getDeliveryAttemptsList,
		arg.ServiceID,
		arg.ForwarderID,
		arg.Status,
		arg.Cursor,
		arg.PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetDeliveryAttemptsListRow
	for rows.Next() {
		var i GetDeliveryAttemptsListRow
		if err := rows.Scan(
			&i.ID,
			&i.TargetID,
			&i.Status,
			&i.ScheduledFor,
			&i.ExecutedAt,
			&i.ResponseCode,
			&i.ResponseBody,
			&i.ResponseHeaders,
			&i.ErrorMessage,
			&i.CreatedAt,
			&i.HashValue,
			&i.WorkerName,
			&i.ID_2,
			&i.WebhookID,
			&i.ForwarderID,
			&i.CreatedAt_2,
			&i.HashValue_2,
			&i.ID_3,
			&i.Name,
			&i.Url,
			&i.Method,
			&i.Body,
			&i.Headers,
			&i.QueryParams,
			&i.WebhookServiceID,
			&i.DeliveryStatus,
			&i.CreatedAt_3,
			&i.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getDueDeliveryAttempts = `-- name: GetDueDeliveryAttempts :many
SELECT da.id, da.target_id, da.status, da.scheduled_for, da.executed_at, da.response_code, da.response_body, da.response_headers, da.error_message, da.created_at, da.hash_value, da.worker_name, wt.id, wt.webhook_id, wt.forwarder_id, wt.created_at, wt.hash_value, w.id, w.name, w.url, w.method, w.body, w.headers, w.query_params, w.webhook_service_id, w.delivery_status, w.created_at, w.idempotency_key FROM delivery_attempts da
    JOIN public.webhook_targets wt on da.target_id = wt.id
    JOIN public.webhooks w on wt.webhook_id = w.id
WHERE da.status = 'scheduled' AND (da.scheduled_for <= $1 OR da.scheduled_for IS NULL)
`

type GetDueDeliveryAttemptsRow struct {
	ID               int64
	TargetID         pgtype.Int8
	Status           DeliveryStatus
	ScheduledFor     pgtype.Timestamptz
	ExecutedAt       pgtype.Timestamptz
	ResponseCode     pgtype.Int4
	ResponseBody     pgtype.Text
	ResponseHeaders  []byte
	ErrorMessage     pgtype.Text
	CreatedAt        pgtype.Timestamptz
	HashValue        int64
	WorkerName       pgtype.Text
	ID_2             int64
	WebhookID        pgtype.Int8
	ForwarderID      string
	CreatedAt_2      pgtype.Timestamptz
	HashValue_2      int64
	ID_3             int64
	Name             string
	Url              string
	Method           string
	Body             string
	Headers          []byte
	QueryParams      []byte
	WebhookServiceID string
	DeliveryStatus   DeliveryStatus
	CreatedAt_3      pgtype.Timestamptz
	IdempotencyKey   pgtype.Text
}

func (q *Queries) GetDueDeliveryAttempts(ctx context.Context, scheduledFor pgtype.Timestamptz) ([]GetDueDeliveryAttemptsRow, error) {
	rows, err := q.db.Query(ctx, getDueDeliveryAttempts, scheduledFor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetDueDeliveryAttemptsRow
	for rows.Next() {
		var i GetDueDeliveryAttemptsRow
		if err := rows.Scan(
			&i.ID,
			&i.TargetID,
			&i.Status,
			&i.ScheduledFor,
			&i.ExecutedAt,
			&i.ResponseCode,
			&i.ResponseBody,
			&i.ResponseHeaders,
			&i.ErrorMessage,
			&i.CreatedAt,
			&i.HashValue,
			&i.WorkerName,
			&i.ID_2,
			&i.WebhookID,
			&i.ForwarderID,
			&i.CreatedAt_2,
			&i.HashValue_2,
			&i.ID_3,
			&i.Name,
			&i.Url,
			&i.Method,
			&i.Body,
			&i.Headers,
			&i.QueryParams,
			&i.WebhookServiceID,
			&i.DeliveryStatus,
			&i.CreatedAt_3,
			&i.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getMostRecentDeliveryAttemptByWebhookId = `-- name: GetMostRecentDeliveryAttemptByWebhookId :one
SELECT da.id, target_id, status, scheduled_for, executed_at, response_code, response_body, response_headers, error_message, da.created_at, da.hash_value, worker_name, wt.id, webhook_id, forwarder_id, wt.created_at, wt.hash_value FROM delivery_attempts da
         JOIN webhook_targets wt ON da.target_id = wt.id
WHERE wt.webhook_id = $1
ORDER BY da.created_at DESC
LIMIT 1
`

type GetMostRecentDeliveryAttemptByWebhookIdRow struct {
	ID              int64
	TargetID        pgtype.Int8
	Status          DeliveryStatus
	ScheduledFor    pgtype.Timestamptz
	ExecutedAt      pgtype.Timestamptz
	ResponseCode    pgtype.Int4
	ResponseBody    pgtype.Text
	ResponseHeaders []byte
	ErrorMessage    pgtype.Text
	CreatedAt       pgtype.Timestamptz
	HashValue       int64
	WorkerName      pgtype.Text
	ID_2            int64
	WebhookID       pgtype.Int8
	ForwarderID     string
	CreatedAt_2     pgtype.Timestamptz
	HashValue_2     int64
}

func (q *Queries) GetMostRecentDeliveryAttemptByWebhookId(ctx context.Context, webhookID pgtype.Int8) (GetMostRecentDeliveryAttemptByWebhookIdRow, error) {
	row := q.db.QueryRow(ctx, getMostRecentDeliveryAttemptByWebhookId, webhookID)
	var i GetMostRecentDeliveryAttemptByWebhookIdRow
	err := row.Scan(
		&i.ID,
		&i.TargetID,
		&i.Status,
		&i.ScheduledFor,
		&i.ExecutedAt,
		&i.ResponseCode,
		&i.ResponseBody,
		&i.ResponseHeaders,
		&i.ErrorMessage,
		&i.CreatedAt,
		&i.HashValue,
		&i.WorkerName,
		&i.ID_2,
		&i.WebhookID,
		&i.ForwarderID,
		&i.CreatedAt_2,
		&i.HashValue_2,
	)
	return i, err
}

const getSortedHashRing = `-- name: GetSortedHashRing :many
SELECT id, node_name, virtual_id, hash_key FROM hash_ring ORDER BY hash_key
`

func (q *Queries) GetSortedHashRing(ctx context.Context) ([]HashRing, error) {
	rows, err := q.db.Query(ctx, getSortedHashRing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []HashRing
	for rows.Next() {
		var i HashRing
		if err := rows.Scan(
			&i.ID,
			&i.NodeName,
			&i.VirtualID,
			&i.HashKey,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getUnprocessedWebhooks = `-- name: GetUnprocessedWebhooks :many
SELECT id, name, url, method, body, headers, query_params, webhook_service_id, delivery_status, created_at, idempotency_key FROM webhooks
WHERE delivery_status = 'future'
`

func (q *Queries) GetUnprocessedWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := q.db.Query(ctx, getUnprocessedWebhooks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Webhook
	for rows.Next() {
		var i Webhook
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Url,
			&i.Method,
			&i.Body,
			&i.Headers,
			&i.QueryParams,
			&i.WebhookServiceID,
			&i.DeliveryStatus,
			&i.CreatedAt,
			&i.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getWebhookTargetDetails = `-- name: GetWebhookTargetDetails :one
SELECT
    wt.id, wt.webhook_id, wt.forwarder_id, wt.created_at, wt.hash_value,
    w.webhook_service_id,
    w.url,
    count(da.id) as attempt_count
FROM webhook_targets wt
         JOIN webhooks w ON wt.webhook_id = w.id
         LEFT JOIN delivery_attempts da ON da.target_id = wt.id
WHERE wt.id = $1
GROUP BY wt.id, w.webhook_service_id, w.url
`

type GetWebhookTargetDetailsRow struct {
	ID               int64
	WebhookID        pgtype.Int8
	ForwarderID      string
	CreatedAt        pgtype.Timestamptz
	HashValue        int64
	WebhookServiceID string
	Url              string
	AttemptCount     int64
}

func (q *Queries) GetWebhookTargetDetails(ctx context.Context, id int64) (GetWebhookTargetDetailsRow, error) {
	row := q.db.QueryRow(ctx, getWebhookTargetDetails, id)
	var i GetWebhookTargetDetailsRow
	err := row.Scan(
		&i.ID,
		&i.WebhookID,
		&i.ForwarderID,
		&i.CreatedAt,
		&i.HashValue,
		&i.WebhookServiceID,
		&i.Url,
		&i.AttemptCount,
	)
	return i, err
}

const getWebhookTargetsList = `-- name: GetWebhookTargetsList :many
WITH latest_attempts AS (
    SELECT DISTINCT ON (target_id)
        target_id,
        status,
        response_code
    FROM delivery_attempts
    ORDER BY target_id, created_at DESC
),
     attempt_counts AS (
         SELECT target_id, COUNT(*) as attempt_count
         FROM delivery_attempts
         GROUP BY target_id
     )
SELECT
    wt.id,
    wt.forwarder_id,
    wt.created_at,
    w.webhook_service_id,
    COALESCE(la.status, 'future'::delivery_status) as status,
    la.response_code,
    COALESCE(ac.attempt_count, 0) as attempt_count
FROM webhook_targets wt
         JOIN webhooks w ON wt.webhook_id = w.id
         LEFT JOIN latest_attempts la ON la.target_id = wt.id
         LEFT JOIN attempt_counts ac ON ac.target_id = wt.id
WHERE
    ($1::text = '' OR w.webhook_service_id = $1) AND
    ($2::text = '' OR wt.forwarder_id = $2) AND
    ($3::text = '' OR la.status = $3::delivery_status) AND
    ($4::bigint = 0 OR wt.id < $4)
ORDER BY wt.id DESC
LIMIT $5
`

type GetWebhookTargetsListParams struct {
	ServiceID   string
	ForwarderID string
	Status      string
	Cursor      int64
	PageSize    int32
}

type GetWebhookTargetsListRow struct {
	ID               int64
	ForwarderID      string
	CreatedAt        pgtype.Timestamptz
	WebhookServiceID string
	Status           DeliveryStatus
	ResponseCode     pgtype.Int4
	AttemptCount     int64
}

func (q *Queries) GetWebhookTargetsList(ctx context.Context, arg GetWebhookTargetsListParams) ([]GetWebhookTargetsListRow, error) {
	rows, err := q.db.Query(ctx, getWebhookTargetsList,
		arg.ServiceID,
		arg.ForwarderID,
		arg.Status,
		arg.Cursor,
		arg.PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetWebhookTargetsListRow
	for rows.Next() {
		var i GetWebhookTargetsListRow
		if err := rows.Scan(
			&i.ID,
			&i.ForwarderID,
			&i.CreatedAt,
			&i.WebhookServiceID,
			&i.Status,
			&i.ResponseCode,
			&i.AttemptCount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getWebhooksByServiceId = `-- name: GetWebhooksByServiceId :many
SELECT id, name, url, method, body, headers, query_params, webhook_service_id, delivery_status, created_at, idempotency_key FROM webhooks
WHERE webhook_service_id = $1 ORDER BY created_at DESC
`

func (q *Queries) GetWebhooksByServiceId(ctx context.Context, webhookServiceID string) ([]Webhook, error) {
	rows, err := q.db.Query(ctx, getWebhooksByServiceId, webhookServiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Webhook
	for rows.Next() {
		var i Webhook
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Url,
			&i.Method,
			&i.Body,
			&i.Headers,
			&i.QueryParams,
			&i.WebhookServiceID,
			&i.DeliveryStatus,
			&i.CreatedAt,
			&i.IdempotencyKey,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertWebhookEvent = `-- name: InsertWebhookEvent :one
INSERT INTO webhooks (name, url, webhook_service_id, method, body, headers, query_params)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, url, method, body, headers, query_params, webhook_service_id, delivery_status, created_at, idempotency_key
`

type InsertWebhookEventParams struct {
	Name             string
	Url              string
	WebhookServiceID string
	Method           string
	Body             string
	Headers          []byte
	QueryParams      []byte
}

func (q *Queries) InsertWebhookEvent(ctx context.Context, arg InsertWebhookEventParams) (Webhook, error) {
	row := q.db.QueryRow(ctx, insertWebhookEvent,
		arg.Name,
		arg.Url,
		arg.WebhookServiceID,
		arg.Method,
		arg.Body,
		arg.Headers,
		arg.QueryParams,
	)
	var i Webhook
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Url,
		&i.Method,
		&i.Body,
		&i.Headers,
		&i.QueryParams,
		&i.WebhookServiceID,
		&i.DeliveryStatus,
		&i.CreatedAt,
		&i.IdempotencyKey,
	)
	return i, err
}

const insertWebhookTarget = `-- name: InsertWebhookTarget :one
INSERT INTO webhook_targets (webhook_id, forwarder_id, hash_value)
VALUES ($1, $2, $3)
RETURNING id, webhook_id, forwarder_id, created_at, hash_value
`

type InsertWebhookTargetParams struct {
	WebhookID   pgtype.Int8
	ForwarderID string
	HashValue   int64
}

func (q *Queries) InsertWebhookTarget(ctx context.Context, arg InsertWebhookTargetParams) (WebhookTarget, error) {
	row := q.db.QueryRow(ctx, insertWebhookTarget, arg.WebhookID, arg.ForwarderID, arg.HashValue)
	var i WebhookTarget
	err := row.Scan(
		&i.ID,
		&i.WebhookID,
		&i.ForwarderID,
		&i.CreatedAt,
		&i.HashValue,
	)
	return i, err
}

const markDeliveryAttemptAsFailed = `-- name: MarkDeliveryAttemptAsFailed :exec
UPDATE delivery_attempts SET status = 'failed', executed_at=now(), error_message = $2
WHERE id = $1
`

type MarkDeliveryAttemptAsFailedParams struct {
	ID           int64
	ErrorMessage pgtype.Text
}

func (q *Queries) MarkDeliveryAttemptAsFailed(ctx context.Context, arg MarkDeliveryAttemptAsFailedParams) error {
	_, err := q.db.Exec(ctx, markDeliveryAttemptAsFailed, arg.ID, arg.ErrorMessage)
	return err
}

const markDeliveryAttemptAsSuccess = `-- name: MarkDeliveryAttemptAsSuccess :exec
UPDATE delivery_attempts SET
status = 'success', executed_at=now(),
response_code = $2, response_body = $3, response_headers = $4
WHERE id = $1
`

type MarkDeliveryAttemptAsSuccessParams struct {
	ID              int64
	ResponseCode    pgtype.Int4
	ResponseBody    pgtype.Text
	ResponseHeaders []byte
}

func (q *Queries) MarkDeliveryAttemptAsSuccess(ctx context.Context, arg MarkDeliveryAttemptAsSuccessParams) error {
	_, err := q.db.Exec(ctx, markDeliveryAttemptAsSuccess,
		arg.ID,
		arg.ResponseCode,
		arg.ResponseBody,
		arg.ResponseHeaders,
	)
	return err
}

const markWebhookAsScheduled = `-- name: MarkWebhookAsScheduled :exec
UPDATE webhooks SET delivery_status = 'scheduled'
WHERE id = $1
`

func (q *Queries) MarkWebhookAsScheduled(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, markWebhookAsScheduled, id)
	return err
}

const reclaimAbandonedDeliveryAttempts = `-- name: ReclaimAbandonedDeliveryAttempts :exec
UPDATE delivery_attempts
SET status = 'scheduled', worker_name = NULL, executed_at = NULL
WHERE status = 'processing' AND executed_at < NOW() - INTERVAL '10 minutes'
`

func (q *Queries) ReclaimAbandonedDeliveryAttempts(ctx context.Context) error {
	_, err := q.db.Exec(ctx, reclaimAbandonedDeliveryAttempts)
	return err
}

const registerNodeInHashRing = `-- name: RegisterNodeInHashRing :one
INSERT INTO hash_ring (node_name, virtual_id, hash_key)
VALUES ($1, $2, $3)
ON CONFLICT (node_name) DO NOTHING
RETURNING id, node_name, virtual_id, hash_key
`

type RegisterNodeInHashRingParams struct {
	NodeName  string
	VirtualID int32
	HashKey   int64
}

func (q *Queries) RegisterNodeInHashRing(ctx context.Context, arg RegisterNodeInHashRingParams) (HashRing, error) {
	row := q.db.QueryRow(ctx, registerNodeInHashRing, arg.NodeName, arg.VirtualID, arg.HashKey)
	var i HashRing
	err := row.Scan(
		&i.ID,
		&i.NodeName,
		&i.VirtualID,
		&i.HashKey,
	)
	return i, err
}

const releaseTaskLock = `-- name: ReleaseTaskLock :exec
DELETE FROM task_locks
WHERE task_name = $1
`

func (q *Queries) ReleaseTaskLock(ctx context.Context, taskName string) error {
	_, err := q.db.Exec(ctx, releaseTaskLock, taskName)
	return err
}

const scheduleDeliveryAttempt = `-- name: ScheduleDeliveryAttempt :one
INSERT INTO delivery_attempts (target_id, scheduled_for, status)
VALUES ($1, $2, $3)
RETURNING id, target_id, status, scheduled_for, executed_at, response_code, response_body, response_headers, error_message, created_at, hash_value, worker_name
`

type ScheduleDeliveryAttemptParams struct {
	TargetID     pgtype.Int8
	ScheduledFor pgtype.Timestamptz
	Status       DeliveryStatus
}

func (q *Queries) ScheduleDeliveryAttempt(ctx context.Context, arg ScheduleDeliveryAttemptParams) (DeliveryAttempt, error) {
	row := q.db.QueryRow(ctx, scheduleDeliveryAttempt, arg.TargetID, arg.ScheduledFor, arg.Status)
	var i DeliveryAttempt
	err := row.Scan(
		&i.ID,
		&i.TargetID,
		&i.Status,
		&i.ScheduledFor,
		&i.ExecutedAt,
		&i.ResponseCode,
		&i.ResponseBody,
		&i.ResponseHeaders,
		&i.ErrorMessage,
		&i.CreatedAt,
		&i.HashValue,
		&i.WorkerName,
	)
	return i, err
}

const setWebhookIdempotencyKey = `-- name: SetWebhookIdempotencyKey :exec
UPDATE webhooks SET idempotency_key = $2
WHERE id = $1
`

type SetWebhookIdempotencyKeyParams struct {
	ID             int64
	IdempotencyKey pgtype.Text
}

func (q *Queries) SetWebhookIdempotencyKey(ctx context.Context, arg SetWebhookIdempotencyKeyParams) error {
	_, err := q.db.Exec(ctx, setWebhookIdempotencyKey, arg.ID, arg.IdempotencyKey)
	return err
}

const touchLock = `-- name: TouchLock :exec
UPDATE task_locks SET touched_at = NOW() WHERE task_name = $1
`

func (q *Queries) TouchLock(ctx context.Context, taskName string) error {
	_, err := q.db.Exec(ctx, touchLock, taskName)
	return err
}

const updateWebhookDeliveryStatus = `-- name: UpdateWebhookDeliveryStatus :exec
UPDATE webhooks SET delivery_status = $2
WHERE id = $1
`

type UpdateWebhookDeliveryStatusParams struct {
	ID             int64
	DeliveryStatus DeliveryStatus
}

func (q *Queries) UpdateWebhookDeliveryStatus(ctx context.Context, arg UpdateWebhookDeliveryStatusParams) error {
	_, err := q.db.Exec(ctx, updateWebhookDeliveryStatus, arg.ID, arg.DeliveryStatus)
	return err
}
