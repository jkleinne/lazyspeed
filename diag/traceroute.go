package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// RealDiagBackend implements DiagBackend using real network calls.
type RealDiagBackend struct{}

// ResolveDNS resolves a hostname to an IP address and measures DNS lookup latency.
func (b *RealDiagBackend) ResolveDNS(ctx context.Context, host string) (string, time.Duration, error) {
	resolver := &net.Resolver{}
	start := time.Now()
	addrs, err := resolver.LookupHost(ctx, host)
	latency := time.Since(start)
	if err != nil {
		return "", 0, fmt.Errorf("failed to resolve DNS for %s: %v", host, err)
	}
	if len(addrs) == 0 {
		return "", 0, fmt.Errorf("failed to resolve DNS for %s: no addresses found", host)
	}
	return addrs[0], latency, nil
}

// Traceroute runs a traceroute to the target, trying ICMP first and falling back to UDP.
func (b *RealDiagBackend) Traceroute(ctx context.Context, target string, maxHops int) ([]Hop, string, error) {
	// Resolve target to IP if it's a hostname
	ip := net.ParseIP(target)
	if ip == nil {
		resolver := &net.Resolver{}
		addrs, err := resolver.LookupHost(ctx, target)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve target %s: %v", target, err)
		}
		if len(addrs) == 0 {
			return nil, "", fmt.Errorf("failed to resolve target %s: no addresses found", target)
		}
		ip = net.ParseIP(addrs[0])
	}

	destIP := ip.String()

	// Try ICMP first
	hops, err := icmpTraceroute(ctx, destIP, maxHops)
	if err == nil {
		return hops, MethodICMP, nil
	}

	// Fall back to UDP on permission error
	if isPermissionError(err) {
		hops, udpErr := udpTraceroute(ctx, destIP, maxHops)
		if udpErr != nil {
			return nil, "", fmt.Errorf("failed to run traceroute: %v", udpErr)
		}
		return hops, MethodUDP, nil
	}

	return nil, "", fmt.Errorf("failed to run traceroute: %v", err)
}

// isPermissionError checks if the error indicates a permission problem.
func isPermissionError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		return os.IsPermission(syscallErr.Err)
	}
	return false
}

const hopTimeout = 3 * time.Second

const (
	traceroutePayload  = "LAZYSPEED"
	reverseDNSTimeout  = 2 * time.Second
	tracerouteBasePort = 33434
	icmpIDMask         = 0xffff // ICMP Echo ID field is 16-bit
	maxPacketSize      = 1500
	icmpProtocol       = 1
)

// traceLoop runs a traceroute loop, calling hopFn for each TTL until the destination
// is reached, maxHops is exceeded, or the context is cancelled.
func traceLoop(ctx context.Context, destIP string, maxHops int, hopFn func(ttl int) Hop) []Hop {
	var hops []Hop
	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		hop := hopFn(ttl)
		hops = append(hops, hop)
		if hop.IP == destIP {
			break
		}
	}
	return hops
}

// readICMPResponse reads an ICMP response from conn, parses it, and populates the hop
// if the response type matches one of validTypes. Returns false if any step fails.
func readICMPResponse(conn *icmp.PacketConn, start time.Time, hop *Hop, validTypes ...icmp.Type) bool {
	buf := make([]byte, maxPacketSize)
	n, peer, err := conn.ReadFrom(buf)
	latency := time.Since(start)
	if err != nil {
		return false
	}

	rm, err := icmp.ParseMessage(icmpProtocol, buf[:n])
	if err != nil {
		return false
	}

	peerIP := ""
	if udpAddr, ok := peer.(*net.UDPAddr); ok {
		peerIP = udpAddr.IP.String()
	}

	for _, t := range validTypes {
		if rm.Type == t {
			hop.IP = peerIP
			hop.Host = reverseResolve(peerIP)
			hop.Latency = latency
			return true
		}
	}
	return false
}

// icmpTraceroute performs traceroute using ICMP echo requests via "udp4" network.
// Using "udp4" with icmp.ListenPacket works without root on macOS and on Linux
// with appropriate sysctl settings.
func icmpTraceroute(ctx context.Context, destIP string, maxHops int) ([]Hop, error) {
	conn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		return nil, fmt.Errorf("failed to open ICMP listener: %v", err)
	}
	defer func() { _ = conn.Close() }()

	return traceLoop(ctx, destIP, maxHops, func(ttl int) Hop {
		return traceHop(ctx, conn, destIP, ttl)
	}), nil
}

// setHopDeadline sets the read deadline on conn to the sooner of hopTimeout
// or the context deadline.
func setHopDeadline(ctx context.Context, conn *icmp.PacketConn) error {
	deadline := time.Now().Add(hopTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	return conn.SetReadDeadline(deadline)
}

// traceHop sends an ICMP echo request with the given TTL and waits for a response.
func traceHop(ctx context.Context, conn *icmp.PacketConn, destIP string, ttl int) Hop {
	hop := Hop{Number: ttl}

	if err := conn.IPv4PacketConn().SetTTL(ttl); err != nil {
		hop.Timeout = true
		return hop
	}

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & icmpIDMask,
			Seq:  ttl,
			Data: []byte(traceroutePayload),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		hop.Timeout = true
		return hop
	}

	dst := &net.UDPAddr{IP: net.ParseIP(destIP)}
	start := time.Now()

	if _, err := conn.WriteTo(msgBytes, dst); err != nil {
		hop.Timeout = true
		return hop
	}

	if err := setHopDeadline(ctx, conn); err != nil {
		hop.Timeout = true
		return hop
	}

	if !readICMPResponse(conn, start, &hop, ipv4.ICMPTypeEchoReply, ipv4.ICMPTypeTimeExceeded) {
		hop.Timeout = true
	}
	return hop
}

// udpTraceroute performs traceroute using UDP packets with increasing TTL,
// listening for ICMP TTL Exceeded responses.
func udpTraceroute(ctx context.Context, destIP string, maxHops int) ([]Hop, error) {
	icmpConn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		return nil, fmt.Errorf("failed to listen for ICMP: %v", err)
	}
	defer func() { _ = icmpConn.Close() }()

	return traceLoop(ctx, destIP, maxHops, func(ttl int) Hop {
		return udpTraceHop(ctx, icmpConn, destIP, ttl)
	}), nil
}

// udpTraceHop sends a UDP packet to tracerouteBasePort+ttl with a specific TTL
// and listens for an ICMP TTL Exceeded response.
func udpTraceHop(ctx context.Context, icmpConn *icmp.PacketConn, destIP string, ttl int) Hop {
	hop := Hop{Number: ttl}

	port := tracerouteBasePort + ttl
	udpConn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.ParseIP(destIP),
		Port: port,
	})
	if err != nil {
		hop.Timeout = true
		return hop
	}
	defer func() { _ = udpConn.Close() }()

	rawConn, err := udpConn.SyscallConn()
	if err != nil {
		hop.Timeout = true
		return hop
	}

	var setsockoptErr error
	err = rawConn.Control(func(fd uintptr) {
		setsockoptErr = setTTL(fd, ttl)
	})
	if err != nil {
		hop.Timeout = true
		return hop
	}
	if setsockoptErr != nil {
		hop.Timeout = true
		return hop
	}

	start := time.Now()

	// Send a UDP packet
	if _, err := udpConn.Write([]byte(traceroutePayload)); err != nil {
		hop.Timeout = true
		return hop
	}

	if err := setHopDeadline(ctx, icmpConn); err != nil {
		hop.Timeout = true
		return hop
	}

	if !readICMPResponse(icmpConn, start, &hop, ipv4.ICMPTypeTimeExceeded, ipv4.ICMPTypeDestinationUnreachable) {
		hop.Timeout = true
	}
	return hop
}

// reverseResolve does best-effort reverse DNS lookup for an IP address.
// Returns the IP string if reverse lookup fails.
func reverseResolve(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), reverseDNSTimeout)
	defer cancel()

	resolver := &net.Resolver{}
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ip
	}
	// Remove trailing dot from DNS name
	return strings.TrimSuffix(names[0], ".")
}
