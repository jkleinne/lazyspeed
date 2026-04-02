package model

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

// Server is a display-friendly representation of a speed test server,
// decoupled from the speedtest-go library type.
type Server struct {
	ID       string
	Name     string
	Sponsor  string
	Country  string
	Host     string
	Latency  time.Duration
	Distance float64
}

// ServerStore owns the speed test server list and provides fetch/lookup operations.
type ServerStore struct {
	servers speedtest.Servers
}

// Fetch retrieves the server list from the backend and sorts by latency.
func (s *ServerStore) Fetch(ctx context.Context, backend Backend) error {
	serverList, err := backend.FetchServers()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to fetch servers: %v", err)
	}
	slices.SortFunc(serverList, func(a, b *speedtest.Server) int {
		return cmp.Compare(a.Latency, b.Latency)
	})
	s.servers = serverList
	return nil
}

// List returns the server list as display-friendly Server values.
func (s *ServerStore) List() []Server {
	servers := make([]Server, len(s.servers))
	for i, srv := range s.servers {
		servers[i] = Server{
			ID:       srv.ID,
			Name:     srv.Name,
			Sponsor:  srv.Sponsor,
			Country:  srv.Country,
			Host:     srv.Host,
			Latency:  srv.Latency,
			Distance: srv.Distance,
		}
	}
	return servers
}

// FindIndex returns the index of the server with the given ID,
// or -1 and false if not found.
func (s *ServerStore) FindIndex(id string) (int, bool) {
	idx := slices.IndexFunc(s.servers, func(srv *speedtest.Server) bool {
		return srv.ID == id
	})
	return idx, idx >= 0
}

// Raw returns the underlying speedtest.Servers slice for callers that need
// the library type (e.g. passing a *speedtest.Server to test methods).
func (s *ServerStore) Raw() speedtest.Servers {
	return s.servers
}

// Len returns the number of servers.
func (s *ServerStore) Len() int {
	return len(s.servers)
}

// SetRaw replaces the underlying server list. Intended for test setup
// in external packages that cannot access the unexported servers field.
func (s *ServerStore) SetRaw(servers speedtest.Servers) {
	s.servers = servers
}
