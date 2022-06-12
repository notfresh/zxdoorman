package doorman

import (
	"context"
	"github.com/notfresh/zxdoorman/proto"
	goproto "google.golang.org/protobuf/proto"
	"log"
	"path/filepath"
	"sync"
	"time"
)

type Server struct {
	ServerId       string
	isConfigured   chan bool
	mu             sync.RWMutex
	resources      map[string]*Resource
	isMaster       bool
	becameMasterAt time.Time
	currentMaster  string
	config         *proto.ResourceRepository
	quit           chan bool
	proto.UnimplementedCapacityServer
}

//func (server *Server) MustEmbedUnimplementedCapacityServer() {
//	//TODO implement me
//	panic("implement me")
//}

func (server *Server) WaitUntilConfigured() {
	<-server.isConfigured
}

func (server *Server) Close() {
	server.quit <- true
}

func (server *Server) GetLearningModeEndTime(learningLength time.Duration) time.Time {
	if learningLength.Seconds() <= 0 {
		return time.Unix(0, 0)
	}
	return server.becameMasterAt.Add(learningLength)
}

// the server was created.
func (server *Server) LoadConfig(ctx context.Context, config *proto.ResourceRepository, expiryTimes map[string]*time.Time) error {
	//if err := validateResourceRepository(config); err != nil {
	//	return err
	//}
	// zx ? take part in election? How?
	server.mu.Lock()
	defer server.mu.Unlock()

	firstTime := server.config == nil

	// Stores the new configuration in the server object.
	server.config = config // zx set the config

	// If this is the first load of a config there are no resources
	// in the server map, so no need to process those, but we do need
	// to let people who were waiting on the server configuration
	// known: for this purpose we close isConfigured channel.
	// Also since we are now a configured server we can
	// start participating in the election process.
	if firstTime { // zx ? when will this be called second time?
		close(server.isConfigured)
		//return server.triggerElection(ctx) // zx elect
	}

	// Goes through the server's map of resources, loads a new
	// configuration and updates expiration time for each of them.
	for id, resource := range server.resources { // zx lazy create
		resource.LoadConfig(server.findConfigForResource(id), expiryTimes[id])
	}

	return nil
}

func NewServer(ctx context.Context, id string) (*Server, error) {
	server := &Server{
		ServerId:       id,
		isConfigured:   make(chan bool),
		resources:      make(map[string]*Resource),
		becameMasterAt: time.Now(),
		quit:           make(chan bool),
	}

	go server.run()

	return server, nil
}

// run is the server's main loop. It takes care of requesting new resources,
// and managing ones already claimed. This is the only method that should be
// performing RPC.

var defaultInterval = time.Duration(1 * time.Second)

func (server *Server) run() {
	for { // zx
		select {
		case <-server.quit: // zx wait to check if closed, quit gracefully
			// The server is closed, nothing to do here.
			return
		}
	}
}

func (server *Server) findConfigForResource(id string) *proto.ResourcePB {
	// Try to match it literally.
	for _, tpl := range server.config.Resources {
		if tpl.GetIdentifierGlob() == id {
			return tpl
		}
	}
	for _, tpl := range server.config.Resources {
		glob := tpl.GetIdentifierGlob()
		matched, err := filepath.Match(glob, id)

		if err != nil {
			log.Println("Error trying to match %v to %v", id, glob)
			continue
		} else if matched {
			return tpl
		}
	}
	return nil
}

// item is the mapping between the client id and the lease that algorithm assigned to the client with this id.
type item struct {
	id    string
	lease Lease
}

type clientRequest struct {
	client string
	resID  string
	has    int32
	want   int32
}

// GetCapacity assigns capacity leases to clients. It is part of the
// doorman.CapacityServer implementation.
// zx the core
func (server *Server) GetCapacity(ctx context.Context, in *proto.GetCapacityRequest) (out *proto.GetCapacityResponse, err error) {
	out = new(proto.GetCapacityResponse)
	client := in.GetClientId()
	// We will create a new goroutine for every resource in the
	// request. This is the channel that the leases come back on.
	itemsC := make(chan item, len(in.Resource))

	var requests []clientRequest

	for _, req := range in.Resource {
		request := clientRequest{
			client: client,
			resID:  req.GetResourceId(),
			has:    req.GetHas().GetCapacity(),
			want:   req.GetWant(),
		}
		requests = append(requests, request)
	}
	server.getCapacity(requests, itemsC)
	// We collect the assigned leases.
	for range in.Resource {
		item := <-itemsC
		resp := &proto.GetCapacityResponse_ResourceResponse{
			ResourceId: *goproto.String(item.id),
			Gets: &proto.Lease{
				RefreshInterval: *goproto.Int64(int64(item.lease.RefreshInterval.Seconds())),
				ExpiryTime:      *goproto.Int64(item.lease.ExpireTime.Unix()),
				Capacity:        *goproto.Int32(item.lease.Has),
			},
		}
		server.getOrCreateResource(item.id).SetSafeCapacity(resp)
		out.Response = append(out.Response, resp)
	}

	return out, nil
}

func (server *Server) getCapacity(crequests []clientRequest, itemsC chan item) {
	for _, creq := range crequests {
		res := server.getOrCreateResource(creq.resID)
		req := Request{
			ClientId: creq.client,
			Has:      creq.has,
			Want:     creq.want,
		}

		go func(req Request) {
			itemsC <- item{
				id:    res.resourceId,
				lease: res.Decide(&req),
			}
		}(req)
	}
}

// getResource takes a resource identifier and returns the matching
// resource (which will be created if necessary).
func (server *Server) getOrCreateResource(id string) *Resource {
	server.mu.Lock()
	defer server.mu.Unlock()

	// Resource already exists in the server state; return it.
	if res, ok := server.resources[id]; ok {
		return res
	}

	resource := server.newResource(id, server.findConfigForResource(id))
	server.resources[id] = resource
	return resource
}

// newResource returns a new resource named id and configured using
// cfg.
func (server *Server) newResource(id string, cfg *proto.ResourcePB) *Resource {
	res := &Resource{
		resourceId: id,
		store:      NewLeaseStore(id),
	}
	res.LoadConfig(cfg, nil) // zx load expireTime has no usage.

	// Calculates the learning mode end time. If one was not specified in the
	// algorithm the learning mode duration equals the lease length, because
	// that is the maximum time after which we can assume clients to have either
	// reported in or lost their lease.
	algo := res.config.GetAlgo()

	var learningModeDuration time.Duration

	if algo.GetLearningModeLength() != 0 {
		learningModeDuration = time.Duration(algo.GetLearningModeLength()) * time.Second
	} else {
		learningModeDuration = time.Duration(algo.GetLeaseLength()) * time.Second
	}
	res.learningEndAt = server.GetLearningModeEndTime(learningModeDuration)
	return res
}
