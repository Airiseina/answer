package service

import (
	"context"
	"testing"

	"github.com/Airiseina/answer/kitex_service/admin_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// fakeAdminDao 是内存实现，用于在不依赖数据库的情况下验证服务层逻辑。
type fakeAdminDao struct {
	seq   int64
	byID  map[int64]model.AdminUser
	byAcc map[string]int64
}

func newFakeAdminDao() *fakeAdminDao {
	return &fakeAdminDao{
		byID:  map[int64]model.AdminUser{},
		byAcc: map[string]int64{},
	}
}

func (f *fakeAdminDao) CreateAdmin(account, name, hash, role string) (int64, error) {
	f.seq++
	id := f.seq
	f.byID[id] = model.AdminUser{
		BaseModel: model.BaseModel{ID: id},
		Account:   account,
		Name:      name,
		Hash:      hash,
		Role:      role,
		Status:    model.StatusEnabled,
	}
	f.byAcc[account] = id
	return id, nil
}

func (f *fakeAdminDao) GetAdminByAccount(account string) (model.AdminUser, error) {
	id, ok := f.byAcc[account]
	if !ok {
		return model.AdminUser{}, nil
	}
	return f.byID[id], nil
}

func (f *fakeAdminDao) GetAdminById(id int64) (model.AdminUser, error) {
	a, ok := f.byID[id]
	if !ok {
		return model.AdminUser{}, nil
	}
	return a, nil
}

func (f *fakeAdminDao) ListAdmins() ([]model.AdminUser, error) {
	list := make([]model.AdminUser, 0, len(f.byID))
	for i := int64(1); i <= f.seq; i++ {
		if a, ok := f.byID[i]; ok {
			list = append(list, a)
		}
	}
	return list, nil
}

func (f *fakeAdminDao) CountAdmins() (int64, error) {
	return int64(len(f.byID)), nil
}

func (f *fakeAdminDao) UpdatePassword(id int64, hash string) error {
	a := f.byID[id]
	a.Hash = hash
	f.byID[id] = a
	return nil
}

func (f *fakeAdminDao) UpdateStatus(id int64, status int16) error {
	a := f.byID[id]
	a.Status = status
	f.byID[id] = a
	return nil
}

func (f *fakeAdminDao) UpdateLastLogin(id int64, ts int64) error {
	a := f.byID[id]
	a.LastLoginAt = ts
	f.byID[id] = a
	return nil
}

func (f *fakeAdminDao) UpdateNameAndRole(id int64, name, role string) error {
	a := f.byID[id]
	a.Name = name
	a.Role = role
	f.byID[id] = a
	return nil
}

func newTestService(t *testing.T, account, password, role string) (*AdminService, *fakeAdminDao, int64) {
	t.Helper()
	dao := newFakeAdminDao()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("生成口令哈希失败: %v", err)
	}
	id, err := dao.CreateAdmin(account, account, string(hash), role)
	if err != nil {
		t.Fatalf("创建测试账号失败: %v", err)
	}
	return NewAdminService(dao, nil), dao, id
}

func TestLoginSuccessAndFailure(t *testing.T) {
	ctx := context.Background()
	svc, _, id := newTestService(t, "admin", "secret", model.RoleSuper)

	dto, err := svc.Login(ctx, "admin", "secret")
	if err != nil {
		t.Fatalf("登录不应报错: %v", err)
	}
	if dto.Id != id {
		t.Fatalf("登录成功应返回账号 ID，期望 %d，实际 %d", id, dto.Id)
	}
	if dto.LastLoginAt == 0 {
		t.Fatalf("登录成功后应刷新最后登录时间")
	}

	if dto, err := svc.Login(ctx, "admin", "wrong"); err != nil || dto.Id != 0 {
		t.Fatalf("错误口令应返回空结果，实际 (%+v, %v)", dto, err)
	}
	if dto, _ := svc.Login(ctx, "ghost", "secret"); dto.Id != 0 {
		t.Fatalf("不存在的账号应返回空结果，实际 %+v", dto)
	}
}

func TestLoginDisabledAccount(t *testing.T) {
	ctx := context.Background()
	svc, dao, id := newTestService(t, "ops", "secret", model.RoleOps)
	if err := dao.UpdateStatus(id, model.StatusDisabled); err != nil {
		t.Fatalf("停用账号失败: %v", err)
	}
	if dto, _ := svc.Login(ctx, "ops", "secret"); dto.Id != 0 {
		t.Fatalf("停用账号不应登录成功，实际 %+v", dto)
	}
}

func TestEnsureBootstrapIdempotent(t *testing.T) {
	ctx := context.Background()
	dao := newFakeAdminDao()
	svc := NewAdminService(dao, nil)

	entries := []config.AdminBootstrap{{
		Account:  "root",
		Name:     "平台管理员",
		Role:     model.RoleSuper,
		Password: "first-pass",
	}}
	if err := svc.EnsureBootstrap(ctx, entries); err != nil {
		t.Fatalf("首次引导应成功: %v", err)
	}

	// 再次引导时改名与口令：应只同步名称/角色，不覆盖口令。
	entries[0].Name = "总管理员"
	entries[0].Password = "second-pass"
	if err := svc.EnsureBootstrap(ctx, entries); err != nil {
		t.Fatalf("重复引导应成功: %v", err)
	}

	count, _ := dao.CountAdmins()
	if count != 1 {
		t.Fatalf("重复引导不应新建账号，期望 1，实际 %d", count)
	}
	if dto, _ := svc.Login(ctx, "root", "first-pass"); dto.Id == 0 {
		t.Fatalf("重复引导后原口令应仍然有效")
	}
	if dto, _ := svc.Login(ctx, "root", "second-pass"); dto.Id != 0 {
		t.Fatalf("重复引导不应覆盖已有口令")
	}
	admin, _ := dao.GetAdminByAccount("root")
	if admin.Name != "总管理员" {
		t.Fatalf("引导应同步名称，期望 总管理员，实际 %s", admin.Name)
	}
}

func TestCreateRequiresSuper(t *testing.T) {
	ctx := context.Background()
	svc, _, reviewerID := newTestService(t, "reviewer", "secret", model.RoleReviewer)

	ok, _, err := svc.Create(ctx, reviewerID, "newbie", "新人", model.RoleOps, "pass")
	if err != nil {
		t.Fatalf("非超管创建不应报错: %v", err)
	}
	if ok {
		t.Fatalf("非超管不应创建成功")
	}
}

func TestCreateBySuper(t *testing.T) {
	ctx := context.Background()
	svc, _, superID := newTestService(t, "root", "secret", model.RoleSuper)

	ok, id, err := svc.Create(ctx, superID, "ops1", "运营", model.RoleOps, "pass")
	if err != nil {
		t.Fatalf("超管创建应成功: %v", err)
	}
	if !ok || id == 0 {
		t.Fatalf("超管创建应返回成功与新 ID，实际 ok=%v id=%d", ok, id)
	}
	// 重复账号应被拒绝。
	if ok, _, _ := svc.Create(ctx, superID, "ops1", "运营2", model.RoleOps, "pass"); ok {
		t.Fatalf("重复账号不应创建成功")
	}
	// 非法角色应被拒绝。
	if ok, _, _ := svc.Create(ctx, superID, "ops2", "运营2", "unknown", "pass"); ok {
		t.Fatalf("非法角色不应创建成功")
	}
}

func TestUpdateStatusCannotDisableSelf(t *testing.T) {
	ctx := context.Background()
	svc, _, superID := newTestService(t, "root", "secret", model.RoleSuper)

	ok, err := svc.UpdateStatus(ctx, superID, superID, model.StatusDisabled)
	if err != nil {
		t.Fatalf("停用自己不应报错: %v", err)
	}
	if ok {
		t.Fatalf("不应允许停用自己")
	}

	_, targetID, _ := svc.Create(ctx, superID, "ops1", "运营", model.RoleOps, "pass")
	ok, err = svc.UpdateStatus(ctx, superID, targetID, model.StatusDisabled)
	if err != nil || !ok {
		t.Fatalf("超管应能停用他人，实际 ok=%v err=%v", ok, err)
	}
}
