package azure

import "net/http"

type ApiError struct {
	StatusCode int
}

func (e ApiError) Error() string {
	return http.StatusText(e.StatusCode)

}
