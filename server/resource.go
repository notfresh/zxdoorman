package doorman

import (
	goproto "github.com/golang/protobuf/proto"
	"github.com/notfresh/zxdoorman/proto"
	"sync"
	"time"
)

type Resource struct {
	resourceId    string
	mu            sync.RWMutex
	store         LeaseStore
	algo          Algorithm
	learnerAlgo   Algorithm
	learningEndAt time.Time
	config        *proto.ResourcePB
	expiryTime    time.Time
}

func (res *Resource) Capacity() int {
	// zx already expired
	if !res.expiryTime.IsZero() && res.expiryTime.Before(time.Now()) {
		return 0
	}
	return int(res.config.GetCapacity())
}

func (res *Resource) Release(clientId string) {
	res.mu.Lock()
	defer res.mu.Unlock()
	res.store.Release(clientId)
}

func (res *Resource) Decide(request *Request) Lease {
	res.mu.Lock()
	defer res.mu.Unlock()
	res.store.Clean()

	if res.learningEndAt.After(time.Now()) {
		return res.learnerAlgo(res.store, res.Capacity(), request)
	}
	return res.algo(res.store, res.Capacity(), request)
}

func (res *Resource) LoadConfig(cfg *proto.ResourcePB, expireTime *time.Time) {
	res.mu.Lock()
	defer res.mu.Unlock()
	res.config = cfg
	res.expiryTime = *expireTime
	algo := cfg.GetAlgo()
	res.algo = algoMapper[algo.GetKind()](algo)
	res.learnerAlgo = Learn(algo)

}

// SetSafeCapacity sets the safe capacity in a response.
func (res *Resource) SetSafeCapacity(resp *proto.GetCapacityResponse_ResourceResponse) {
	res.mu.RLock()
	defer res.mu.RUnlock()

	// If the resource configuration does not have a safe capacity
	// configured we return a dynamic safe capacity which equals
	// the capacity divided by the number of clients that we
	// know about.
	// needs to take sub clients into account (in a multi-server tree).
	if res.config.SafeCapacity == 0 {
		resp.SafeCapacity = *goproto.Int32(res.config.Capacity / int32(res.store.Count()))
	} else {
		resp.SafeCapacity = *goproto.Int32(res.config.SafeCapacity)
	}
}
