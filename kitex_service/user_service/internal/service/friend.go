package service

import (
	"user_service/internal/dal/mysql"
	"user_service/internal/model"
)

type FriendService struct {
	dao mysql.UserDao
}

func NewFriendService(dao mysql.UserDao) *FriendService {
	return &FriendService{dao}
}

// 加入查询全部分组
func (s *FriendService) AddFriend(userID, receiver int64, message string) (bool, error) {
	if userID == receiver {
		return false, nil
	}
	receiverUser, err := s.dao.GetUserById(receiver)
	if err != nil {
		return false, err
	}
	if receiverUser.ID == 0 {
		return false, nil
	}
	friend, err := s.dao.GetFriend(userID, receiver)
	if err != nil {
		return false, err
	}
	if friend.ID != 0 {
		return false, nil
	}
	existingReq, err := s.dao.GetFriendRequestBetweenUsers(userID, receiver)
	if err != nil {
		return false, err
	}
	if existingReq.ID != 0 && existingReq.Status == model.FriendRequestPending {
		return false, nil
	}
	err = s.dao.CreateFriendRequest(userID, receiver, message)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *FriendService) HandleFriendReq(sender, userID int64, accept bool) (bool, error) {
	req, err := s.dao.GetFriendRequest(sender, userID)
	if err != nil {
		return false, err
	}
	if req.ID == 0 {
		return false, nil
	}
	if req.Receiver != userID {
		return false, nil
	}
	if req.Status != model.FriendRequestPending {
		return false, nil
	}
	if accept {
		err = s.dao.UpdateFriendRequestStatus(sender, userID, model.FriendRequestAccepted)
		if err != nil {
			return false, err
		}
		err = s.dao.CreateFriend(req.Sender, req.Receiver, 0)
		if err != nil {
			return false, err
		}
		err = s.dao.CreateFriend(req.Receiver, req.Sender, 0)
		if err != nil {
			return false, err
		}
	} else {
		err = s.dao.UpdateFriendRequestStatus(sender, userID, model.FriendRequestRejected)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *FriendService) DeleteFriend(userID, friendID int64) (bool, error) {
	friend, err := s.dao.GetFriend(userID, friendID)
	if err != nil {
		return false, err
	}
	if friend.ID == 0 {
		return false, nil
	}
	err = s.dao.DeleteFriend(userID, friendID)
	if err != nil {
		return false, err
	}
	err = s.dao.DeleteFriend(friendID, userID)
	if err != nil {
		return false, err
	}
	return true, nil
}

type FriendDTO struct {
	FriendID int64
	Remark   string
	GroupID  int64
	Name     string
}

func (s *FriendService) GetFriendList(userID int64) ([]FriendDTO, error) {
	friends, err := s.dao.GetFriendList(userID)
	if err != nil {
		return nil, err
	}
	var result []FriendDTO
	for _, f := range friends {
		friendUser, err := s.dao.GetUserById(f.FriendID)
		if err != nil {
			return nil, err
		}
		result = append(result, FriendDTO{
			FriendID: f.FriendID,
			Remark:   f.Remark, //并未有设置备注的函数
			GroupID:  f.GroupID,
			Name:     friendUser.Name,
		})
	}
	return result, nil
}

type FriendRequestDTO struct {
	Sender   int64
	Receiver int64
	Message  string
	Status   int64
}

func (s *FriendService) GetFriendRequests(userID int64) ([]FriendRequestDTO, error) {
	requests, err := s.dao.GetFriendRequestsByReceiver(userID)
	if err != nil {
		return nil, err
	}
	var result []FriendRequestDTO
	for _, r := range requests {
		result = append(result, FriendRequestDTO{
			Sender:   r.Sender,
			Receiver: r.Receiver,
			Message:  r.Message,
			Status:   r.Status,
		})
	}
	return result, nil
}

func (s *FriendService) CreateFriendGroup(userID int64, name string) (int64, error) {
	if name == "" {
		return 0, nil
	}
	groupID, err := s.dao.CreateFriendGroup(userID, name)
	if err != nil {
		return 0, err
	}
	return groupID, nil
}

func (s *FriendService) UpdateFriendGroup(groupID, userID int64, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	group, err := s.dao.GetFriendGroup(groupID)
	if err != nil {
		return false, err
	}
	if group.ID == 0 {
		return false, nil
	}
	if group.UserID != userID {
		return false, nil
	}
	err = s.dao.UpdateFriendGroup(groupID, name)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *FriendService) DeleteFriendGroup(groupID, userID int64) (bool, error) {
	group, err := s.dao.GetFriendGroup(groupID)
	if err != nil {
		return false, err
	}
	if group.ID == 0 {
		return false, nil
	}
	if group.UserID != userID {
		return false, nil
	}
	err = s.dao.ResetFriendsGroupID(groupID)
	if err != nil {
		return false, err
	}
	err = s.dao.DeleteFriendGroup(groupID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *FriendService) MoveFriendToGroup(userID, friendID, groupID int64) (bool, error) {
	friend, err := s.dao.GetFriend(userID, friendID)
	if err != nil {
		return false, err
	}
	if friend.ID == 0 {
		return false, nil
	}
	if groupID != 0 {
		group, err := s.dao.GetFriendGroup(groupID)
		if err != nil {
			return false, err
		}
		if group.ID == 0 {
			return false, nil
		}
		if group.UserID != userID {
			return false, nil
		}
	}
	err = s.dao.UpdateFriendGroupID(userID, friendID, groupID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *FriendService) UpdateFriendRemark(userID, friendID int64, remark string) (bool, error) {
	friend, err := s.dao.GetFriend(userID, friendID)
	if err != nil {
		return false, err
	}
	if friend.ID == 0 {
		return false, nil
	}
	err = s.dao.UpdateFriendRemark(userID, friendID, remark)
	if err != nil {
		return false, err
	}
	return true, nil
}

type FriendGroupDTO struct {
	GroupID int64
	Name    string
}

func (s *FriendService) GetFriendGroups(userID int64) ([]FriendGroupDTO, error) {
	groups, err := s.dao.GetFriendGroupsByUserId(userID)
	if err != nil {
		return nil, err
	}
	var result []FriendGroupDTO
	for _, g := range groups {
		result = append(result, FriendGroupDTO{
			GroupID: g.ID,
			Name:    g.Name,
		})
	}
	return result, nil
}
