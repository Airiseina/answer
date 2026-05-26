namespace go user

struct CommonRes{
1:bool success
}
struct RegisterReq{
1:string account,
2:string name,
3:string password,
}
struct RegisterRes{
1:bool isExit,
}
struct LoginReq{
1:string account,
2:string password,
}
struct LoginRes{
1:i64 id,
2:string account,
3:string avatar_url,
}
struct CheckUsersExistReq{
1:list<i64> userIds,
}
struct CheckUsersExistRes{
1:bool allExist,
}
struct AddFriendReq{
1:i64 user_id,
2:i64 receiver,
3:string message,
}
struct HandleFriendReqReq{
1:i64 sender,
2:i64 user_id,
3:bool accept,
}
struct DeleteFriendReq{
1:i64 user_id,
2:i64 friend_id,
}
struct GetFriendListReq{
1:i64 user_id,
}
struct FriendInfo{
1:i64 friend_id,
2:string remark,
3:i64 group_id
4:string name,
}
struct GetFriendListRes{
1:list<FriendInfo> friends,
}
struct GetFriendRequestsReq{
1:i64 user_id,
}
struct FriendRequestInfo{
1:i64 sender,
2:i64 receiver,
3:string message,
4:i64 status,
}
struct GetFriendRequestsRes{
1:list<FriendRequestInfo> requests,
}
struct CreateFriendGroupReq{
1:i64 user_id,
2:string name,
}
struct CreateFriendGroupRes{
1:i64 group_id,
}
struct UpdateFriendGroupReq{
1:i64 group_id,
2:i64 user_id,
3:string name,
}
struct DeleteFriendGroupReq{
1:i64 group_id,
2:i64 user_id,
}
struct MoveFriendToGroupReq{
1:i64 user_id,
2:i64 friend_id,
3:i64 group_id,
}
struct UpdateFriendRemarkReq{
1:i64 user_id,
2:i64 friend_id,
3:string remark,
}
struct GetFriendGroupsReq{
1:i64 user_id,
}
struct FriendGroupInfo{
1:i64 group_id,
2:string name,
}
struct GetFriendGroupsRes{
1:list<FriendGroupInfo> groups,
}
struct SearchUserByAccountReq{
1:string account,
}
struct SearchUserResult{
1:i64 id,
2:string account,
3:string name,
4:string avatar_url,
}
struct SearchUserByAccountRes{
1:SearchUserResult user_info,
}
struct GetUserNamesReq{
1:list<i64> user_ids,
}
struct UserNameInfo{
1:i64 id,
2:string name,
3:string account,
4:string avatar_url,
}
struct GetUserNamesRes{
1:list<UserNameInfo> users,
}
struct GetUserIdsByAccountsReq{
1:list<string> accounts,
}
struct UserAccountPair{
1:i64 id,
2:string account,
}
struct GetUserIdsByAccountsRes{
1:list<UserAccountPair> users,
}
struct GetUsersInfoByAccountsReq{
1:list<string> accounts,
}
struct UserInfoItem{
1:string account,
2:string name,
3:string avatar_url,
}
struct GetUsersInfoByAccountsRes{
1:list<UserInfoItem> users,
}
struct UpdateAvatarReq{
1:i64 user_id,
2:string avatar_url,
}
struct CreateBotUserReq{
1:string name,
2:string avatar_url,
}
struct CreateBotUserRes{
1:bool success,
2:i64 user_id,
}
struct UpdateBotUserNameReq{
1:i64 user_id,
2:string name,
}
service LoginService{
RegisterRes Register(1:RegisterReq req)
LoginRes Login(1:LoginReq req)
CheckUsersExistRes CheckUsersExist(1:CheckUsersExistReq req)
CommonRes AddFriend(1:AddFriendReq req)
CommonRes HandleFriendReq(1:HandleFriendReqReq req)
CommonRes DeleteFriend(1:DeleteFriendReq req)
GetFriendListRes GetFriendList(1:GetFriendListReq req)
GetFriendRequestsRes GetFriendRequests(1:GetFriendRequestsReq req)
CreateFriendGroupRes CreateFriendGroup(1:CreateFriendGroupReq req)
CommonRes UpdateFriendGroup(1:UpdateFriendGroupReq req)
CommonRes DeleteFriendGroup(1:DeleteFriendGroupReq req)
CommonRes MoveFriendToGroup(1:MoveFriendToGroupReq req)
CommonRes UpdateFriendRemark(1:UpdateFriendRemarkReq req)
GetFriendGroupsRes GetFriendGroups(1:GetFriendGroupsReq req)
SearchUserByAccountRes SearchUserByAccount(1:SearchUserByAccountReq req)
GetUserNamesRes GetUserNames(1:GetUserNamesReq req)
GetUserIdsByAccountsRes GetUserIdsByAccounts(1:GetUserIdsByAccountsReq req)
GetUsersInfoByAccountsRes GetUsersInfoByAccounts(1:GetUsersInfoByAccountsReq req)
CommonRes UpdateAvatar(1:UpdateAvatarReq req)
CreateBotUserRes CreateBotUser(1:CreateBotUserReq req)
CommonRes UpdateBotUserName(1:UpdateBotUserNameReq req)
}
