// Copyright 2016 Google, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package doorman

import (
	"net"
	"time"

	"github.com/notfresh/zxdoorman/proto"
	"golang.org/x/net/context"
	rpc "google.golang.org/grpc"
	goproto "google.golang.org/protobuf/proto"
)

type fixture struct {
	client    proto.CapacityClient
	server    *Server
	rpcServer *rpc.Server
	lis       net.Listener
}

func (fix fixture) tearDown() {
	if fix.rpcServer != nil {
		fix.rpcServer.Stop()
	}
	if fix.server != nil {
		fix.server.Close()
	}
	if fix.lis != nil {
		fix.lis.Close()
	}
}

func (fix fixture) Addr() string {
	return fix.lis.Addr().String()
}

// setUp sets up a test root server.
func setUp() (fixture, error) {
	return setUpIntermediate("test", "")
}

// setUpIntermediate sets up a test intermediate server.
func setUpIntermediate(name string, addr string) (fixture, error) {
	var (
		fix fixture
		err error
	)

	fix.server, err = MakeTestIntermediateServer(
		name, addr,
		&proto.ResourcePB{
			IdentifierGlob: *goproto.String("*"),
			Capacity:       *goproto.Int32(100),
			SafeCapacity:   *goproto.Int32(2),
			Algo: &proto.AlgorithmPB{
				Kind:            proto.AlgorithmPB_NO_ALGORITHM,
				RefreshInterval: *goproto.Int64(1),
				LeaseLength:     *goproto.Int64(2),
			},
		})
	if err != nil {
		return fixture{}, err
	}

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return fixture{}, err
	}

	fix.lis = lis

	fix.rpcServer = rpc.NewServer()

	proto.RegisterCapacityServer(fix.rpcServer, fix.server)

	go fix.rpcServer.Serve(lis)

	conn, err := rpc.Dial(fix.Addr(), rpc.WithInsecure())
	if err != nil {
		return fixture{}, err
	}

	fix.client = proto.NewCapacityClient(conn)
	return fix, nil
}

func makeRequest(fix fixture, wants, has int32) (*proto.GetCapacityResponse, error) {
	req := &proto.GetCapacityRequest{
		ClientId: *goproto.String("client"),
		Resource: []*proto.GetCapacityRequest_ResourceRequest{
			{
				ResourceId: *goproto.String("resource"),
				//Priority:   *goproto.Int64(1),
				Has: &proto.Lease{
					ExpiryTime:      *goproto.Int64(0),
					RefreshInterval: *goproto.Int64(0),
					Capacity:        *goproto.Int32(0),
				},
				Want: *goproto.Int32(wants),
			},
		},
	}

	if has > 0 {
		req.Resource[0].Has = &proto.Lease{
			ExpiryTime:      *goproto.Int64(time.Now().Add(1 * time.Minute).Unix()),
			RefreshInterval: *goproto.Int64(5),
			Capacity:        *goproto.Int32(has),
		}
	}

	return fix.client.GetCapacity(context.Background(), req)
}

type clientWants struct {
	wants      float64
	numClients int64
}
