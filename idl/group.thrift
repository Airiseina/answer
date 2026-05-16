namespace go group

struct Group {
    1: i64 group_id
    2: string name
    3: i64 owner_id
    4: string owner_name
    5: string notice
    6: i64 create_time
    7: i64 group_number
}

struct GroupMember {
    1: i64 group_id
    2: i64 user_id
    3: i64 role      // 2:"owner", 1:"admin", 0:"member"
    4: bool is_muted
    5: i64 join_time
    6: string name
}

struct CommonRes{
    1:bool success
}

struct CreateGroupReq {
    1: i64 creator_id
    2: string name
    3: list<i64> initial_members
}

struct CreateGroupRes {
    1: i64 group_id
    2: i64 group_number
}

struct InviteMembersReq {
    1: i64 inviter_id
    2: i64 group_id
    3: list<i64> user_ids
}

struct KickMembersReq {
    1: i64 operator_id
    2: i64 group_id
    3: list<i64> user_ids
}

struct GetGroupInfoReq {
    1: i64 group_id
}

struct GetGroupInfoRes {
    1: Group group
    2: list<GroupMember> members
}

struct ChangeOwnerReq{
    1:i64 old_id
    2:i64 new_id
    3:i64 group_id
}

struct ChangeNoticeReq{
    1:i64 operator_id
    2:i64 group_id
    3:string notice
}

struct MutedReq{
    1:i64 operator_id
    2:i64 group_id
    3:i64 muted_id
    4:bool is_muted
}

struct SetAdminReq {
    1: i64 operator_id
    2: i64 group_id
    3: i64 target_id
    4: i64 role
}

struct GetUserGroupsReq {
    1: i64 user_id
}

struct UserGroupInfo {
    1: i64 group_id
    2: string name
    3: i64 group_number
}

struct GetUserGroupsRes {
    1: list<UserGroupInfo> groups
}

struct SearchGroupByNumberReq {
    1: i64 group_number
}

struct GroupSearchResult {
    1: i64 group_id
    2: string name
    3: string owner_name
    4: i64 group_number
}

struct SearchGroupByNumberRes {
    1: GroupSearchResult group_info
}

struct JoinGroupReq {
    1: i64 user_id
    2: i64 group_number
    3: string message
}

struct HandleJoinReqReq {
    1: i64 operator_id
    2: i64 group_id
    3: i64 user_id
    4: bool accept
}

struct GetJoinRequestsReq {
    1: i64 group_id
}

struct JoinRequestInfo {
    1: i64 user_id
    2: string name
    3: string message
    4: i64 status   // 0:pending, 1:accepted, 2:rejected
}

struct GetJoinRequestsRes {
    1: list<JoinRequestInfo> requests
}

service GroupService {
    CreateGroupRes CreateGroup(1: CreateGroupReq req)
    CommonRes InviteMembers(1: InviteMembersReq req)
    CommonRes KickMembers(1: KickMembersReq req)
    GetGroupInfoRes GetGroupInfo(1: GetGroupInfoReq req)
    CommonRes ChangeOwner(1: ChangeOwnerReq req)
    CommonRes ChangeNotice(1: ChangeNoticeReq req)
    CommonRes Muted(1: MutedReq req)
    CommonRes SetAdmin(1: SetAdminReq req)
    GetUserGroupsRes GetUserGroups(1: GetUserGroupsReq req)
    SearchGroupByNumberRes SearchGroupByNumber(1: SearchGroupByNumberReq req)
    CommonRes JoinGroup(1: JoinGroupReq req)
    CommonRes HandleJoinReq(1: HandleJoinReqReq req)
    GetJoinRequestsRes GetJoinRequests(1: GetJoinRequestsReq req)
}
