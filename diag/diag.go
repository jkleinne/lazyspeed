package diag

import (
	"encoding/json"
	"time"
)

type Hop struct {
	Number  int           `json:"number"`
	IP      string        `json:"ip"`
	Host    string        `json:"host"`
	Latency time.Duration `json:"latency"`
	Timeout bool          `json:"timeout"`
}

func (h Hop) MarshalJSON() ([]byte, error) {
	type Alias Hop
	return json.Marshal(&struct {
		Alias
		Latency float64 `json:"latency"`
	}{
		Alias:   (Alias)(h),
		Latency: float64(h.Latency.Microseconds()) / 1000.0,
	})
}

func (h *Hop) UnmarshalJSON(data []byte) error {
	type Alias Hop
	aux := &struct {
		*Alias
		Latency float64 `json:"latency"`
	}{Alias: (*Alias)(h)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	h.Latency = time.Duration(aux.Latency * float64(time.Millisecond))
	return nil
}

type DNSResult struct {
	Host    string        `json:"host"`
	IP      string        `json:"resolved_ip"`
	Latency time.Duration `json:"latency"`
	Cached  bool          `json:"cached"`
}

func (d DNSResult) MarshalJSON() ([]byte, error) {
	type Alias DNSResult
	return json.Marshal(&struct {
		Alias
		Latency float64 `json:"latency"`
	}{
		Alias:   (Alias)(d),
		Latency: float64(d.Latency.Microseconds()) / 1000.0,
	})
}

func (d *DNSResult) UnmarshalJSON(data []byte) error {
	type Alias DNSResult
	aux := &struct {
		*Alias
		Latency float64 `json:"latency"`
	}{Alias: (*Alias)(d)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	d.Latency = time.Duration(aux.Latency * float64(time.Millisecond))
	return nil
}

type QualityScore struct {
	Score int    `json:"score"`
	Grade string `json:"grade"`
	Label string `json:"label"`
}

type DiagResult struct {
	Target    string       `json:"target"`
	Method    string       `json:"method"`
	Hops      []Hop        `json:"hops"`
	DNS       *DNSResult   `json:"dns"`
	Quality   QualityScore `json:"quality"`
	Timestamp time.Time    `json:"timestamp"`
}
