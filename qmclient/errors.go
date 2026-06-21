package qmclient

import (
	"fmt"
	"net/http"
)

// APIError is returned when Quartermaster responds with a non-success status.
type APIError struct {
	StatusCode int
	Body       ErrorBody
}

func (e *APIError) Error() string {
	return fmt.Sprintf("quartermaster: %s (%s)", e.Body.Error, e.Body.ErrorDescription)
}

func errorFromResponse(status int, body *ErrorBody) error {
	if body == nil {
		return fmt.Errorf("quartermaster: unexpected status %s", http.StatusText(status))
	}
	return &APIError{StatusCode: status, Body: *body}
}

func firstErrorBody(bodies ...*ErrorBody) *ErrorBody {
	for _, b := range bodies {
		if b != nil {
			return b
		}
	}
	return nil
}
