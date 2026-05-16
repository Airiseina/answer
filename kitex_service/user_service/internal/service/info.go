package service

type UserNameDTO struct {
	Id   int64
	Name string
}

type SearchUserDTO struct {
	Id      int64
	Account string
	Name    string
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
		Id:      userInfo.ID,
		Account: userInfo.Account,
		Name:    userInfo.Name,
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
			Id:   u.ID,
			Name: u.Name,
		})
	}
	return result, nil
}
