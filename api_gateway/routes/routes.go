package routes

import (
	"api_gateway/handle"
	"api_gateway/middleware"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Routes(h *server.Hertz) {
	h.POST("/register", handle.Register)
	h.POST("/login", middleware.Authmiddleware.LoginHandler)
	authGroup := h.Group("/api", middleware.Authmiddleware.MiddlewareFunc())
	authGroup.POST("/create_group", handle.CreateGroup)
	authGroup.POST("/invite_members", handle.InviteMembers)
	authGroup.POST("/kick_members", handle.KickMembers)
	authGroup.GET("/get_group_info", handle.GetGroupInfo)
	authGroup.POST("/change_owner", handle.ChangeOwner)
	authGroup.POST("/change_notice", handle.ChangeNotice)
	authGroup.POST("/muted", handle.Muted)
	authGroup.POST("/set_admin", handle.SetAdmin)
	authGroup.POST("/add_friend", handle.AddFriend)
	authGroup.POST("/handle_friend_req", handle.HandleFriendReq)
	authGroup.POST("/delete_friend", handle.DeleteFriend)
	authGroup.GET("/get_friend_list", handle.GetFriendList)
	authGroup.GET("/get_friend_requests", handle.GetFriendRequests)
	authGroup.POST("/create_friend_group", handle.CreateFriendGroup)
	authGroup.POST("/update_friend_group", handle.UpdateFriendGroup)
	authGroup.POST("/delete_friend_group", handle.DeleteFriendGroup)
	authGroup.POST("/move_friend_to_group", handle.MoveFriendToGroup)
	authGroup.POST("/update_friend_remark", handle.UpdateFriendRemark)
	authGroup.GET("/get_friend_groups", handle.GetFriendGroups)
	authGroup.GET("/get_user_groups", handle.GetUserGroups)
	authGroup.GET("/search_group", handle.SearchGroupByNumber)
	authGroup.POST("/join_group", handle.JoinGroup)
	authGroup.POST("/handle_join_req", handle.HandleJoinReq)
	authGroup.GET("/get_join_requests", handle.GetJoinRequests)
	authGroup.GET("/search_user", handle.SearchUserByAccount)
	authGroup.GET("/v1/chat/history", handle.GetHistory)
	authGroup.GET("/v1/chat/conversations", handle.GetConversations)
}
