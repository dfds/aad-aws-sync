package azure

import (
	"fmt"
	"github.com/joomcode/errorx"
	"net/http"
	"strings"
)

type ApiError struct {
	StatusCode int
}

func (e ApiError) Error() string {
	return http.StatusText(e.StatusCode)

}

// unexpectedStatusError builds a diagnosable error for an unexpected HTTP
// status: it names the operation (caller embeds the relevant identifier) and
// includes the response body, which is where Graph returns the failure reason.
// The "status code: %d" substring is relied on by callers that string-match on
// it, so keep it intact.
func unexpectedStatusError(operation string, statusCode int, body []byte) error {
	return fmt.Errorf("%s returned unexpected status code: %d, body: %s", operation, statusCode, strings.TrimSpace(string(body)))
}

var (
	AzureError     = errorx.NewNamespace("azure")
	AdUserNotFound = AzureError.NewType("ad_user_not_found")
	HttpError403   = AzureError.NewType("http_error_403")
	HttpError404   = AzureError.NewType("http_error_404")
	HttpError      = AzureError.NewType("http_error")
)
