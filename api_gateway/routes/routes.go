package routes

import (
	"github.com/Airiseina/answer/api_gateway/handle"
	"github.com/Airiseina/answer/api_gateway/middleware"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Routes(h *server.Hertz) {
	h.POST("/register", handle.Register)
	h.POST("/login", middleware.Authmiddleware.LoginHandler)
	h.GET("/files/*filepath", handle.FileProxy)
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
	authGroup.GET("/chat/messages", handle.GetHistory)
	authGroup.GET("/chat/conversations", handle.GetConversations)
	authGroup.POST("/chat/mark_read/:conversation_id", handle.MarkRead)
	authGroup.POST("/chat/online_status", handle.GetOnlineStatus)
	authGroup.POST("/chat/recall/:msg_id", handle.RecallMessage)
	authGroup.POST("/chat/edit/:msg_id", handle.EditMessage)
	authGroup.POST("/chat/edit_history/:msg_id", handle.GetEditHistory)
	authGroup.POST("/chat/sync", handle.SyncMessages)
	authGroup.POST("/chat/conversation_members", handle.GetConversationMembers)
	authGroup.POST("/update_avatar", handle.UpdateAvatar)
	authGroup.POST("/files", handle.Upload)

	authGroup.POST("/bot/create", handle.CreateBot)
	authGroup.POST("/bot/update", handle.UpdateBot)
	authGroup.POST("/bot/delete", handle.DeleteBot)
	authGroup.GET("/bot/list", handle.GetUserBots)
	authGroup.POST("/bot/add_to_conversation", handle.AddBotToConversation)
	authGroup.POST("/bot/chat", handle.ChatWithBot)
	authGroup.GET("/bot/system", handle.GetSystemBot)
}
