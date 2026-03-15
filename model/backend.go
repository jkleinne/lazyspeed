package model

import (
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

// Backend interface wraps the network I/O operations from speedtest-go
// to allow mocking in unit tests.
type Backend interface {
	FetchUserInfo() (*speedtest.User, error)
	FetchServers() (speedtest.Servers, error)
	PingTest(server *speedtest.Server, fn func(time.Duration)) error
	DownloadTest(server *speedtest.Server) error
	UploadTest(server *speedtest.Server) error
}

// realBackend is the default implementation that calls the actual speedtest-go library.
type realBackend struct{}

func (b *realBackend) FetchUserInfo() (*speedtest.User, error) {
	return speedtest.FetchUserInfo()
}

func (b *realBackend) FetchServers() (speedtest.Servers, error) {
	return speedtest.FetchServers()
}

func (b *realBackend) PingTest(server *speedtest.Server, fn func(time.Duration)) error {
	return server.PingTest(fn)
}

func (b *realBackend) DownloadTest(server *speedtest.Server) error {
	return server.DownloadTest()
}

func (b *realBackend) UploadTest(server *speedtest.Server) error {
	return server.UploadTest()
}
