package core

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the standard error replied by the API.
// Note that each route/route group may implement its own replies.
type ErrorResponse struct {
	Message string      `json:"message"`
	Code    int         `json:"code"`
	Data    interface{} `json:"data,omitempty"`
}

func (er ErrorResponse) Error() string {
	return er.Message
}

// NewErrorResponse returns an ErrorResponse with the specified message.
func NewErrorResponse(str string) *ErrorResponse {
	return &ErrorResponse{Message: str}
}

// NewErrorResponseWithCode returns an ErrorResponse with the specified message and code.
func NewErrorResponseWithCode(str string, code int) *ErrorResponse {
	return &ErrorResponse{Message: str, Code: code}
}

// PrintError sent error message and tag string to standart output
func PrintError(err error, tag string) {
	if err != nil && flag.Lookup("test.v") == nil {
		fmt.Printf("| Error | %v: %#v \n", tag, err)
	}
}

// HTTPError returns a standard error to HTTP client
func HTTPError(c *gin.Context, err error) {
	var er ErrorResponse
	if e, ok := err.(ErrorResponse); ok {
		er = e
	}

	if er.Code == 0 {
		er.Code = http.StatusInternalServerError
	}

	if er.Message == "" {
		er.Message = err.Error()
	}

	c.JSON(er.Code, er)
}
