-- +goose Up
-- +goose StatementBegin
CREATE TYPE delivery_status AS ENUM ('future', 'scheduled', 'processing', 'success', 'failed', 'not_needed');

CREATE TABLE webhooks (
                          id   BIGSERIAL PRIMARY KEY,
                          name text      NOT NULL,
                          url  text      NOT NULL,
                          method varchar(32)   NOT NULL,
                          body text     NOT NULL,
                          headers jsonb NOT NULL,
                          query_params jsonb NOT NULL,
                          webhook_service_id VARCHAR NOT NULL,
                          delivery_status delivery_status NOT NULL DEFAULT 'future',
                          created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                          idempotency_key varchar(255) UNIQUE
);

CREATE TABLE webhook_targets (
                                id   BIGSERIAL PRIMARY KEY,
                                webhook_id BIGINT REFERENCES webhooks(id) ON DELETE CASCADE,
                                forwarder_id VARCHAR NOT NULL,
                                created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                hash_value BIGINT NOT NULL
);

CREATE TABLE delivery_attempts (
                                   id   BIGSERIAL PRIMARY KEY,
                                   target_id    BIGINT REFERENCES webhook_targets(id) ON DELETE CASCADE,
                                   status delivery_status NOT NULL,
                                   scheduled_for timestamp with time zone,
                                   executed_at timestamp with time zone,
                                   response_code integer,
                                   response_body text,
                                   response_headers jsonb,
                                   error_message text,
                                   created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
                                   hash_value BIGINT NOT NULL,
                                   worker_name VARCHAR
);

CREATE TABLE hash_ring
(
    id         SERIAL PRIMARY KEY,
    node_name  VARCHAR NOT NULL,
    virtual_id INTEGER NOT NULL,
    hash_key   BIGINT  NOT NULL UNIQUE
);

CREATE TABLE task_locks
(
    id         SERIAL PRIMARY KEY,
    task_name VARCHAR NOT NULL UNIQUE ,
    worker_name VARCHAR NOT NULL,
    acquired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    touched_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE delivery_attempts;
DROP TABLE webhook_targets;
DROP TABLE webhooks;
DROP TYPE delivery_status;
DROP TABLE hash_ring;
DROP TABLE task_locks;
-- +goose StatementEnd
