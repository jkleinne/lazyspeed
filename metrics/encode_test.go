package metrics

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

func TestEncodePoint_HappyPath(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 123.4,
		UploadSpeed:   45.6,
		Ping:          12.3,
		Jitter:        1.2,
		Distance:      50.0,
		ServerName:    "Hurricane Electric",
		ServerSponsor: "HE.net",
		Country:       "US",
		UserISP:       "Comcast",
		Timestamp:     time.Unix(1712761200, 0).UTC(),
	}

	out := string(EncodePoint(result, "myhost"))

	if !strings.HasPrefix(out, "lazyspeed_speedtest,") {
		t.Errorf("missing measurement prefix: %q", out)
	}

	wantTags := []string{
		"host=myhost",
		"server_name=Hurricane\\ Electric",
		"server_sponsor=HE.net",
		"country=US",
		"user_isp=Comcast",
	}
	for _, substr := range wantTags {
		if !strings.Contains(out, substr) {
			t.Errorf("missing tag %q in output %q", substr, out)
		}
	}

	wantFields := []string{
		"download_mbps=123.4",
		"upload_mbps=45.6",
		"ping_ms=12.3",
		"jitter_ms=1.2",
		"distance_km=50",
	}
	for _, substr := range wantFields {
		if !strings.Contains(out, substr) {
			t.Errorf("missing field %q in output %q", substr, out)
		}
	}

	wantTs := "1712761200000000000"
	if !strings.Contains(out, wantTs) {
		t.Errorf("missing nanosecond timestamp %q in output %q", wantTs, out)
	}

	if !strings.HasSuffix(out, "\n") {
		t.Errorf("missing trailing newline: %q", out)
	}
}

func TestEncodePoint_OmitEmptyTags(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 100,
		ServerName:    "server",
		// Country, UserISP, ServerSponsor intentionally empty
		Timestamp: time.Unix(1, 0),
	}

	out := string(EncodePoint(result, "host"))

	forbidden := []string{"country=", "user_isp=", "server_sponsor="}
	for _, s := range forbidden {
		if strings.Contains(out, s) {
			t.Errorf("empty tag %q leaked into output: %q", s, out)
		}
	}
}

func TestEncodePoint_EmptyHost(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 100,
		Timestamp:     time.Unix(1, 0),
	}
	out := string(EncodePoint(result, ""))
	if strings.Contains(out, "host=") {
		t.Errorf("empty host leaked into output: %q", out)
	}
}

func TestEncodePoint_CustomHost(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 100,
		Timestamp:     time.Unix(1, 0),
	}
	out := string(EncodePoint(result, "node-42"))
	if !strings.Contains(out, "host=node-42") {
		t.Errorf("expected host=node-42, got %q", out)
	}
}

func TestEscapeTagValue(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"simple", "simple"},
		{"with space", "with\\ space"},
		{"with,comma", "with\\,comma"},
		{"with=equals", "with\\=equals"},
		{"all three, =x", "all\\ three\\,\\ \\=x"},
		{"", ""},
		// Backslash escaping: input "with\backslash" → output "with\\backslash"
		{"with\\backslash", "with\\\\backslash"},
		// Trailing backslash: input "value\" → output "value\\"
		{"value\\", "value\\\\"},
		// Newline becomes space: input "line1\nline2" → output "line1 line2"
		{"line1\nline2", "line1 line2"},
		// CR becomes space: input "line1\rline2" → output "line1 line2"
		{"line1\rline2", "line1 line2"},
		// CRLF becomes two spaces: input "a\r\nb" → output "a  b"
		{"a\r\nb", "a  b"},
		// Mixed: input "a b\c,d=e" → output "a\ b\\c\,d\=e"
		{"a b\\c,d=e", "a\\ b\\\\c\\,d\\=e"},
	}
	for _, tt := range tests {
		got := escapeTagValue(tt.in)
		if got != tt.want {
			t.Errorf("escapeTagValue(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEncodePoint_SanitizesControlChars(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 100,
		ServerName:    "line1\nline2",
		Timestamp:     time.Unix(1, 0),
	}
	out := EncodePoint(result, "host")

	// Count newlines: only the trailing one should exist.
	nlCount := strings.Count(string(out), "\n")
	if nlCount != 1 {
		t.Errorf("expected exactly 1 newline (trailing), got %d in %q", nlCount, out)
	}
	// Trailing newline must be present.
	if !strings.HasSuffix(string(out), "\n") {
		t.Errorf("missing trailing newline: %q", out)
	}
	// The control char must have been substituted — verify no literal \n in the tag section.
	tagSection := strings.TrimSuffix(string(out), "\n")
	if strings.Contains(tagSection, "\n") {
		t.Errorf("control char leaked into tag section: %q", out)
	}
}

func TestEncodePoint_SubstitutesNonFiniteFloats(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: math.NaN(),
		UploadSpeed:   math.Inf(1),
		Ping:          math.Inf(-1),
		Jitter:        5.0,
		Distance:      10.0,
		Timestamp:     time.Unix(1, 0),
	}
	out := string(EncodePoint(result, "host"))

	// None of the non-finite literals should appear.
	for _, bad := range []string{"NaN", "+Inf", "-Inf", "Inf"} {
		if strings.Contains(out, bad) {
			t.Errorf("non-finite literal %q leaked into output: %q", bad, out)
		}
	}
	// The substituted zeros should appear for download, upload, ping.
	for _, want := range []string{"download_mbps=0", "upload_mbps=0", "ping_ms=0"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got %q", want, out)
		}
	}
	// Finite fields should still render normally.
	if !strings.Contains(out, "jitter_ms=5") {
		t.Errorf("expected jitter_ms=5 in output, got %q", out)
	}
}

func TestEncodePoint_FloatFormattingNoScientific(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 0.000001,
		UploadSpeed:   1234567890.0,
		Timestamp:     time.Unix(1, 0),
	}
	out := string(EncodePoint(result, "h"))
	if strings.Contains(out, "e+") || strings.Contains(out, "e-") {
		t.Errorf("scientific notation in output: %q", out)
	}
}

func TestEncodePoint_RoundTrip(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 100.5,
		UploadSpeed:   50.25,
		Ping:          10.0,
		Jitter:        0.5,
		Distance:      42.0,
		// No spaces in tag values so SplitN(out, " ", 3) produces exactly
		// three sections: measurement+tags, fields, timestamp.
		ServerName: "TestServer",
		Country:    "CA",
		Timestamp:  time.Unix(1712761200, 123456789),
	}
	out := EncodePoint(result, "host1")

	parts := bytes.SplitN(out, []byte(" "), 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 space-separated sections, got %d: %q", len(parts), out)
	}
	if !bytes.HasPrefix(parts[0], []byte("lazyspeed_speedtest,")) {
		t.Errorf("measurement+tags section malformed: %q", parts[0])
	}
	tsSection := bytes.TrimSuffix(parts[2], []byte("\n"))
	if string(tsSection) != "1712761200123456789" {
		t.Errorf("timestamp section: got %q, want %q", tsSection, "1712761200123456789")
	}
}
