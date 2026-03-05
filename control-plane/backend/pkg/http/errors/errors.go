package errors

import "github.com/gin-gonic/gin"

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Write(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{Code: code, Message: message})
}
