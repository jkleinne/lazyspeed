package model

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

func TestServerStore_FetchSortsByLatency(t *testing.T) {
	backend := &mockBackend{
		fetchServersFn: func() (speedtest.Servers, error) {
			return speedtest.Servers{
				&speedtest.Server{Name: "Slow", Latency: 50 * time.Millisecond},
				&speedtest.Server{Name: "Fast", Latency: 5 * time.Millisecond},
				&speedtest.Server{Name: "Medium", Latency: 20 * time.Millisecond},
			}, nil
		},
	}

	s := &ServerStore{}
	err := s.Fetch(context.Background(), backend)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if s.Len() != 3 {
		t.Fatalf("Expected 3 servers, got %d", s.Len())
	}

	raw := s.Raw()
	if raw[0].Name != "Fast" || raw[1].Name != "Medium" || raw[2].Name != "Slow" {
		t.Errorf("Expected servers sorted by latency (Fast, Medium, Slow), got (%s, %s, %s)",
			raw[0].Name, raw[1].Name, raw[2].Name)
	}
}

func TestServerStore_FetchBackendError(t *testing.T) {
	backend := &mockBackend{
		fetchServersFn: func() (speedtest.Servers, error) {
			return nil, errors.New("connection refused")
		},
	}

	s := &ServerStore{}
	err := s.Fetch(context.Background(), backend)
	if err == nil || err.Error() != "failed to fetch servers: connection refused" {
		t.Errorf("Expected wrapped error, got %v", err)
	}
}

func TestServerStore_FetchCancelledContext(t *testing.T) {
	backend := &mockBackend{
		fetchServersFn: func() (speedtest.Servers, error) {
			return nil, errors.New("timeout")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &ServerStore{}
	err := s.Fetch(ctx, backend)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestServerStore_FindIndex(t *testing.T) {
	s := &ServerStore{
		servers: speedtest.Servers{
			&speedtest.Server{ID: "10"},
			&speedtest.Server{ID: "20"},
			&speedtest.Server{ID: "30"},
		},
	}

	t.Run("Found", func(t *testing.T) {
		idx, found := s.FindIndex("20")
		if !found || idx != 1 {
			t.Errorf("FindIndex(20) = (%d, %v), want (1, true)", idx, found)
		}
	})

	t.Run("Not found", func(t *testing.T) {
		idx, found := s.FindIndex("99")
		if found || idx != -1 {
			t.Errorf("FindIndex(99) = (%d, %v), want (-1, false)", idx, found)
		}
	})
}

func TestServerStore_List(t *testing.T) {
	s := &ServerStore{
		servers: speedtest.Servers{
			&speedtest.Server{
				ID: "1", Name: "Alpha", Sponsor: "Sponsor A",
				Country: "US", Host: "a.example.com:8080",
				Latency: 10 * time.Millisecond, Distance: 100.5,
			},
			&speedtest.Server{
				ID: "2", Name: "Beta", Sponsor: "Sponsor B",
				Country: "DE", Host: "b.example.com",
				Latency: 25 * time.Millisecond, Distance: 500.0,
			},
		},
	}

	servers := s.List()
	if len(servers) != 2 {
		t.Fatalf("Expected 2 servers, got %d", len(servers))
	}

	first := servers[0]
	if first.ID != "1" || first.Name != "Alpha" || first.Sponsor != "Sponsor A" {
		t.Errorf("server[0] identity = (%q, %q, %q), want (1, Alpha, Sponsor A)",
			first.ID, first.Name, first.Sponsor)
	}
	if first.Country != "US" || first.Host != "a.example.com:8080" {
		t.Errorf("server[0] location = (%q, %q), want (US, a.example.com:8080)",
			first.Country, first.Host)
	}
	if first.Latency != 10*time.Millisecond || first.Distance != 100.5 {
		t.Errorf("server[0] metrics = (%v, %v), want (10ms, 100.5)",
			first.Latency, first.Distance)
	}
}

func TestServerStore_ListEmpty(t *testing.T) {
	s := &ServerStore{}
	servers := s.List()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(servers))
	}
}

func TestServerStore_LenAndRaw(t *testing.T) {
	s := &ServerStore{}

	if s.Len() != 0 {
		t.Errorf("Expected Len() = 0, got %d", s.Len())
	}
	if s.Raw() != nil {
		t.Errorf("Expected Raw() = nil, got %v", s.Raw())
	}

	input := speedtest.Servers{
		&speedtest.Server{ID: "1"},
		&speedtest.Server{ID: "2"},
	}
	s.SetRaw(input)

	if s.Len() != 2 {
		t.Errorf("Expected Len() = 2, got %d", s.Len())
	}
	raw := s.Raw()
	if len(raw) != 2 || raw[0].ID != "1" || raw[1].ID != "2" {
		t.Errorf("Raw() did not return expected servers")
	}
}
