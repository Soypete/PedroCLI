package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     Config{Endpoint: "http://localhost:9000", Bucket: "test-bucket"},
			wantErr: false,
		},
		{
			name:    "missing endpoint",
			cfg:     Config{Bucket: "test-bucket"},
			wantErr: true,
		},
		{
			name:    "missing bucket",
			cfg:     Config{Endpoint: "http://localhost:9000"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if client == nil {
				t.Errorf("expected client, got nil")
			}
		})
	}
}

func TestUpload(t *testing.T) {
	var receivedBody string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint: server.URL,
		Bucket:   "test-bucket",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Upload(context.Background(), "test/key.txt", strings.NewReader("hello world"), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if receivedBody != "hello world" {
		t.Errorf("expected body 'hello world', got '%s'", receivedBody)
	}
	if receivedContentType != "text/plain" {
		t.Errorf("expected content type 'text/plain', got '%s'", receivedContentType)
	}
}

func TestDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("transcript content"))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint: server.URL,
		Bucket:   "test-bucket",
	})
	if err != nil {
		t.Fatal(err)
	}

	reader, err := client.Download(context.Background(), "episodes/123/transcript.txt")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "transcript content" {
		t.Errorf("expected 'transcript content', got '%s'", string(data))
	}
}

func TestDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint: server.URL,
		Bucket:   "test-bucket",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Delete(context.Background(), "test/key.txt")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		if strings.Contains(r.URL.Path, "exists") {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint: server.URL,
		Bucket:   "test-bucket",
	})
	if err != nil {
		t.Fatal(err)
	}

	exists, err := client.Exists(context.Background(), "exists/key.txt")
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		t.Error("expected exists=true")
	}

	exists, err = client.Exists(context.Background(), "missing/key.txt")
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}
	if exists {
		t.Error("expected exists=false")
	}
}

func TestSignRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "mykey" || pass != "mysecret" {
			t.Errorf("expected basic auth with mykey/mysecret, got %s/%s (ok=%v)", user, pass, ok)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint:  server.URL,
		Bucket:    "test-bucket",
		AccessKey: "mykey",
		SecretKey: "mysecret",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Upload(context.Background(), "test.txt", strings.NewReader("data"), "text/plain")
	if err != nil {
		t.Fatalf("upload with auth failed: %v", err)
	}
}
