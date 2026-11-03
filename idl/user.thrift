namespace go user

struct CommonRes{
1:bool success
}
struct TenantBrief{
1:i64 id,
2:string name,
3:bool is_personal,
4:i16 role,
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
4:list<TenantBrief> tenants,
5:i64 personal_tenant_id,
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

// ===== 多租户（Phase 1）=====

struct TenantInfo{
1:i64 id,
2:string name,
3:string code,
4:string logo_url,
5:i64 owner_id,
6:bool is_personal,
7:i16 role,          // 当前用户在该租户的角色：2=owner 1=admin 0=member
8:i64 member_count,
9:i16 status,
}
struct CreateTenantReq{
1:i64 user_id,
2:string name,
3:string logo_url,
}
struct CreateTenantRes{
1:bool success,
2:TenantInfo tenant,
}
struct ListTenantsReq{
1:i64 user_id,
}
struct ListTenantsRes{
1:list<TenantInfo> tenants,
}
struct GetTenantReq{
1:i64 user_id,
2:i64 tenant_id,
}
struct GetTenantRes{
1:TenantInfo tenant,
}
struct UpdateTenantReq{
1:i64 user_id,
2:i64 tenant_id,
3:string name,
4:string logo_url,
}
struct TenantMemberInfo{
1:i64 user_id,
2:string account,
3:string name,
4:string avatar_url,
5:i16 role,
6:i64 dept_id,
7:i16 status,
}
struct ListTenantMembersReq{
1:i64 operator_id,
2:i64 tenant_id,
}
struct ListTenantMembersRes{
1:list<TenantMemberInfo> members,
}
struct AddTenantMemberReq{
1:i64 operator_id,
2:i64 tenant_id,
3:string account,      // 本轮按账号直接添加（不做邀请码）
4:i16 role,
5:i64 dept_id,
}
struct SetTenantMemberRoleReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 user_id,
4:i16 role,
}
struct RemoveTenantMemberReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 user_id,
}
struct TenantDepartmentInfo{
1:i64 id,
2:i64 tenant_id,
3:i64 parent_id,
4:string name,
5:i32 order,
}
struct ListDepartmentsReq{
1:i64 operator_id,
2:i64 tenant_id,
}
struct ListDepartmentsRes{
1:list<TenantDepartmentInfo> departments,
}
struct CreateDepartmentReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 parent_id,
4:string name,
}
struct CreateDepartmentRes{
1:bool success,
2:i64 dept_id,
}
struct RenameDepartmentReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 dept_id,
4:string name,
}
struct MoveDepartmentReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 dept_id,
4:i64 parent_id,
}
struct RemoveDepartmentReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 dept_id,
}
struct SetMemberDepartmentReq{
1:i64 operator_id,
2:i64 tenant_id,
3:i64 user_id,
4:i64 dept_id,
}

// ===== 系统任务枚举（Phase 4 日/周总结调度使用）=====

// ListActiveUsersReq 游标式枚举非 Bot 用户，供后台定时任务逐个生成总结
struct ListActiveUsersReq{
1:i64 after_user_id,   // 只返回 id > after_user_id 的用户，首页传 0
2:i32 limit,           // 单页条数，默认500，上限1000
}

struct ActiveUserInfo{
1:i64 user_id,
2:i64 tenant_id,       // 用户的个人租户 ID
}

struct ListActiveUsersRes{
1:list<ActiveUserInfo> users,
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
CreateTenantRes CreateTenant(1:CreateTenantReq req)
ListTenantsRes ListTenants(1:ListTenantsReq req)
GetTenantRes GetTenant(1:GetTenantReq req)
CommonRes UpdateTenant(1:UpdateTenantReq req)
ListTenantMembersRes ListTenantMembers(1:ListTenantMembersReq req)
CommonRes AddTenantMember(1:AddTenantMemberReq req)
CommonRes SetTenantMemberRole(1:SetTenantMemberRoleReq req)
CommonRes RemoveTenantMember(1:RemoveTenantMemberReq req)
ListDepartmentsRes ListDepartments(1:ListDepartmentsReq req)
CreateDepartmentRes CreateDepartment(1:CreateDepartmentReq req)
CommonRes RenameDepartment(1:RenameDepartmentReq req)
CommonRes MoveDepartment(1:MoveDepartmentReq req)
CommonRes RemoveDepartment(1:RemoveDepartmentReq req)
CommonRes SetMemberDepartment(1:SetMemberDepartmentReq req)
ListActiveUsersRes ListActiveUsers(1:ListActiveUsersReq req)
}
