package internal

import (
	"net/http"
	"net/url"
)

func HeadersToJSON(requestHeaders http.Header) map[string][]string {
	headers := make(map[string][]string)
	for name, values := range requestHeaders {
		headers[name] = values
	}

	return headers
}

// QueryParamsToJSON converts a map of query parameters to a map of strings.
func QueryParamsToJSON(requestQuery url.Values) map[string][]string {
	queryParams := make(map[string][]string)
	for name, values := range requestQuery {
		queryParams[name] = values
	}
	return queryParams
}
