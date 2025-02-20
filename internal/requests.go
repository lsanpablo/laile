package internal

import (
	"encoding/json"
	"net/http"
	"net/url"
)

func HeadersToJson(requestHeaders http.Header) map[string][]string {
	headers := make(map[string][]string)
	for name, values := range requestHeaders {
		headers[name] = values
	}

	return headers
}

func BytesToHeaders(headers []byte) (http.Header, error) {
	var headerMap map[string][]string
	err := json.Unmarshal(headers, &headerMap)
	if err != nil {
		return nil, err
	}
	return headerMap, nil
}

// QueryParamsToJson converts a map of query parameters to a map of strings
func QueryParamsToJson(requestQuery url.Values) map[string][]string {
	queryParams := make(map[string][]string)
	for name, values := range requestQuery {
		queryParams[name] = values
	}
	return queryParams
}
func BytesToQueryParams(queryParams []byte) (url.Values, error) {
	var queryMap map[string][]string
	err := json.Unmarshal(queryParams, &queryMap)
	if err != nil {
		return nil, err
	}
	return queryMap, nil
}
