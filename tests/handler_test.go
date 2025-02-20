package tests

import (
	"encoding/json"
	"laile/internal/server"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	s := &server.Server{}

	// Call the handler
	s.HelloWorldHandler(resp, req)

	// Assertions
	if resp.Code != http.StatusOK {
		t.Errorf("handler() wrong status code = %v", resp.Code)
		return
	}

	expected := map[string]string{"message": "Hello World"}
	var actual map[string]string

	// Decode the response body into the actual map
	if err := json.NewDecoder(resp.Body).Decode(&actual); err != nil {
		t.Errorf("handler() error decoding response body: %v", err)
		return
	}

	// Compare the decoded response with the expected value
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("handler() wrong response body. expected = %v, actual = %v", expected, actual)
		return
	}
}
