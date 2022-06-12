package configuration

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
)

// zx define a source
type SourceFunc func(ctx context.Context) (data []byte, err error)

type pair struct {
	data []byte
	err  error
}

func LocalFile(path string) SourceFunc {
	updates := make(chan pair, 1)
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP)
	c <- syscall.SIGHUP
	go func() {
		for range c {
			data, err := ioutil.ReadFile(path)
			updates <- pair{data: data, err: err}
		}
	}()
	return func(ctx context.Context) (data []byte, err error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case p := <-updates:
			return p.data, p.err
		}
	}
}

func ParseSource(text string) (kind string, path string) {
	return "file", text
}
