package routes

import (
	"api_gateway/handle"
	"api_gateway/middleware"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
)

func Routes(h *server.Hertz) {
	h.POST("/register", handle.Register)
	h.POST("/login", middleware.Authmiddleware.LoginHandler)
	authGroup := h.Group("/api", middleware.Authmiddleware.MiddlewareFunc())
	authGroup.GET("/ws", adaptor.HertzHandler(http.HandlerFunc(handle.HandleWebsocket)))
	authGroup.POST("/create_group", handle.CreateGroup)
	authGroup.POST("/invite_members", handle.InviteMembers)
	authGroup.POST("/kick_members", handle.KickMembers)
	authGroup.GET("/get_group_info", handle.GetGroupInfo)
	authGroup.POST("/change_owner", handle.ChangeOwner)
	authGroup.POST("/change_notice", handle.ChangeNotice)
	authGroup.POST("/muted", handle.Muted)
	authGroup.POST("/set_admin", handle.SetAdmin)
	//authGroup.POST("/upload", stoClient.UploadFile)
}
