package service

func (dao *UserService) CheckUsersExist(UserIds []int64) (bool, error) {
	counts, err := dao.dao.CountUsersByIds(UserIds)
	if err != nil {
		return false, err
	}
	return int(counts) == len(UserIds), nil
}
