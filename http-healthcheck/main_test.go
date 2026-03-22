package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestParseExpect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    map[int]struct{}
		wantErr bool
	}{
		{
			name:  "single code",
			input: "200",
			want:  map[int]struct{}{200: {}},
		},
		{
			name:  "comma separated with spaces",
			input: "200, 204 , 301",
			want:  map[int]struct{}{200: {}, 204: {}, 301: {}},
		},
		{
			name:  "trailing comma ignored empty parts",
			input: "404,",
			want:  map[int]struct{}{404: {}},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only commas and spaces",
			input:   " , , ",
			wantErr: true,
		},
		{
			name:    "not a number",
			input:   "ok",
			wantErr: true,
		},
		{
			name:    "below HTTP status range",
			input:   "99",
			wantErr: true,
		},
		{
			name:    "above HTTP status range",
			input:   "600",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseExpect(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseExpect: expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseExpect: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len got %d want %d", len(got), len(tt.want))
			}
			for code := range tt.want {
				if _, ok := got[code]; !ok {
					t.Errorf("missing code %d in %v", code, got)
				}
			}
		})
	}
}

func TestFixInsecureBoolShorthand(t *testing.T) {
	t.Parallel()
	in, a := fixInsecureBoolShorthand(true, []string{"false", "https://example.com"})
	if in {
		t.Fatal("expected insecure false")
	}
	if len(a) != 1 || a[0] != "https://example.com" {
		t.Fatalf("got %#v", a)
	}
	in, a = fixInsecureBoolShorthand(true, []string{"true", "https://example.com"})
	if !in {
		t.Fatal("expected insecure true")
	}
	if len(a) != 1 || a[0] != "https://example.com" {
		t.Fatalf("got %#v", a)
	}
	in, a = fixInsecureBoolShorthand(false, []string{"https://example.com"})
	if !in && len(a) != 1 {
		t.Fatalf("got %#v", a)
	}
	in, a = fixInsecureBoolShorthand(true, []string{"https://example.com"})
	if !in || len(a) != 1 {
		t.Fatalf("got insecure=%v args=%#v", in, a)
	}
}

func TestValidateHTTPURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "https", raw: "https://example.com/path", wantErr: false},
		{name: "http", raw: "http://localhost:8080/", wantErr: false},
		{name: "not a url", raw: "false", wantErr: true},
		{name: "no scheme", raw: "example.com", wantErr: true},
		{name: "ftp", raw: "ftp://example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateHTTPURL(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckURL(t *testing.T) {
	t.Parallel()

	ok200 := map[int]struct{}{200: {}}
	ok204 := map[int]struct{}{204: {}}

	t.Run("matches expected status", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		client := newHTTPClient(5*time.Second, false)
		code, err := checkURL(context.Background(), client, http.MethodGet, srv.URL, ok200)
		if err != nil {
			t.Fatal(err)
		}
		if code != http.StatusOK {
			t.Fatalf("code %d want %d", code, http.StatusOK)
		}
	})

	t.Run("wrong status returns error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
		t.Cleanup(srv.Close)

		client := newHTTPClient(5*time.Second, false)
		_, err := checkURL(context.Background(), client, http.MethodGet, srv.URL, ok200)
		if err == nil {
			t.Fatal("expected error for unexpected status")
		}
	})

	t.Run("expected set includes returned code", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(srv.Close)

		client := newHTTPClient(5*time.Second, false)
		code, err := checkURL(context.Background(), client, http.MethodGet, srv.URL, ok204)
		if err != nil {
			t.Fatal(err)
		}
		if code != http.StatusNoContent {
			t.Fatalf("code %d want %d", code, http.StatusNoContent)
		}
	})
}

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	c := newHTTPClient(7*time.Second, false)
	if c.Timeout != 7*time.Second {
		t.Fatalf("Timeout %v want %v", c.Timeout, 7*time.Second)
	}
	if c.Transport == nil {
		t.Fatal("Transport is nil")
	}

	insecure := newHTTPClient(3*time.Second, true)
	if insecure.Timeout != 3*time.Second {
		t.Fatalf("Timeout %v want %v", insecure.Timeout, 3*time.Second)
	}
	tr, ok := insecure.Transport.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("insecure client should set InsecureSkipVerify on transport")
	}
}

// TestCheckURL_RealNetwork hits public HTTPS endpoints. Requires network.
// Skipped when: go test -short, or HTTP_HEALTHCHECK_SKIP_NET=1.
func TestCheckURL_RealNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("skip real URL checks (run without -short)")
	}
	if os.Getenv("HTTP_HEALTHCHECK_SKIP_NET") == "1" {
		t.Skip("HTTP_HEALTHCHECK_SKIP_NET=1")
	}

	client := newHTTPClient(20*time.Second, false)
	ctx := context.Background()

	tests := []struct {
		name    string
		method  string
		url     string
		okCodes map[int]struct{}
		wantErr bool
	}{
		{
			name:    "example.com returns 200",
			method:  http.MethodGet,
			url:     "https://example.com/",
			okCodes: map[int]struct{}{200: {}},
		},
		{
			name:    "httpbin returns 204",
			method:  http.MethodGet,
			url:     "https://httpbin.org/status/204",
			okCodes: map[int]struct{}{204: {}},
		},
		{
			name:    "httpbin 404 when only 200 allowed",
			method:  http.MethodGet,
			url:     "https://httpbin.org/status/404",
			okCodes: map[int]struct{}{200: {}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, err := checkURL(ctx, client, tt.method, tt.url, tt.okCodes)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got status %d", code)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := tt.okCodes[code]; !ok {
				t.Fatalf("status %d not in expected set", code)
			}
		})
	}
}
