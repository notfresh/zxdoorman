package doorman

import (
	"context"
	zx "github.com/notfresh/zxdoorman"
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
	congifg        *zx.ResourceRepository
	quit           chan bool
}

func (server *Server) WaitUntilConfigured() {
	<-server.isConfigured
}

func (server *Server) GetLearningModeEndTime(learningLength time.Duration) time.Time {
	if learningLength.Seconds() <= 0 {
		return time.Unix(0, 0)
	}
	return server.becameMasterAt.Add(learningLength)
}

func (server *Server) LoadConfig(ctx context.Context, config *zx.ResourceRepository) {

}
