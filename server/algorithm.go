package doorman

import (
	zx "github.com/notfresh/zxdoorman"
	"time"
)

type Request struct {
	ClientId string
	Has      int
	Want     int
}

type Algorithm func(store LeaseStore, capacity int, request *Request) Lease

func getAlgorithmParams(algo *zx.AlgorithmPB) (leaseLength, refreshInterval time.Duration) {
	return time.Duration(algo.GetLeaseLength()) * time.Second, time.Duration(algo.GetRefreshInterval())
}

// zx take a pb-defined algo and make a real function
func NoAlgorithm(algo *zx.AlgorithmPB) Algorithm {
	leaseLength, leaseInterval := getAlgorithmParams(algo)
	return func(store LeaseStore, capacity int, request *Request) Lease {
		return store.Assign(request.ClientId, leaseLength, leaseInterval, request.Has, request.Want)
	}
}

// zx in learning mode, it does assign much than it has
func Learn(algo *zx.AlgorithmPB) Algorithm {
	leaseLength, leaseInterval := getAlgorithmParams(algo)
	return func(store LeaseStore, capacity int, request *Request) Lease {
		return store.Assign(request.ClientId, leaseLength, leaseInterval, request.Has, request.Has)
	}
}

type algoMapperFunc func(pb *zx.AlgorithmPB) Algorithm

var algoMapper = map[zx.AlgorithmPB_Kind]algoMapperFunc{
	zx.AlgorithmPB_NO_ALGORITHM: NoAlgorithm,
}
