package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamReturnsResponseBodyForNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/logs/postgres" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "no running containers found for the specified app", http.StatusNotFound)
	}))
	defer srv.Close()

	client, err := New(srv.URL, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.Stream(context.Background(), "logs/postgres", func(data string) bool {
		t.Fatalf("handler called with %q", data)
		return false
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want error")
	}

	want := "stream returned status 404: no running containers found for the specified app"
	if err.Error() != want {
		t.Fatalf("Stream() error = %q, want %q", err.Error(), want)
	}
}

func TestStreamReturnsStatusForNonOKStatusWithoutBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	client, err := New(srv.URL, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.Stream(context.Background(), "logs/postgres", func(data string) bool {
		t.Fatalf("handler called with %q", data)
		return false
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want error")
	}

	want := "stream returned status 418"
	if err.Error() != want {
		t.Fatalf("Stream() error = %q, want %q", err.Error(), want)
	}
}
