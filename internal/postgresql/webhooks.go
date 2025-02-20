package db_models

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// HeaderFromHeaders returns the headers as a http.Header
func (w *Webhook) HeaderFromHeaders() (http.Header, error) {
	var headers http.Header
	err := json.Unmarshal(w.Headers, &headers)
	if err != nil {
		return nil, err
	}
	return headers, nil
}

// ValuesFromQueryParams returns the query params as a url.Values
func (w *Webhook) ValuesFromQueryParams() (url.Values, error) {
	var queryParams url.Values
	err := json.Unmarshal(w.QueryParams, &queryParams)
	if err != nil {
		return nil, err
	}
	return queryParams, nil
}
