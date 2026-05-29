package helpers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDownloads struct{ url string }

func (f fakeDownloads) GetOutputURL(context.Context, string) (string, error) {
	return f.url, nil
}

// withAllowedHosts temporarily overrides the package allowlist and restores it.
func withAllowedHosts(t *testing.T, hosts ...string) {
	t.Helper()
	orig := AllowedDownloadHosts
	AllowedDownloadHosts = hosts
	t.Cleanup(func() { AllowedDownloadHosts = orig })
}

func TestDownloadOutput_AllowlistedHostSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "video-bytes")
	}))
	defer srv.Close()

	// httptest binds 127.0.0.1; allow it for the duration of the test.
	withAllowedHosts(t, "127.0.0.1")

	var buf strings.Builder
	n, err := DownloadOutput(context.Background(), fakeDownloads{srv.URL}, "job1", &buf)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if buf.String() != "video-bytes" || n != int64(len("video-bytes")) {
		t.Fatalf("unexpected body %q (n=%d)", buf.String(), n)
	}
}

func TestDownloadOutput_NonAllowlistedHostRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "should-not-be-read")
	}))
	defer srv.Close()

	// Default allowlist (backblazeb2.com) does not include 127.0.0.1.
	var buf strings.Builder
	_, err := DownloadOutput(context.Background(), fakeDownloads{srv.URL}, "job1", &buf)
	if err == nil {
		t.Fatal("expected refusal for non-allowlisted host")
	}
	if !strings.Contains(err.Error(), "non-allowlisted host") {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("body should not have been fetched, got %q", buf.String())
	}
}

func TestDownloadOutput_RedirectToNonAllowlistedHostRefused(t *testing.T) {
	// The primary (allowlisted) host issues a redirect to a non-allowlisted
	// host. CheckRedirect runs before the hop is dialed, so the target is
	// never contacted.
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://attacker.invalid/leak", http.StatusFound)
	}))
	defer primary.Close()

	withAllowedHosts(t, "127.0.0.1") // primary only; attacker.invalid is not allowed

	var buf strings.Builder
	_, err := DownloadOutput(context.Background(), fakeDownloads{primary.URL}, "job1", &buf)
	if err == nil {
		t.Fatal("expected refusal for redirect to non-allowlisted host")
	}
	if !strings.Contains(err.Error(), "non-allowlisted host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsAllowedDownloadHost(t *testing.T) {
	withAllowedHosts(t, "backblazeb2.com")
	cases := map[string]bool{
		"f004.backblazeb2.com":      true,
		"backblazeb2.com":           true,
		"f004.backblazeb2.com:443":  true,
		"BACKBLAZEB2.COM":           true,
		"evil-backblazeb2.com":      false,
		"backblazeb2.com.evil.test": false,
		"attacker.invalid":          false,
		"":                          false,
	}
	for host, want := range cases {
		if got := isAllowedDownloadHost(host); got != want {
			t.Errorf("isAllowedDownloadHost(%q) = %v, want %v", host, got, want)
		}
	}
}
