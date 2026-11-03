package service

// 枚举用户分页的默认值与上限，防止调用方一次性拉取过多数据。
const (
	defaultActiveUserPageSize int32 = 500
	maxActiveUserPageSize     int32 = 1000
)

type UserNameDTO struct {
	Id        int64
	Name      string
	Account   string
	AvatarURL string
}

type SearchUserDTO struct {
	Id        int64
	Account   string
	Name      string
	AvatarURL string
}

func (dao *UserService) CheckUsersExist(UserIds []int64) (bool, error) {
	counts, err := dao.dao.CountUsersByIds(UserIds)
	if err != nil {
		return false, err
	}
	return int(counts) == len(UserIds), nil
}

func (dao *UserService) SearchByAccount(account string) (SearchUserDTO, error) {
	userInfo, err := dao.dao.GetUser(account)
	if err != nil {
		return SearchUserDTO{}, err
	}
	if userInfo.Account == "" {
		return SearchUserDTO{}, nil
	}
	return SearchUserDTO{
		Id:        userInfo.ID,
		Account:   userInfo.Account,
		Name:      userInfo.Name,
		AvatarURL: userInfo.AvatarURL,
	}, nil
}

func (dao *UserService) GetUserNames(userIds []int64) ([]UserNameDTO, error) {
	users, err := dao.dao.GetUsersByIds(userIds)
	if err != nil {
		return nil, err
	}
	var result []UserNameDTO
	for _, u := range users {
		result = append(result, UserNameDTO{
			Id:        u.ID,
			Name:      u.Name,
			Account:   u.Account,
			AvatarURL: u.AvatarURL,
		})
	}
	return result, nil
}

type UserAccountDTO struct {
	Id      int64
	Account string
}

type UserInfoDTO struct {
	Account   string
	Name      string
	AvatarURL string
}

func (dao *UserService) GetUsersInfoByAccounts(accounts []string) ([]UserInfoDTO, error) {
	users, err := dao.dao.GetUsersByAccounts(accounts)
	if err != nil {
		return nil, err
	}
	var result []UserInfoDTO
	for _, u := range users {
		result = append(result, UserInfoDTO{
			Account:   u.Account,
			Name:      u.Name,
			AvatarURL: u.AvatarURL,
		})
	}
	return result, nil
}

func (dao *UserService) UpdateAvatar(userID int64, avatarURL string) (bool, error) {
	err := dao.dao.UpdateAvatar(userID, avatarURL)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (dao *UserService) CreateBotUser(name, avatarURL string) (int64, error) {
	userID, err := dao.dao.CreateBotUser(name, avatarURL)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (dao *UserService) UpdateBotUserName(userID int64, name string) (bool, error) {
	err := dao.dao.UpdateBotUserName(userID, name)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (dao *UserService) GetUserIdsByAccounts(accounts []string) ([]UserAccountDTO, error) {
	users, err := dao.dao.GetUsersByAccounts(accounts)
	if err != nil {
		return nil, err
	}
	var result []UserAccountDTO
	for _, u := range users {
		result = append(result, UserAccountDTO{
			Id:      u.ID,
			Account: u.Account,
		})
	}
	return result, nil
}

// ActiveUserDTO 枚举用户的最小信息：用户ID + 个人租户ID。
type ActiveUserDTO struct {
	UserID   int64
	TenantID int64
}

// ListActiveUsers 游标式枚举非 Bot 用户，供日/周总结调度逐个生成。
func (dao *UserService) ListActiveUsers(afterUserID int64, limit int32) ([]ActiveUserDTO, error) {
	if limit <= 0 {
		limit = defaultActiveUserPageSize
	}
	if limit > maxActiveUserPageSize {
		limit = maxActiveUserPageSize
	}
	users, err := dao.dao.ListActiveUsers(afterUserID, int(limit))
	if err != nil {
		return nil, err
	}
	result := make([]ActiveUserDTO, 0, len(users))
	for _, u := range users {
		result = append(result, ActiveUserDTO{
			UserID:   u.ID,
			TenantID: u.TenantID,
		})
	}
	return result, nil
}
