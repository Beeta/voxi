package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStartFileServerServesAwkwardNames covers files whose names would not
// survive being pasted into a route. The route used to carry the file's own
// name, and Go 1.22's ServeMux reads a space in a pattern as the method
// separator: "2026-10-12 16_03_03.mp3" made HandleFunc panic, which took the
// whole app down, since transcription runs in a goroutine.
func TestStartFileServerServesAwkwardNames(t *testing.T) {
	names := []string{
		"2026-10-12 16_03_03.mp3", // spaces
		"会议 记录.m4a",                // spaces and non-ASCII
		"a{b}c.wav",               // ServeMux wildcard syntax
		"weird.",                  // empty extension
		"noext",                   // no extension
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte("audio"), 0o644); err != nil {
				t.Fatal(err)
			}
			port, route, shutdown, err := StartFileServer(path)
			if err != nil {
				t.Fatalf("StartFileServer: %v", err)
			}
			defer shutdown(context.Background())
			if port == 0 || route == "" {
				t.Fatalf("StartFileServer returned port %d, route %q", port, route)
			}

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, route))
			if err != nil {
				t.Fatalf("GET %s: %v", route, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", route, resp.StatusCode)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "audio" {
				t.Errorf("the served file holds %q, want the file's contents", body)
			}
		})
	}
}

// TestWatchedFileServerCountsFetchesNotProbes: the bytes the service fetches
// are reported, the reachability probes' are not.
func TestWatchedFileServerCountsFetchesNotProbes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.m4a")
	os.WriteFile(path, []byte("0123456789"), 0o644)
	var sent int64
	port, route, shutdown, err := StartWatchedFileServer(path, func(n int64) { sent += n })
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown(context.Background())
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, route)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set(ProbeHeader, "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if sent != 0 {
		t.Fatalf("a probe counted %d bytes", sent)
	}
	resp, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if sent != 10 {
		t.Errorf("the fetch counted %d bytes, want 10", sent)
	}
}
