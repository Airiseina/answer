package service

import (
	"github.com/Airiseina/answer/kitex_service/user_service/internal/dal/mysql"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	dao       mysql.UserDao
	tenantDao mysql.TenantDao
}

func NewUserService(dao mysql.UserDao, tenantDao mysql.TenantDao) *UserService {
	return &UserService{dao, tenantDao}
}

func (dao *UserService) Register(account, name, password string) (bool, error) {
	if account == "" || name == "" || password == "" {
		return false, nil
	}
	userInfo, err := dao.dao.GetUser(account)
	if err != nil {
		return false, err
	}
	if userInfo.Account != "" {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, nil
	}
	// 注册与个人租户创建在同一事务内完成，避免出现没有归属租户的用户。
	if _, _, err = dao.tenantDao.CreateUserWithPersonalTenant(account, name, string(hash)); err != nil {
		return false, err
	}
	return true, nil
}

type UserDTO struct {
	Id               int64            `json:"id"`
	Account          string           `json:"account"`
	AvatarURL        string           `json:"avatar_url"`
	Tenants          []TenantBriefDTO `json:"tenants"`
	PersonalTenantID int64            `json:"personal_tenant_id"`
}

// TenantBriefDTO 是登录返回的租户摘要（用于前端切换租户）。
type TenantBriefDTO struct {
	Id         int64
	Name       string
	IsPersonal bool
	Role       int16
}

func (dao *UserService) Login(account string, password string) (UserDTO, error) {
	userInfo, err := dao.dao.GetUser(account)
	if err != nil {
		return UserDTO{}, err
	}
	if userInfo.Account == "" {
		return UserDTO{}, nil
	}
	err = bcrypt.CompareHashAndPassword([]byte(userInfo.Hash), []byte(password))
	if err != nil {
		return UserDTO{}, nil
	}
	tenants, err := dao.tenantDao.ListTenantsByUser(userInfo.ID)
	if err != nil {
		return UserDTO{}, err
	}
	user := UserDTO{
		Id:        userInfo.ID,
		Account:   userInfo.Account,
		AvatarURL: userInfo.AvatarURL,
	}
	for _, t := range tenants {
		user.Tenants = append(user.Tenants, TenantBriefDTO{
			Id:         t.ID,
			Name:       t.Name,
			IsPersonal: t.IsPersonal,
			Role:       t.Role,
		})
		if t.IsPersonal {
			user.PersonalTenantID = t.ID
		}
	}
	return user, nil
}
