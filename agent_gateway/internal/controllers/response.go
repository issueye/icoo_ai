package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func JSON(c *gin.Context, status int, value any) {
	c.JSON(status, value)
}

func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{Code: code, Message: message})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "bad_request", message)
}

func InternalError(c *gin.Context, err error) {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	Error(c, http.StatusInternalServerError, "internal_error", msg)
}
