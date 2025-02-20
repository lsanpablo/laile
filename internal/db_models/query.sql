-- name: InsertWebhookEvent :one
INSERT INTO webhooks (name, url, webhook_service_id, method, body, headers, query_params)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: SetWebhookIdempotencyKey :exec
UPDATE webhooks SET idempotency_key = $2
WHERE id = $1;

-- name: GetUnprocessedWebhooks :many
SELECT * FROM webhooks
WHERE delivery_status = 'future';

-- name: GetWebhooksByServiceId :many
SELECT * FROM webhooks
WHERE webhook_service_id = $1 ORDER BY created_at DESC;

-- name: UpdateWebhookDeliveryStatus :exec
UPDATE webhooks SET delivery_status = $2
WHERE id = $1;

-- name: GetMostRecentDeliveryAttemptByWebhookId :one
SELECT * FROM delivery_attempts da
         JOIN webhook_targets wt ON da.target_id = wt.id
WHERE wt.webhook_id = $1
ORDER BY da.created_at DESC
LIMIT 1;

-- name: InsertWebhookTarget :one
INSERT INTO webhook_targets (webhook_id, forwarder_id)
VALUES ($1, $2)
RETURNING *;

-- name: MarkWebhookAsScheduled :exec
UPDATE webhooks SET delivery_status = 'scheduled'
WHERE id = $1;

-- name: ScheduleDeliveryAttempt :one
INSERT INTO delivery_attempts (target_id, scheduled_for, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetDueDeliveryAttempts :many
SELECT da.*, wt.*, w.* FROM delivery_attempts da
    JOIN public.webhook_targets wt on da.target_id = wt.id
    JOIN public.webhooks w on wt.webhook_id = w.id
WHERE da.status = 'scheduled' AND (da.scheduled_for <= $1 OR da.scheduled_for IS NULL);

-- name: CountDueDeliveryAttempts :one
SELECT count(*) FROM delivery_attempts ds
WHERE ds.status = 'scheduled' AND (ds.scheduled_for <= $1 OR ds.scheduled_for IS NULL);

-- name: GetDeliveryAttemptCount :one
SELECT count(*) FROM delivery_attempts
WHERE target_id = $1;

-- name: MarkDeliveryAttemptAsFailed :exec
UPDATE delivery_attempts SET status = 'failed', executed_at=now(), error_message = $2
WHERE id = $1;

-- name: MarkDeliveryAttemptAsSuccess :exec
UPDATE delivery_attempts SET
status = 'success', executed_at=now(),
response_code = $2, response_body = $3, response_headers = $4
WHERE id = $1;


-- name: GetDeliveryAttemptsList :many
SELECT da.*, wt.*, w.*
FROM delivery_attempts da
         JOIN webhook_targets wt ON da.target_id = wt.id
         JOIN webhooks w ON wt.webhook_id = w.id
WHERE
    (@service_id::text = '' OR w.webhook_service_id = @service_id) AND
    (@forwarder_id::text = '' OR wt.forwarder_id = @forwarder_id) AND
    (@status::text = '' OR da.status = @status::delivery_status) AND
    (@cursor::bigint = 0 OR da.id < @cursor)
ORDER BY da.id DESC
LIMIT @page_size;

-- name: GetWebhookTargetsList :many
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
    (@service_id::text = '' OR w.webhook_service_id = @service_id) AND
    (@forwarder_id::text = '' OR wt.forwarder_id = @forwarder_id) AND
    (@status::text = '' OR la.status = @status::delivery_status) AND
    (@cursor::bigint = 0 OR wt.id < @cursor)
ORDER BY wt.id DESC
LIMIT @page_size;

-- name: GetDeliveryAttemptsByTargetId :many
SELECT
    id,
    scheduled_for,
    executed_at,
    created_at,
    response_body,
    status
FROM delivery_attempts
WHERE target_id = $1
ORDER BY created_at DESC;

-- name: GetWebhookTargetDetails :one
SELECT
    wt.*,
    w.webhook_service_id,
    w.url,
    count(da.id) as attempt_count
FROM webhook_targets wt
         JOIN webhooks w ON wt.webhook_id = w.id
         LEFT JOIN delivery_attempts da ON da.target_id = wt.id
WHERE wt.id = $1
GROUP BY wt.id, w.webhook_service_id, w.url;