package snowflake

import (
	"fmt"
	"sync"
	"time"
)

const (
	epoch         = int64(1700000000000)
	workerBits    = uint(10)
	sequenceBits  = uint(12)
	maxWorkerID   = int64(-1) ^ (int64(-1) << workerBits)
	maxSequence   = int64(-1) ^ (int64(-1) << sequenceBits)
	workerShift   = sequenceBits
	timeShift     = sequenceBits + workerBits
	maxClockDrift = int64(500)
)

type Node struct {
	mu        sync.Mutex
	workerID  int64
	timestamp int64
	sequence  int64
}

func NewNode(workerID int64) *Node {
	if workerID < 0 || workerID > maxWorkerID {
		panic("worker ID out of range")
	}
	return &Node{workerID: workerID}
}

func (n *Node) Generate() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < n.timestamp {
		drift := n.timestamp - now
		if drift <= maxClockDrift {
			time.Sleep(time.Duration(drift) * time.Millisecond)
			now = time.Now().UnixMilli()
		}
		if now < n.timestamp {
			panic(fmt.Sprintf("时钟回拨超过%dms，拒绝生成ID", maxClockDrift))
		}
	}
	if now == n.timestamp {
		n.sequence = (n.sequence + 1) & maxSequence
		if n.sequence == 0 {
			for now <= n.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.sequence = 0
	}
	n.timestamp = now

	return ((now - epoch) << timeShift) | (n.workerID << workerShift) | n.sequence
}
