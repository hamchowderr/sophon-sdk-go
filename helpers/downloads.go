package helpers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// AllowedDownloadHosts is the set of host suffixes a presigned SOPHON output
// URL is permitted to resolve to. SOPHON serves encoded outputs from a
// presigned Backblaze B2 URL (24h TTL); DownloadOutput refuses to fetch — or
// to follow a redirect to — any host outside this list, and follows at most
// one redirect. This keeps the download path from becoming an open redirect
// follower if the upstream Location header is ever manipulated.
//
// A host matches an entry if it equals the entry or is a subdomain of it
// (e.g. "f004.backblazeb2.com" matches "backblazeb2.com"). Override only if
// the platform begins serving outputs from a different CDN:
//
//	helpers.AllowedDownloadHosts = append(helpers.AllowedDownloadHosts, "cdn.example.com")
var AllowedDownloadHosts = []string{"backblazeb2.com"}

// isAllowedDownloadHost reports whether host (which may include a port) matches
// one of the AllowedDownloadHosts suffixes, case-insensitively.
func isAllowedDownloadHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, allowed := range AllowedDownloadHosts {
		allowed = strings.ToLower(strings.TrimSuffix(allowed, "."))
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// DownloadOutput streams an encoded job output to w. It calls
// GET /v1/jobs/{id}/output to obtain a presigned redirect, follows the
// redirect to a public URL, and copies the body. Returns the byte count
// written to w. Errors classify to *NotFoundError when the job is unknown
// and other typed *APIError variants for non-2xx upstream responses.
//
// The presigned URL's host — and any single redirect hop it makes — must
// match AllowedDownloadHosts (Backblaze B2 by default); the download client
// follows at most one redirect and never to a non-allowlisted host.
//
//	downloads := helpers.NewDownloadsClient(client)
//	f, _ := os.Create("out.mp4")
//	defer f.Close()
//	n, err := helpers.DownloadOutput(ctx, downloads, jobID, f)
func DownloadOutput(ctx context.Context, api DownloadsClient, jobID string, w io.Writer) (int64, error) {
	if w == nil {
		return 0, fmt.Errorf("sophon: nil writer")
	}
	signedURL, err := api.GetOutputURL(ctx, jobID)
	if err != nil {
		return 0, err
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		return 0, fmt.Errorf("sophon: invalid output url: %w", err)
	}
	if !isAllowedDownloadHost(parsed.Host) {
		return 0, fmt.Errorf("sophon: refusing to download from non-allowlisted host %q", parsed.Host)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
	if err != nil {
		return 0, err
	}
	// Never an open follower: cap at one redirect and re-validate the hop host
	// against the allowlist.
	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return fmt.Errorf("sophon: refusing to follow more than one output redirect")
			}
			if !isAllowedDownloadHost(r.URL.Host) {
				return fmt.Errorf("sophon: refusing redirect to non-allowlisted host %q", r.URL.Host)
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, &NetworkError{&APIError{Status: 0, Message: err.Error()}}
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, classifyError(resp, fmt.Errorf("sophon: download failed: %s", resp.Status))
	}
	return io.Copy(w, resp.Body)
}

// DownloadOutputToFile is a thin wrapper that creates path (overwriting
// any existing file) and streams the output into it. Closes the file on
// return.
func DownloadOutputToFile(ctx context.Context, api DownloadsClient, jobID, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return DownloadOutput(ctx, api, jobID, f)
}
