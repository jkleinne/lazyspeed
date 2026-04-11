// Package metrics exports speed test results to InfluxDB via HTTP line
// protocol. The package is stateless and mirrors the shape of the notify/
// package: pure encoder, HTTP writer with retry, top-level Dispatch.
package metrics

import (
	"bytes"
	"strconv"

	"github.com/jkleinne/lazyspeed/model"
)

// measurement is the fixed InfluxDB measurement name for every exported
// point. Not user-configurable to keep the schema predictable for
// dashboards. Future additional measurements would be separate functions.
const measurement = "lazyspeed_speedtest"

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
	writeField(&buf, "download_mbps", result.DownloadSpeed, true)
	writeField(&buf, "upload_mbps", result.UploadSpeed, false)
	writeField(&buf, "ping_ms", result.Ping, false)
	writeField(&buf, "jitter_ms", result.Jitter, false)
	writeField(&buf, "distance_km", result.Distance, false)

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
// significant digits while avoiding scientific notation.
func writeField(buf *bytes.Buffer, key string, value float64, first bool) {
	if !first {
		buf.WriteByte(',')
	}
	buf.WriteString(key)
	buf.WriteByte('=')
	buf.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
}

// escapeTagValue escapes comma, equals, and space in a tag value per the
// InfluxDB line protocol spec. These are the only characters that require
// escaping in tag keys, tag values, and field keys.
func escapeTagValue(s string) string {
	needsEscape := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ',', '=', ' ':
			needsEscape = true
		}
		if needsEscape {
			break
		}
	}
	if !needsEscape {
		return s
	}
	var sb bytes.Buffer
	sb.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ',' || c == '=' || c == ' ' {
			sb.WriteByte('\\')
		}
		sb.WriteByte(c)
	}
	return sb.String()
}
