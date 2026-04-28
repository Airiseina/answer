package routes

import (
	"answer/internal/api/handle"
	"answer/internal/api/middleware"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
)

func Routes(h *server.Hertz, stoClient *handle.UploadController) {
	h.POST("/register", handle.Register)
	h.POST("/login", middleware.Authmiddleware.LoginHandler)
	authGroup := h.Group("/api", middleware.Authmiddleware.MiddlewareFunc())
	authGroup.GET("/ws", adaptor.HertzHandler(http.HandlerFunc(handle.HandleWebsocket)))
	authGroup.POST("/upload", stoClient.UploadFile)
}
