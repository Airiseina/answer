package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Airiseina/answer/kitex_service/user_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/model"
)

// fakeUserDao 只实现租户逻辑用到的用户查询，其余方法由嵌入接口兜底。
type fakeUserDao struct {
	mysql.UserDao
	accounts map[string]model.User
}

func newFakeUserDao() *fakeUserDao {
	return &fakeUserDao{accounts: map[string]model.User{}}
}

func (f *fakeUserDao) add(id int64, account, name string) {
	f.accounts[account] = model.User{
		BaseModel: model.BaseModel{ID: id},
		Account:   account,
		Name:      name,
	}
}

func (f *fakeUserDao) GetUser(account string) (model.User, error) {
	return f.accounts[account], nil
}

func (f *fakeUserDao) GetUsersByIds(ids []int64) ([]model.User, error) {
	var result []model.User
	for _, u := range f.accounts {
		for _, id := range ids {
			if u.ID == id {
				result = append(result, u)
			}
		}
	}
	return result, nil
}

func memberKey(tenantID, userID int64) string {
	return fmt.Sprintf("%d:%d", tenantID, userID)
}

// fakeTenantDao 是租户表的内存实现，用于离线校验服务层权限与树形逻辑。
type fakeTenantDao struct {
	mysql.TenantDao
	tenantSeq int64
	deptSeq   int64
	tenants   map[int64]model.Tenant
	members   map[string]model.TenantMember
	depts     map[int64]model.TenantDepartment
}

func newFakeTenantDao() *fakeTenantDao {
	return &fakeTenantDao{
		tenants: map[int64]model.Tenant{},
		members: map[string]model.TenantMember{},
		depts:   map[int64]model.TenantDepartment{},
	}
}

func (f *fakeTenantDao) CreateUserWithPersonalTenant(account, name, hash string) (model.User, model.Tenant, error) {
	f.tenantSeq++
	tenant := model.Tenant{
		BaseModel:  model.BaseModel{ID: f.tenantSeq},
		Name:       name,
		Code:       "p_test",
		IsPersonal: true,
		Status:     model.TenantStatusNormal,
	}
	f.tenants[tenant.ID] = tenant
	f.members[memberKey(tenant.ID, 1)] = model.TenantMember{
		TenantID: tenant.ID, UserID: 1, Role: model.TenantRoleOwner, Status: model.TenantMemberNormal,
	}
	return model.User{BaseModel: model.BaseModel{ID: 1}, Account: account, Name: name}, tenant, nil
}

func (f *fakeTenantDao) CreateTenantWithOwner(name, code, logoURL string, ownerID int64) (model.Tenant, error) {
	f.tenantSeq++
	tenant := model.Tenant{
		BaseModel: model.BaseModel{ID: f.tenantSeq},
		Name:      name,
		Code:      code,
		LogoURL:   logoURL,
		OwnerID:   ownerID,
		Status:    model.TenantStatusNormal,
	}
	f.tenants[tenant.ID] = tenant
	f.members[memberKey(tenant.ID, ownerID)] = model.TenantMember{
		TenantID: tenant.ID, UserID: ownerID, Role: model.TenantRoleOwner, Status: model.TenantMemberNormal,
	}
	return tenant, nil
}

func (f *fakeTenantDao) GetTenant(tenantID int64) (model.Tenant, error) {
	return f.tenants[tenantID], nil
}

func (f *fakeTenantDao) UpdateTenant(tenantID int64, name, logoURL string) error {
	t, ok := f.tenants[tenantID]
	if !ok {
		return nil
	}
	if name != "" {
		t.Name = name
	}
	if logoURL != "" {
		t.LogoURL = logoURL
	}
	f.tenants[tenantID] = t
	return nil
}

func (f *fakeTenantDao) ListTenantsByUser(userID int64) ([]mysql.MemberTenant, error) {
	var result []mysql.MemberTenant
	for _, m := range f.members {
		if m.UserID == userID && m.Status != model.TenantMemberRemoved {
			result = append(result, mysql.MemberTenant{Tenant: f.tenants[m.TenantID], Role: m.Role})
		}
	}
	return result, nil
}

func (f *fakeTenantDao) CountTenantMembers(tenantID int64) (int64, error) {
	var count int64
	for _, m := range f.members {
		if m.TenantID == tenantID && m.Status != model.TenantMemberRemoved {
			count++
		}
	}
	return count, nil
}

func (f *fakeTenantDao) GetTenantMember(tenantID, userID int64) (model.TenantMember, error) {
	return f.members[memberKey(tenantID, userID)], nil
}

func (f *fakeTenantDao) ListTenantMembers(tenantID int64) ([]model.TenantMember, error) {
	var result []model.TenantMember
	for _, m := range f.members {
		if m.TenantID == tenantID && m.Status != model.TenantMemberRemoved {
			result = append(result, m)
		}
	}
	return result, nil
}

func (f *fakeTenantDao) UpsertTenantMember(tenantID, userID int64, role int16, deptID int64) error {
	f.members[memberKey(tenantID, userID)] = model.TenantMember{
		TenantID: tenantID, UserID: userID, Role: role, DeptID: deptID, Status: model.TenantMemberNormal,
	}
	return nil
}

func (f *fakeTenantDao) UpdateTenantMemberRole(tenantID, userID int64, role int16) error {
	m := f.members[memberKey(tenantID, userID)]
	m.Role = role
	f.members[memberKey(tenantID, userID)] = m
	return nil
}

func (f *fakeTenantDao) UpdateTenantMemberDept(tenantID, userID int64, deptID int64) error {
	m := f.members[memberKey(tenantID, userID)]
	m.DeptID = deptID
	f.members[memberKey(tenantID, userID)] = m
	return nil
}

func (f *fakeTenantDao) RemoveTenantMember(tenantID, userID int64) error {
	m := f.members[memberKey(tenantID, userID)]
	m.Status = model.TenantMemberRemoved
	f.members[memberKey(tenantID, userID)] = m
	return nil
}

func (f *fakeTenantDao) CreateDepartment(dept model.TenantDepartment) (model.TenantDepartment, error) {
	f.deptSeq++
	dept.ID = f.deptSeq
	f.depts[dept.ID] = dept
	return dept, nil
}

func (f *fakeTenantDao) GetDepartment(deptID int64) (model.TenantDepartment, error) {
	return f.depts[deptID], nil
}

func (f *fakeTenantDao) ListDepartments(tenantID int64) ([]model.TenantDepartment, error) {
	var result []model.TenantDepartment
	for _, d := range f.depts {
		if d.TenantID == tenantID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (f *fakeTenantDao) UpdateDepartmentName(deptID int64, name string) error {
	d := f.depts[deptID]
	d.Name = name
	f.depts[deptID] = d
	return nil
}

func (f *fakeTenantDao) UpdateDepartmentParent(deptID, parentID int64) error {
	d := f.depts[deptID]
	d.ParentID = parentID
	f.depts[deptID] = d
	return nil
}

func (f *fakeTenantDao) RemoveDepartment(dept model.TenantDepartment) error {
	// 与真实 DAO 一致：子部门上挂到被删部门的父节点，成员回到未分配。
	for id, d := range f.depts {
		if d.TenantID == dept.TenantID && d.ParentID == dept.ID {
			d.ParentID = dept.ParentID
			f.depts[id] = d
		}
	}
	for key, m := range f.members {
		if m.TenantID == dept.TenantID && m.DeptID == dept.ID {
			m.DeptID = 0
			f.members[key] = m
		}
	}
	delete(f.depts, dept.ID)
	return nil
}

func newTestTenantService() (*TenantService, *fakeUserDao, *fakeTenantDao) {
	userDao := newFakeUserDao()
	userDao.add(1, "owner", "所有者")
	userDao.add(2, "member", "成员")
	userDao.add(3, "outsider", "路人")
	tenantDao := newFakeTenantDao()
	return NewTenantService(userDao, tenantDao), userDao, tenantDao
}

func TestCreateTenantGrantsOwnerRole(t *testing.T) {
	svc, _, _ := newTestTenantService()
	tenant, err := svc.CreateTenant(1, "示例团队", "")
	if err != nil {
		t.Fatalf("创建租户不应报错: %v", err)
	}
	if tenant.Id == 0 || tenant.Role != model.TenantRoleOwner {
		t.Fatalf("创建者应成为 owner，实际 %+v", tenant)
	}
	tenants, err := svc.ListTenants(1)
	if err != nil || len(tenants) != 1 {
		t.Fatalf("列表应包含新建租户，实际 (%v, %v)", tenants, err)
	}
}

func TestUpdateTenantRequiresAdmin(t *testing.T) {
	svc, _, dao := newTestTenantService()
	tenant, _ := svc.CreateTenant(1, "示例团队", "")
	dao.UpsertTenantMember(tenant.Id, 2, model.TenantRoleMember, 0)
	dao.UpsertTenantMember(tenant.Id, 3, model.TenantRoleAdmin, 0)

	ok, err := svc.UpdateTenant(2, tenant.Id, "改名", "")
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("普通成员更新应返回无权限业务拒绝: %v", err)
	}
	if ok {
		t.Fatalf("普通成员不应能更新租户")
	}
	if ok, err = svc.UpdateTenant(3, tenant.Id, "改名", ""); err != nil || !ok {
		t.Fatalf("管理员应能更新租户，实际 ok=%v err=%v", ok, err)
	}
}

func TestGetTenantRejectsNonMember(t *testing.T) {
	svc, _, _ := newTestTenantService()
	tenant, _ := svc.CreateTenant(1, "示例团队", "")
	if _, err := svc.GetTenant(3, tenant.Id); !errors.Is(err, ErrNoPermission) {
		t.Fatalf("非成员访问应返回无权限，实际 %v", err)
	}
}

func TestAddMemberByAccount(t *testing.T) {
	svc, _, _ := newTestTenantService()
	tenant, _ := svc.CreateTenant(1, "示例团队", "")

	ok, err := svc.AddMember(1, tenant.Id, "ghost", model.TenantRoleMember, 0)
	if err != nil || ok {
		t.Fatalf("不存在账号不应添加成功，实际 ok=%v err=%v", ok, err)
	}
	if ok, err = svc.AddMember(1, tenant.Id, "member", model.TenantRoleMember, 0); err != nil || !ok {
		t.Fatalf("owner 应能添加成员，实际 ok=%v err=%v", ok, err)
	}
	// 成员自身没有管理权限。
	if ok, _ = svc.AddMember(2, tenant.Id, "outsider", model.TenantRoleMember, 0); ok {
		t.Fatalf("普通成员不应能添加成员")
	}
	members, err := svc.ListMembers(1, tenant.Id)
	if err != nil || len(members) != 2 {
		t.Fatalf("成员列表应有 2 人，实际 (%v, %v)", members, err)
	}
}

func TestSetMemberRoleAndRemove(t *testing.T) {
	svc, _, _ := newTestTenantService()
	tenant, _ := svc.CreateTenant(1, "示例团队", "")
	svc.AddMember(1, tenant.Id, "member", model.TenantRoleMember, 0)

	if ok, _ := svc.SetMemberRole(2, tenant.Id, 3, model.TenantRoleMember); ok {
		t.Fatalf("普通成员不应能设置角色")
	}
	if ok, err := svc.SetMemberRole(1, tenant.Id, 2, model.TenantRoleAdmin); err != nil || !ok {
		t.Fatalf("owner 应能提升成员为管理员，实际 ok=%v err=%v", ok, err)
	}
	if ok, _ := svc.SetMemberRole(1, tenant.Id, 1, model.TenantRoleAdmin); ok {
		t.Fatalf("不应允许修改自己的角色")
	}
	if ok, err := svc.RemoveMember(1, tenant.Id, 2); err != nil || !ok {
		t.Fatalf("owner 应能移除成员，实际 ok=%v err=%v", ok, err)
	}
	if ok, _ := svc.RemoveMember(1, tenant.Id, 1); ok {
		t.Fatalf("不应允许移除自己")
	}
}

func TestDepartmentTreeOperations(t *testing.T) {
	svc, _, _ := newTestTenantService()
	tenant, _ := svc.CreateTenant(1, "示例团队", "")

	rootID, err := svc.CreateDepartment(1, tenant.Id, 0, "研发中心")
	if err != nil || rootID == 0 {
		t.Fatalf("创建根部门失败: id=%d err=%v", rootID, err)
	}
	childID, err := svc.CreateDepartment(1, tenant.Id, rootID, "后端组")
	if err != nil || childID == 0 {
		t.Fatalf("创建子部门失败: id=%d err=%v", childID, err)
	}
	if id, err := svc.CreateDepartment(1, tenant.Id, 999, "幽灵组"); err != nil || id != 0 {
		t.Fatalf("父部门不存在时不应创建成功")
	}
	if _, err = svc.CreateDepartment(2, tenant.Id, 0, "越权部门"); !errors.Is(err, ErrNoPermission) {
		t.Fatalf("非管理员建部门应返回无权限，实际 %v", err)
	}
	// 把父部门挂到自己的子部门下会成环，应被拒绝。
	if ok, err := svc.MoveDepartment(1, tenant.Id, rootID, childID); err != nil || ok {
		t.Fatalf("成环移动应被拒绝，实际 ok=%v err=%v", ok, err)
	}
	if ok, err := svc.MoveDepartment(1, tenant.Id, childID, 0); err != nil || !ok {
		t.Fatalf("合法移动应成功，实际 ok=%v err=%v", ok, err)
	}

	svc.AddMember(1, tenant.Id, "member", model.TenantRoleMember, 0)
	if ok, err := svc.SetMemberDepartment(1, tenant.Id, 2, rootID); err != nil || !ok {
		t.Fatalf("归档成员到部门应成功，实际 ok=%v err=%v", ok, err)
	}
	if ok, err := svc.RemoveDepartment(1, tenant.Id, rootID); err != nil || !ok {
		t.Fatalf("删除部门应成功，实际 ok=%v err=%v", ok, err)
	}
	if m, _ := svc.tenantDao.GetTenantMember(tenant.Id, 2); m.DeptID != 0 {
		t.Fatalf("删除部门后成员应回到未分配，实际 dept_id=%d", m.DeptID)
	}
}
