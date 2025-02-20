package event

import (
	"fmt"
	"laile/internal/config"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"
)

type Event struct {
	ID        int64
	WebhookID int64
	Body      string
}

func (e *Event) Validate() error {
	return validation.ValidateStruct(e,
		// Street cannot be empty, and the length must between 5 and 50
		validation.Field(e.Body, validation.Required, validation.Length(0, 2056)),
	)
}

func NewIdempotencyKey(eventID int64, servicePath string) string {
	return fmt.Sprintf("event:v1-%d-%s", eventID, servicePath)
}

type WebhookService struct {
	ID     string
	Name   string
	Path   string
	Config *config.WebhookService
}

func (ls *WebhookService) IsAuthenticated(request *http.Request) (bool, error) {
	return true, nil
}
