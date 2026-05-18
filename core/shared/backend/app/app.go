package app

import "github.com/gin-gonic/gin"

// App envuelve dependencias de runtime compartidas por entrypoints local/lambda.
type App struct {
	Router *gin.Engine
}
