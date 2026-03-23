package diag

import (
	"context"
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
		addrs, err := net.LookupHost(target)
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
	if os.IsPermission(err) || isPermissionError(err) {
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
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range []string{"permission denied", "operation not permitted", "EPERM", "EACCES"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

const hopTimeout = 3 * time.Second

// icmpTraceroute performs traceroute using ICMP echo requests via "udp4" network.
// Using "udp4" with icmp.ListenPacket works without root on macOS and on Linux
// with appropriate sysctl settings.
func icmpTraceroute(ctx context.Context, destIP string, maxHops int) ([]Hop, error) {
	conn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	var hops []Hop
	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		hop := traceHop(ctx, conn, destIP, ttl)
		hops = append(hops, hop)
		if hop.IP == destIP {
			break
		}
	}
	return hops, nil
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
			ID:   os.Getpid() & 0xffff,
			Seq:  ttl,
			Data: []byte("LAZYSPEED"),
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

	// Calculate deadline from context or hopTimeout, whichever is sooner
	deadline := time.Now().Add(hopTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		hop.Timeout = true
		return hop
	}

	buf := make([]byte, 1500)
	n, peer, err := conn.ReadFrom(buf)
	latency := time.Since(start)
	if err != nil {
		hop.Timeout = true
		return hop
	}

	rm, err := icmp.ParseMessage(1, buf[:n]) // protocol 1 = ICMP
	if err != nil {
		hop.Timeout = true
		return hop
	}

	peerIP := ""
	if udpAddr, ok := peer.(*net.UDPAddr); ok {
		peerIP = udpAddr.IP.String()
	}

	switch rm.Type {
	case ipv4.ICMPTypeEchoReply, ipv4.ICMPTypeTimeExceeded:
		hop.IP = peerIP
		hop.Host = reverseResolve(peerIP)
		hop.Latency = latency
	default:
		hop.Timeout = true
	}

	return hop
}

// udpTraceroute performs traceroute using UDP packets with increasing TTL,
// listening for ICMP TTL Exceeded responses.
func udpTraceroute(ctx context.Context, destIP string, maxHops int) ([]Hop, error) {
	// Listen for ICMP responses
	icmpConn, err := icmp.ListenPacket("udp4", "")
	if err != nil {
		return nil, fmt.Errorf("failed to listen for ICMP: %v", err)
	}
	defer func() { _ = icmpConn.Close() }()

	var hops []Hop
	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		hop := udpTraceHop(ctx, icmpConn, destIP, ttl)
		hops = append(hops, hop)
		if hop.IP == destIP {
			break
		}
	}
	return hops, nil
}

// udpTraceHop sends a UDP packet to port 33434+ttl with a specific TTL
// and listens for an ICMP TTL Exceeded response.
func udpTraceHop(ctx context.Context, icmpConn *icmp.PacketConn, destIP string, ttl int) Hop {
	hop := Hop{Number: ttl}

	port := 33434 + ttl
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
	if _, err := udpConn.Write([]byte("LAZYSPEED")); err != nil {
		hop.Timeout = true
		return hop
	}

	// Calculate deadline from context or hopTimeout, whichever is sooner
	deadline := time.Now().Add(hopTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := icmpConn.SetReadDeadline(deadline); err != nil {
		hop.Timeout = true
		return hop
	}

	buf := make([]byte, 1500)
	n, peer, err := icmpConn.ReadFrom(buf)
	latency := time.Since(start)
	if err != nil {
		hop.Timeout = true
		return hop
	}

	rm, err := icmp.ParseMessage(1, buf[:n])
	if err != nil {
		hop.Timeout = true
		return hop
	}

	peerIP := ""
	if udpAddr, ok := peer.(*net.UDPAddr); ok {
		peerIP = udpAddr.IP.String()
	}

	switch rm.Type {
	case ipv4.ICMPTypeTimeExceeded, ipv4.ICMPTypeDestinationUnreachable:
		hop.IP = peerIP
		hop.Host = reverseResolve(peerIP)
		hop.Latency = latency
	default:
		hop.Timeout = true
	}

	return hop
}

// reverseResolve does best-effort reverse DNS lookup for an IP address.
// Returns the IP string if reverse lookup fails.
func reverseResolve(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolver := &net.Resolver{}
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ip
	}
	// Remove trailing dot from DNS name
	return strings.TrimSuffix(names[0], ".")
}
