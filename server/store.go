package doorman

import "time"

type Lease struct { // zx a store level
	Has, Want       int
	ExpireTime      time.Time
	RefreshInterval time.Duration
}

func (l *Lease) IsZero() bool {
	return l.ExpireTime.IsZero()
}

type LeaseStore interface {
	Get(clientId string) Lease
	Assign(clientId string, leaseLength, refreshInterval time.Duration, has, want int) Lease
	Release(clientId string)
	Clean()
	Count() int // zx the numbers of clients
	SumHas() int
	SumWant() int
}

type leaseStoreImp struct {
	ResourceId             string
	leases                 map[string]Lease
	sumHas, sumWant, count int
}

func NewLeaseStore(resourceId string) LeaseStore {
	return &leaseStoreImp{
		ResourceId: resourceId,
		leases:     make(map[string]Lease),
	}
	//return nil
}

func (store *leaseStoreImp) Get(clientId string) Lease {
	return store.leases[clientId]
}

func (store *leaseStoreImp) Assign(clientId string, leaseLength, refreshInterval time.Duration, has, want int) Lease {
	lease, ok := store.leases[clientId]
	store.sumHas += has - lease.Has
	store.sumWant += want - lease.Want
	if ok {
		store.count += 1 // TODO zx
	}
	lease.Has, lease.Want = has, want
	lease.ExpireTime = time.Now().Add(leaseLength)
	lease.RefreshInterval = refreshInterval
	store.leases[clientId] = lease
	return lease
}

func (store *leaseStoreImp) Release(clientId string) {
	lease, ok := store.leases[clientId]
	if !ok {
		return
	}
	store.sumHas -= lease.Has
	store.sumWant -= lease.Want
	delete(store.leases, clientId)
}

func (store *leaseStoreImp) Clean() {
	when := time.Now()
	for clientId, lease := range store.leases {
		if when.After(lease.ExpireTime) {
			store.Release(clientId)
		}
	}
}

func (store *leaseStoreImp) Count() int {
	return 0
}

func (store *leaseStoreImp) SumHas() int {
	return store.sumHas
}

func (store *leaseStoreImp) SumWant() int {
	return store.sumWant
}
