package tunnel

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DefaultCloudflaredPath is the Homebrew (Apple Silicon) install location, used
// as the default setting and the settings placeholder.
const DefaultCloudflaredPath = "/opt/homebrew/bin/cloudflared"

// ResolveCloudflared finds the cloudflared binary, trying the configured path
// first, then $PATH, then common install locations. A GUI .app launched from
// Finder/Dock does not inherit the shell's $PATH (no Homebrew dirs), so the
// configured absolute path (or these fallbacks) is what makes it work there.
func ResolveCloudflared(configured string) (string, error) {
	var candidates []string
	if configured = strings.TrimSpace(configured); configured != "" {
		candidates = append(candidates, configured)
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates,
		DefaultCloudflaredPath,       // Homebrew on Apple Silicon
		"/usr/local/bin/cloudflared", // Homebrew on Intel
		"/opt/local/bin/cloudflared", // MacPorts
	)
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("找不到 cloudflared，请在设置里填写正确路径或安装（brew install cloudflared）")
}

// NewUUID returns an RFC4122 v4 UUID, used for request ids and server tokens.
func NewUUID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:16]), nil
}

// PickFreePort asks the kernel for a free local port.
func PickFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// StartFileServer serves exactly one file over 127.0.0.1 at a random-token
// path, returning the port, the route and a shutdown func. The token keeps the
// public tunnel URL unguessable.
func StartFileServer(filePath string) (int, string, func(context.Context) error, error) {
	port, err := PickFreePort()
	if err != nil {
		return 0, "", nil, err
	}
	token, err := NewUUID()
	if err != nil {
		return 0, "", nil, err
	}
	route := "/" + token + "/" + filepath.Base(filePath)
	mux := http.NewServeMux()
	mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	})
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	return port, route, srv.Shutdown, nil
}

var (
	tunnelURLRe   = regexp.MustCompile(`https://[-a-z0-9]+\.trycloudflare\.com`)
	tunnelReadyRe = regexp.MustCompile(`Registered tunnel connection`)
)

// tunnelReadyGrace bounds the wait for cloudflared's "registered" log before
// handing off to reachability probing.
const tunnelReadyGrace = 8 * time.Second

// StartTunnel runs a cloudflared quick tunnel mapping localPort to a public
// URL and returns that base URL. The process dies when ctx is cancelled.
func StartTunnel(ctx context.Context, localPort int, cloudflared string) (string, error) {
	bin, err := ResolveCloudflared(cloudflared)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, "tunnel", "--url",
		fmt.Sprintf("http://localhost:%d", localPort))
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动 cloudflared 失败：%w", err)
	}
	urlCh := make(chan string, 1)
	readyCh := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		var registered bool
		for sc.Scan() {
			line := sc.Text()
			if m := tunnelURLRe.FindString(line); m != "" {
				select {
				case urlCh <- m:
				default:
				}
			}
			if !registered && tunnelReadyRe.MatchString(line) {
				registered = true
				close(readyCh)
			}
		}
	}()
	var base string
	select {
	case base = <-urlCh:
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("等待 cloudflared 隧道地址超时")
	case <-ctx.Done():
		return "", ctx.Err()
	}
	select {
	case <-readyCh:
	case <-time.After(tunnelReadyGrace):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return base, nil
}
