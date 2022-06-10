package doorman

import (
	zx "github.com/notfresh/zxdoorman"
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
	config        *zx.ResourcePB
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

func (res *Resource) LoadConfig(cfg *zx.ResourcePB) {
	res.mu.Lock()
	defer res.mu.Unlock()
	res.config = cfg
	res.expiryTime = time.Now() // zx TODO
	algo := cfg.GetAlgo()
	res.algo = algoMapper[algo.GetKind()](algo)
	res.learnerAlgo = Learn(algo)

}

func NewResource(resourceId string, cfg *zx.ResourcePB) *Resource {
	res := &Resource{
		resourceId: resourceId,
		store:      NewLeaseStore(resourceId),
	}
	res.LoadConfig(cfg)
	//algoPB := cfg.GetAlgo()
	//var learningLength time.Duration
	//algoPBDura := algoPB.GetLearningModeLength()
	//if algoPBDura != 0 {
	//	learningLength = time.Second * time.Duration(algoPBDura)
	//} else {
	//	learningLength = time.Second * time.Duration(algoPB.GetLeaseLength())
	//}
	//TODO
	return nil
}
