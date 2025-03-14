package forwarders

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"laile/internal"
	"laile/internal/config"
	"laile/internal/log"
)

type HTTPForwarder struct {
	Config *config.Forwarder
}

func (f *HTTPForwarder) Forward(ctx context.Context, event *DeliveryAttempt) (*DeliveryResult, error) {
	log.Logger.DebugContext(ctx, "Preparing to forward request",
		"body_length", len(string(*event.Body)),
		"url", f.Config.URL,
		"method", event.Method)

	reader := strings.NewReader(string(*event.Body))
	req, err := http.NewRequestWithContext(ctx, event.Method, f.Config.URL, reader)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to create request", "error", err)
		return nil, fmt.Errorf("failed to create new request for HTTP forwarder: %w", err)
	}

	// Copy headers from the original request to the proxy request
	headers, err := getHeadersFromBytes(event.Headers)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to parse headers", "error", err)
		return nil, fmt.Errorf("failed to parse headers: %w", err)
	}

	// Add Idempotency key to Headers
	headers["laile-idempotency-key"] = []string{event.IdempotencyKey}

	// Add configured forwarder headers, overwriting any existing ones
	for name, value := range f.Config.Headers {
		headers[name] = []string{value}
	}

	for name, headerValues := range headers {
		for _, h := range headerValues {
			req.Header.Add(name, h)
		}
	}

	// Copy query parameters from the original request to the proxy request
	queryParams, err := getQueryParamsFromBytes(event.QueryParams)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Failed to parse query parameters", "error", err)
		return nil, err
	}
	for name, values := range queryParams {
		q := req.URL.Query()
		for _, v := range values {
			q.Add(name, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	// Send the request to the target service
	client := &http.Client{}
	log.Logger.DebugContext(ctx, "Sending request",
		"url", req.URL.String(),
		"method", req.Method,
		"header_count", len(req.Header))

	resp, err := client.Do(req)
	if err != nil {
		log.Logger.ErrorContext(ctx, "Request failed", slog.Any("error", err),
			slog.String("url", req.URL.String()),
			slog.String("method", req.Method))
		return nil, fmt.Errorf("forwarder request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(ctx, "failed to read response body", slog.Any("error", err))
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Logger.InfoContext(ctx, "Request completed successfully",
		"status_code", resp.StatusCode,
		"body_length", len(body),
		"url", req.URL.String())

	// Copy the target service response headers to the client response
	responseHeaders := resp.Header.Clone()
	responseHeadersJSON := internal.HeadersToJSON(responseHeaders)

	return &DeliveryResult{
		StatusCode: resp.StatusCode,
		Headers:    responseHeadersJSON,
		Body:       &body,
	}, nil
}

type Headers map[string][]string
type QueryParameters map[string][]string

func getHeadersFromBytes(b []byte) (Headers, error) {
	var headers Headers
	err := json.Unmarshal(b, &headers)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
	}
	return headers, nil
}
func getQueryParamsFromBytes(b []byte) (QueryParameters, error) {
	var queryParams QueryParameters
	err := json.Unmarshal(b, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal query params: %w", err)
	}
	return queryParams, nil
}

func NewHTTPForwarder(config *config.Forwarder) *HTTPForwarder {
	return &HTTPForwarder{
		Config: config,
	}
}
