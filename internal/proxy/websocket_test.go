package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		wantResult bool
	}{
		{
			name: "valid websocket upgrade",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "upgrade",
			},
			wantResult: true,
		},
		{
			name: "valid websocket upgrade with keep-alive",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "keep-alive, upgrade",
			},
			wantResult: true,
		},
		{
			name: "case insensitive upgrade header",
			headers: map[string]string{
				"Upgrade":    "WebSocket",
				"Connection": "Upgrade",
			},
			wantResult: true,
		},
		{
			name: "case insensitive connection header",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "UPGRADE",
			},
			wantResult: true,
		},
		{
			name: "missing upgrade header",
			headers: map[string]string{
				"Connection": "upgrade",
			},
			wantResult: false,
		},
		{
			name: "missing connection header",
			headers: map[string]string{
				"Upgrade": "websocket",
			},
			wantResult: false,
		},
		{
			name: "wrong upgrade value",
			headers: map[string]string{
				"Upgrade":    "h2c",
				"Connection": "upgrade",
			},
			wantResult: false,
		},
		{
			name: "connection without upgrade",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "keep-alive",
			},
			wantResult: false,
		},
		{
			name:       "no headers",
			headers:    map[string]string{},
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "http://example.com/ws", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			got := isWebSocketUpgrade(req)
			if got != tt.wantResult {
				t.Errorf("isWebSocketUpgrade() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestShutdown_ForceClosesWebSocketTunnels(t *testing.T) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendListener.Close()

	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))
		// Keep the tunnel open until the proxy closes it.
		io.Copy(io.Discard, conn)
	}()

	backendHost, backendPort, err := net.SplitHostPort(backendListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	p := newTestProxy()
	route := &Route{
		Canonical: "example.com",
		Backends:  []Backend{{IP: backendHost, Port: backendPort}},
	}

	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.handleWebSocket(w, r, route, time.Now())
	}))
	defer front.Close()

	frontURL, err := url.Parse(front.URL)
	if err != nil {
		t.Fatal(err)
	}

	clientConn, err := net.Dial("tcp", frontURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	handshake := "GET / HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"\r\n"
	if _, err := clientConn.Write([]byte(handshake)); err != nil {
		t.Fatal(err)
	}

	// Wait for the 101 response so the tunnel is established and tracked.
	buf := make([]byte, 1)
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := clientConn.Read(buf); err != nil {
		t.Fatalf("did not receive handshake response: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- p.Shutdown(ctx) }()

	select {
	case err := <-shutdownDone:
		if err == nil || !strings.Contains(err.Error(), "websocket") {
			t.Errorf("Shutdown() error = %v, want websocket drain error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown() did not return after force-closing websocket tunnels")
	}

	// The tunnel must be closed now: draining the client connection should
	// end in EOF or a reset, not a read timeout.
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	rest := make([]byte, 256)
	for {
		if _, err := clientConn.Read(rest); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				t.Error("client connection still open after Shutdown()")
			}
			break
		}
	}
}
