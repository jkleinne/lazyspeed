// Package metrics exports speed test results to InfluxDB via HTTP line
// protocol. The package is stateless and mirrors the shape of the notify/
// package: pure encoder, HTTP writer with retry, top-level Dispatch.
package metrics

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	"github.com/jkleinne/lazyspeed/model"
)

// fieldPosition indicates whether a field is the first in the field set.
type fieldPosition int

const (
	// fieldFirst means no comma prefix is needed.
	fieldFirst fieldPosition = iota
	// fieldSubsequent means a comma prefix is prepended.
	fieldSubsequent
)

// measurement is the fixed InfluxDB measurement name for every exported
// point. Not user-configurable to keep the schema predictable for
// dashboards. Future additional measurements would be separate functions.
const measurement = "lazyspeed_speedtest"

// escapeGrowHint is the extra capacity allocated when building an escaped
// tag value. Accounts for a small number of backslash-escape sequences.
const escapeGrowHint = 4

// EncodePoint serializes one speed test result as a single line of InfluxDB
// line protocol, terminated with a newline. The host parameter is the
// already-resolved host tag value; passing an empty string omits the host
// tag entirely (same treatment as any other empty tag value).
//
// Tag values that are empty on the result are omitted rather than emitted
// with empty values, since line protocol rejects empty tag values.
func EncodePoint(result *model.SpeedTestResult, host string) []byte {
	var buf bytes.Buffer
	buf.WriteString(measurement)

	writeTag(&buf, "host", host)
	writeTag(&buf, "server_name", result.ServerName)
	writeTag(&buf, "server_sponsor", result.ServerSponsor)
	writeTag(&buf, "country", result.Country)
	writeTag(&buf, "user_isp", result.UserISP)

	buf.WriteByte(' ')
	writeField(&buf, "download_mbps", result.DownloadSpeed, fieldFirst)
	writeField(&buf, "upload_mbps", result.UploadSpeed, fieldSubsequent)
	writeField(&buf, "ping_ms", result.Ping, fieldSubsequent)
	writeField(&buf, "jitter_ms", result.Jitter, fieldSubsequent)
	writeField(&buf, "distance_km", result.Distance, fieldSubsequent)

	buf.WriteByte(' ')
	buf.WriteString(strconv.FormatInt(result.Timestamp.UnixNano(), 10))
	buf.WriteByte('\n')

	return buf.Bytes()
}

// writeTag appends ",key=escapedValue" to buf. Empty values are skipped
// entirely, since line protocol rejects empty tag values.
func writeTag(buf *bytes.Buffer, key, value string) {
	if value == "" {
		return
	}
	buf.WriteByte(',')
	buf.WriteString(key)
	buf.WriteByte('=')
	buf.WriteString(escapeTagValue(value))
}

// writeField appends "key=value" (or ",key=value" if not first) to buf.
// Float values are formatted with 'f' and precision -1 to preserve all
// significant digits while avoiding scientific notation. Non-finite values
// (NaN, +Inf, -Inf) are substituted with 0 because InfluxDB line protocol
// only accepts numeric literals for float fields.
func writeField(buf *bytes.Buffer, key string, value float64, pos fieldPosition) {
	if pos == fieldSubsequent {
		buf.WriteByte(',')
	}
	buf.WriteString(key)
	buf.WriteByte('=')
	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	buf.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
}

// escapeTagValue escapes comma, equals, space, and backslash in a tag value
// per the InfluxDB line protocol spec, and substitutes carriage return or
// newline with a single space because those characters cannot be escaped in
// tag context and would otherwise split the point into multiple lines. This
// helper is called only on tag values in practice; field keys and the
// measurement name are hard-coded safe constants.
func escapeTagValue(s string) string {
	if !strings.ContainsAny(s, ",= \\\n\r") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + escapeGrowHint)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\n', '\r':
			sb.WriteByte(' ')
		case ',', '=', ' ', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
