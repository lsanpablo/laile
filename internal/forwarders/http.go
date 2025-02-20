package forwarders

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"laile/internal"
	"laile/internal/config"
	"laile/internal/logger"
	"net/http"
	"strings"
)

type HTTPForwarder struct {
	Config *config.Forwarder
}

func (f *HTTPForwarder) Forward(ctx context.Context, event *DeliveryAttempt) (*DeliveryResult, error) {
	logger.Debug(ctx, "Preparing to forward request",
		"body_length", len(string(*event.Body)),
		"url", f.Config.URL,
		"method", event.Method)

	reader := strings.NewReader(string(*event.Body))
	req, err := http.NewRequest(event.Method, f.Config.URL, reader)
	if err != nil {
		logger.Error(ctx, "Failed to create request", err)
		return nil, err
	}

	// Copy headers from the original request to the proxy request
	headers, err := getHeadersFromBytes(event.Headers)
	if err != nil {
		logger.Error(ctx, "Failed to parse headers", err)
		return nil, err
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
		logger.Error(ctx, "Failed to parse query parameters", err)
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
	logger.Debug(ctx, "Sending request",
		"url", req.URL.String(),
		"method", req.Method,
		"header_count", len(req.Header))

	resp, err := client.Do(req)
	if err != nil {
		logger.Error(ctx, "Request failed", err,
			"url", req.URL.String(),
			"method", req.Method)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(ctx, "Failed to read response body", err)
		return nil, err
	}

	logger.Info(ctx, "Request completed successfully",
		"status_code", resp.StatusCode,
		"body_length", len(body),
		"url", req.URL.String())

	// Copy the target service response headers to the client response
	responseHeaders := resp.Header.Clone()
	responseHeadersJson := internal.HeadersToJson(responseHeaders)
	if err != nil {
		return nil, errors.New("failed to parse response headers")
	}

	return &DeliveryResult{
		StatusCode: resp.StatusCode,
		Headers:    responseHeadersJson,
		Body:       &body,
	}, nil
}

type Headers map[string][]string
type QueryParameters map[string][]string

func getHeadersFromBytes(b []byte) (Headers, error) {
	var headers Headers
	err := json.Unmarshal(b, &headers)
	if err != nil {
		return nil, err
	}
	return headers, nil
}
func getQueryParamsFromBytes(b []byte) (QueryParameters, error) {
	var queryParams QueryParameters
	err := json.Unmarshal(b, &queryParams)
	if err != nil {
		return nil, err
	}
	return queryParams, nil
}

func NewHTTPForwarder(config *config.Forwarder) *HTTPForwarder {
	return &HTTPForwarder{
		Config: config,
	}
}
