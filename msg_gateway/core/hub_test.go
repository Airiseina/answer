package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testPushSecret = "0123456789abcdef0123456789abcdef01234567"

func withPushSecret(t *testing.T) {
	t.Helper()
	previous := pushSecret
	pushSecret = []byte(testPushSecret)
	t.Cleanup(func() { pushSecret = previous })
}

func TestPushSignatureRoundTrip(t *testing.T) {
	withPushSecret(t)
	body := []byte(`{"type":"chat","target_user_ids":[1,2]}`)

	req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(body)))
	signPushRequest(req, body)
	if !verifyPushSignature(req, body) {
		t.Fatal("签名后应能通过校验")
	}

	if verifyPushSignature(req, []byte(`{"type":"chat","target_user_ids":[9]}`)) {
		t.Fatal("请求体被篡改后必须校验失败")
	}

	missing := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(string(body)))
	if verifyPushSignature(missing, body) {
		t.Fatal("缺少签名头必须校验失败")
	}
}

// newTestManager 构造一个只含连接表的管理器，供多端用例直接操作映射。
func newTestManager() *Manager {
	return &Manager{Clients: make(map[int64][]*Client)}
}

// TestMultiDeviceRegisterKeepsAllDevices：同一用户的多个连接要同时在线，
// 且在线用户数按用户计数而不是按连接计数。
func TestMultiDeviceRegisterKeepsAllDevices(t *testing.T) {
	manager := newTestManager()
	first := &Client{UserId: 7}
	second := &Client{UserId: 7}

	manager.Lock.Lock()
	if evicted, onlineUsers := manager.attachDeviceLocked(first); evicted != nil || onlineUsers != 1 {
		manager.Lock.Unlock()
		t.Fatalf("首个设备不应被淘汰，在线用户数应为 1: evicted=%v online=%d", evicted != nil, onlineUsers)
	}
	if evicted, onlineUsers := manager.attachDeviceLocked(second); evicted != nil || onlineUsers != 1 {
		manager.Lock.Unlock()
		t.Fatalf("第二个设备不应被淘汰，在线用户数仍为 1: evicted=%v online=%d", evicted != nil, onlineUsers)
	}
	manager.Lock.Unlock()

	if devices := manager.localDevices(7); len(devices) != 2 {
		t.Fatalf("该用户应保有 2 个连接，实际 %d", len(devices))
	}
}

// TestMultiDeviceUnregisterKeepsOtherDevices：单个设备断开不能把该用户的其它设备一起下线。
func TestMultiDeviceUnregisterKeepsOtherDevices(t *testing.T) {
	manager := newTestManager()
	phone := &Client{UserId: 7}
	desktop := &Client{UserId: 7}

	manager.Lock.Lock()
	manager.attachDeviceLocked(phone)
	manager.attachDeviceLocked(desktop)
	if remaining, onlineUsers := manager.detachDeviceLocked(phone); remaining != 1 || onlineUsers != 1 {
		manager.Lock.Unlock()
		t.Fatalf("断开一个设备后应剩余 1 个连接: remaining=%d online=%d", remaining, onlineUsers)
	}
	if remaining, onlineUsers := manager.detachDeviceLocked(desktop); remaining != 0 || onlineUsers != 0 {
		manager.Lock.Unlock()
		t.Fatalf("全部断开后应移出在线表: remaining=%d online=%d", remaining, onlineUsers)
	}
	manager.Lock.Unlock()

	if devices := manager.localDevices(7); len(devices) != 0 {
		t.Fatalf("用户应已下线，实际仍有 %d 个连接", len(devices))
	}
}

// TestMultiDeviceEvictsOldestBeyondLimit：超出单用户连接上限时淘汰最早的连接，避免异常客户端堆积。
func TestMultiDeviceEvictsOldestBeyondLimit(t *testing.T) {
	manager := newTestManager()
	oldest := &Client{UserId: 7}

	manager.Lock.Lock()
	manager.attachDeviceLocked(oldest)
	for i := 1; i < maxDevicesPerUser; i++ {
		if evicted, _ := manager.attachDeviceLocked(&Client{UserId: 7}); evicted != nil {
			manager.Lock.Unlock()
			t.Fatalf("第 %d 个连接不应触发淘汰", i+1)
		}
	}
	evicted, _ := manager.attachDeviceLocked(&Client{UserId: 7})
	manager.Lock.Unlock()

	if evicted != oldest {
		t.Fatal("超出上限时应淘汰最早的连接")
	}
	if devices := manager.localDevices(7); len(devices) != maxDevicesPerUser {
		t.Fatalf("连接数应稳定在上限 %d，实际 %d", maxDevicesPerUser, len(devices))
	}
}

// TestLocalDevicesReturnsSnapshot：返回的必须是快照，调用方修改不影响在线表。
func TestLocalDevicesReturnsSnapshot(t *testing.T) {
	manager := newTestManager()
	manager.Lock.Lock()
	manager.attachDeviceLocked(&Client{UserId: 7})
	manager.Lock.Unlock()

	devices := manager.localDevices(7)
	devices[0] = nil
	if len(manager.localDevices(7)) != 1 || manager.localDevices(7)[0] == nil {
		t.Fatal("localDevices 必须返回连接快照，不能暴露内部切片")
	}
}
