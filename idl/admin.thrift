namespace go admin

struct CommonRes{
1:bool success
}

struct AdminLoginReq{
1:string account,
2:string password,
}

struct AdminLoginRes{
1:i64 id,
2:string account,
3:string name,
4:string role,
}

struct AdminInfo{
1:i64 id,
2:string account,
3:string name,
4:string role,
5:i16 status,
6:i64 last_login_at,
}

struct ChangePasswordReq{
1:i64 admin_id,
2:string old_password,
3:string update_password,
}

struct GetAdminReq{
1:i64 admin_id,
}

struct GetAdminRes{
1:AdminInfo admin,
}

struct ListAdminsReq{
1:i64 operator_id,
}

struct ListAdminsRes{
1:list<AdminInfo> admins,
}

struct CreateAdminReq{
1:i64 operator_id,
2:string account,
3:string name,
4:string role,
5:string password,
}

struct CreateAdminRes{
1:bool success,
2:i64 admin_id,
}

struct UpdateAdminStatusReq{
1:i64 operator_id,
2:i64 admin_id,
3:i16 status,
}

service AdminService{
AdminLoginRes Login(1:AdminLoginReq req)
CommonRes ChangePassword(1:ChangePasswordReq req)
GetAdminRes GetAdmin(1:GetAdminReq req)
ListAdminsRes ListAdmins(1:ListAdminsReq req)
CreateAdminRes CreateAdmin(1:CreateAdminReq req)
CommonRes UpdateAdminStatus(1:UpdateAdminStatusReq req)
}
