namespace go user

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
}
struct CheckUsersExistReq{
1:list<i64> userIds,
}
struct CheckUsersExistRes{
1:bool allExist,
}

service LoginService{
RegisterRes Register(1:RegisterReq req)
LoginRes Login(1:LoginReq req)
CheckUsersExistRes CheckUsersExist(1:CheckUsersExistReq req)
}
