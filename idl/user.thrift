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
1:i16 id,
2:string account,
}
service LoginService{
RegisterRes Register(1:RegisterReq req)
LoginRes Login(1:LoginReq req)
}
