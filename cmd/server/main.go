package server

import (
	"context"
	"flag"
	"fmt"
	"github.com/notfresh/zxdoorman/configuration"
	"github.com/notfresh/zxdoorman/proto"
	doorman "github.com/notfresh/zxdoorman/server"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
	"log"
	"net"
	"os"
	"time"
)

var (
	port       = flag.Int("port", 0, "port to bind to")
	debugPort  = flag.Int("debug_port", 8081, "port to bind for HTTP debug info")
	serverRole = flag.String("server_role", "root", "Role of this server in the server tree")
	parent     = flag.String("parent", "", "Address of the parent server which this server connects to")
	hostname   = flag.String("hostname", "", "Use this as the hostname (if empty, use whatever the kernel reports")
	config     = flag.String("config", "", "source to load the config from (text protobufs)")

	rpcDialTimeout = flag.Duration("doorman_rpc_dial_timeout", 5*time.Second, "timeout to use for connecting to the doorman server")

	minimumRefreshInterval = flag.Duration("doorman_minimum_refresh_interval", 5*time.Second, "minimum refresh interval")

	tls      = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile = flag.String("cert_file", "", "The TLS cert file")
	keyFile  = flag.String("key_file", "", "The TLS key file")

	etcdEndpoints      = flag.String("etcd_endpoints", "", "comma separated list of etcd endpoints")
	masterDelay        = flag.Duration("master_delay", 10*time.Second, "delay in master elections")
	masterElectionLock = flag.String("master_election_lock", "", "etcd path for the master election or empty for no master election")
)

func getServerID(port int) string {
	if *hostname != "" {
		return fmt.Sprintf("%s:%d", *hostname, port)
	}
	hn, err := os.Hostname() // zx if there is no host name , take the os name

	if err != nil {
		hn = "unknown.localhost"
	}
	return fmt.Sprintf("%s:%d", hn, port)
}

func main() {
	if *config == "" {
		log.Fatalln("--config cannot be empty")
	}

	var cfg configuration.SourceFunc
	kind, path := configuration.ParseSource(*config)
	switch {
	case kind == "file":
		cfg = configuration.LocalFile(path)
	default:
		log.Fatalln("Fail to Parse config")
	}
	// zx:构建一个服务器实例
	dm, err := doorman.NewServer(context.Background(), getServerID(*port))
	if err != nil {
		log.Fatalf("doorman.NewIntermediate: %v\n", err)
	}

	rpcServer := grpc.NewServer() // zx what's this? the server is a business unrelated server
	proto.RegisterCapacityServer(rpcServer, dm)

	go func() {
		for {
			data, err := cfg(context.Background())
			if err != nil {
				log.Fatalln("Fail to Parse config", err)
			}
			resRepo := new(proto.ResourceRepository)
			if err = yaml.Unmarshal(data, resRepo); err != nil {
				log.Println("Fail to parse config", err)
				continue
			}

			// zx:表示doorman, 现在开始加载配置
			if err := dm.LoadConfig(context.Background(), resRepo, map[string]*time.Time{}); err != nil {
				log.Fatalf("cannot load config: %v\n", err)
			}
		}
	}()
	log.Println(fmt.Sprintf("Server listen on port %v", *debugPort))
	log.Println("Waiting for the server to be configured...")
	dm.WaitUntilConfigured()
	// Runs the server.
	log.Println("Server is configured, ready to go!")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalln(err)
	}
	if err := rpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
