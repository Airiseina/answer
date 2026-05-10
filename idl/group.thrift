namespace go group

struct Group {
    1: i64 group_id
    2: string name
    3: i64 owner_id
    4: string notice  //群公告
    5: i64 create_time
}

struct GroupMember {
    1: i64 group_id
    2: i64 user_id
    3: i64 role      // 2:"owner", 1:"admin", 0:"member"
    4: bool is_muted
    5: i64 join_time
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
}

struct InviteMembersReq {
    1: i64 inviter_id
    2: i64 group_id
    3: list<i64> user_ids // 被拉的人
}

struct KickMembersReq {
    1: i64 operator_id
    2: i64 group_id
    3: list<i64> user_ids // 被踢的人
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
    4: i64 role // 1 for admin, 0 to revoke admin
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
}