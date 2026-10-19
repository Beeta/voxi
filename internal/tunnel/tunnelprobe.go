package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// publicResolver resolves via 8.8.8.8 to dodge the LAN resolver's negative
// cache of the not-yet-live tunnel hostname (trycloudflare NXDOMAIN TTL 60s).
var publicResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "udp", "8.8.8.8:53")
	},
}

// tunnelProbers returns probe clients: public DNS first (dodges the LAN
// negative cache), system DNS as fallback (public DNS is blocked on some LANs).
func tunnelProbers() []*http.Client {
	mk := func(r *net.Resolver) *http.Client {
		return &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 5 * time.Second, Resolver: r}).DialContext,
		}}
	}
	return []*http.Client{mk(publicResolver), mk(nil)}
}

// probeOnce fetches the first bytes to confirm the file is reachable.
func probeOnce(ctx context.Context, client *http.Client, fileURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", "bytes=0-1")
	req.Header.Set(ProbeHeader, "1")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		return nil
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

// WaitTunnelReady polls the public URL until the tunnel edge serves the file.
func WaitTunnelReady(ctx context.Context, fileURL string) error {
	return WaitTunnelReadyWatched(ctx, fileURL, nil)
}

// WaitTunnelReadyWatched is WaitTunnelReady calling onAttempt before each
// round of probes with its number, from 1, and how the last round failed
// ("" before the first). onAttempt may be nil.
func WaitTunnelReadyWatched(ctx context.Context, fileURL string, onAttempt func(n int, last string)) error {
	deadline := time.Now().Add(60 * time.Second)
	clients := tunnelProbers()
	var last string
	for n := 1; time.Now().Before(deadline); n++ {
		if onAttempt != nil {
			onAttempt(n, last)
		}
		for _, client := range clients {
			if err := probeOnce(ctx, client, fileURL); err == nil {
				return nil
			} else {
				last = err.Error()
			}
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("隧道地址在限定时间内不可达（最后状态：%s）", last)
}
