package azure

import (
	"github.com/joomcode/errorx"
	"net/http"
)

type ApiError struct {
	StatusCode int
}

func (e ApiError) Error() string {
	return http.StatusText(e.StatusCode)

}

var (
	AzureError     = errorx.NewNamespace("azure")
	AdUserNotFound = AzureError.NewType("ad_user_not_found")
	HttpError403   = AzureError.NewType("http_error_403")
	HttpError      = AzureError.NewType("http_error")
)
