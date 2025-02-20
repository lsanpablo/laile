# Webhook Ingress Configuration Documentation

This document describes the configuration structure and options for the Webhook Ingress service. The configuration is specified in TOML format.

## Configuration Structure

The configuration consists of two main sections:
- Global Settings
- Webhook Services

```toml
[settings]
listener_port = 8080 # Main port for receiving webhooks (required, range: 1-65535)
admin_port = 8081 # Port for admin interface (required, range: 1-65535)
ticker_enabled = true # Enable/disable the retry mechanism
ticker_interval = 5 # Retry interval in seconds (required if ticker_enabled = true)
```

## Webhook Services

Webhook services define how incoming webhooks are processed and forwarded. Each service is configured as a named section in the TOML file.

### Service Configuration
```toml
[webhook_services.service_name]
path = "incoming" # Optional URL path segment (alphanumeric only)
authentication_type = "header" # Authentication method (currently only "header" supported)
authentication_header = "X-Auth" # Header name for authentication
authentication_secret = "secret123" # Secret value for authentication
```

### Forwarders

Each webhook service can have multiple forwarders that define where the webhook payload should be sent. Forwarders can be either HTTP endpoints or AMQP (RabbitMQ) queues.

#### HTTP Forwarder

```toml
[webhook_services.service_name.forwarders.http_forward]
type = "http" # Forwarder type (required)
url = "https://api.example.com" # Target URL (required)
headers = { "Authorization" = "xyz" } # Optional additional headers. If a header is already a part of the webhook, it will be overwritten with values from this list.
retry_count = 3 # Number of retry attempts (default: 3)
retry_delay = "exponential" # Retry delay type: "exponential" or "fixed" (default: "exponential")
```

#### AMQP Forwarder

```toml
[webhook_services.service_name.forwarders.queue_forward]
type = "amqp" # Forwarder type (required)
connection_url = "amqp://guest:guest@localhost" # AMQP connection URL (required)
exchange = "webhook_exchange" # Exchange name (required)
routing_key = "webhooks.incoming" # Routing key (required)
queue = "webhook_queue" # Queue name (required)
exchange_type = "direct" # Exchange type: "direct", "fanout", "topic", or "headers" (default: "direct")
durable = true # Queue/Exchange durability (default: true)
persistent = true # Message persistence (default: true)
auto_delete = false # Auto-delete queue when unused
exclusive = false # Exclusive queue access
no_wait = false # Don't wait for server confirmation
internal = false # Internal exchange (not accessible outside)
mandatory = false # Require successful routing
immediate = false # Require immediate consumer
```


## Complete Example

Here's a complete example configuration that sets up two webhook services - one for Stripe and one for GitHub:
```toml
[settings]
listener_port = 8080
admin_port = 8081
ticker_enabled = true
ticker_interval = 5
[webhook_services.stripe]
path = "stripe"
authentication_type = "header"
authentication_header = "Stripe-Signature"
authentication_secret = "whsec_..."
[webhook_services.stripe.forwarders.payment_processor]
type = "http"
url = "https://internal-api.example.com/payments"
headers = { "Internal-Auth" = "secret123" }
retry_count = 3
retry_delay = "exponential"
[webhook_services.stripe.forwarders.payment_events]
type = "amqp"
connection_url = "amqp://user:pass@rabbitmq:5672"
exchange = "payment_events"
routing_key = "stripe.webhook"
queue = "stripe_webhooks"
exchange_type = "topic"
durable = true
persistent = true
[webhook_services.github]
path = "github"
authentication_type = "header"
authentication_header = "X-Hub-Signature-256"
authentication_secret = "github_webhook_secret"
[webhook_services.github.forwarders.ci_pipeline]
type = "http"
url = "https://jenkins.internal/github-webhook/"
retry_count = 5
retry_delay = "fixed"
```


## Configuration Notes

1. **Authentication**: Currently, only header-based authentication is supported. Each webhook service can have its own authentication method.

2. **Retry Mechanism**: 
   - The global ticker controls retry attempts for failed forwards
   - Each forwarder can configure its own retry count and delay strategy
   - "exponential" delay increases wait time between retries
   - "fixed" delay uses consistent intervals

3. **AMQP Reliability**:
   - Default settings prioritize reliability (durable queues, persistent messages)
   - Exchange types affect message routing:
     - `direct`: Route based on exact routing key match
     - `fanout`: Broadcast to all bound queues
     - `topic`: Route based on routing key patterns
     - `headers`: Route based on message headers

4. **Port Configuration**:
   - Ensure `listener_port` and `admin_port` are different
   - Both ports must be available on your system
   - Valid port range is 1-65535

5. **Path Configuration**:
   - Service paths must be alphanumeric
   - Each service path must be unique
   - Empty paths are allowed (service will use root path)
