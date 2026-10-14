package mods

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const userAgent = "deploykithq/deploykit-game-images (dk-entrypoint)"

// Downloads are only ever fetched from these hosts — the same idea as the
// fixed hosts of DeployKit's services/mod-sources/. Modrinth's own allowlist
// for .mrpack files is cdn.modrinth.com, github.com, raw.githubusercontent.com
// and gitlab.com; GitHub release assets redirect to *.githubusercontent.com.
var defaultHosts = []string{
	"api.modrinth.com", "cdn.modrinth.com",
	"api.curseforge.com", "edge.forgecdn.net", "mediafilez.forgecdn.net",
	"github.com", "raw.githubusercontent.com", "objects.githubusercontent.com",
	"release-assets.githubusercontent.com", "gitlab.com",
}

func defaultAllowHost(host string) bool {
	for _, h := range defaultHosts {
		if host == h {
			return true
		}
	}
	return false
}

type Client struct {
	HTTP          *http.Client
	ModrinthAPI   string
	CurseForgeAPI string
	CurseForgeKey string
	AllowHost     func(host string) bool
}

func NewClient() *Client {
	c := &Client{
		ModrinthAPI:   "https://api.modrinth.com",
		CurseForgeAPI: "https://api.curseforge.com",
		CurseForgeKey: os.Getenv("DK_CURSEFORGE_API_KEY"),
		AllowHost:     defaultAllowHost,
	}
	c.HTTP = &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !c.AllowHost(req.URL.Hostname()) {
				return fmt.Errorf("redirect to %s is not an allowed host", req.URL.Hostname())
			}
			return nil
		},
	}
	return c
}

func (c *Client) checkURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !c.AllowHost(u.Hostname()) {
		return nil, fmt.Errorf("%s is not an allowed host", u.Hostname())
	}
	return u, nil
}

func (c *Client) do(method, raw string, body io.Reader, headers map[string]string) (*http.Response, error) {
	if _, err := c.checkURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, raw, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, &StatusError{URL: raw, Code: resp.StatusCode}
	}
	return resp, nil
}

type StatusError struct {
	URL  string
	Code int
}

func (e *StatusError) Error() string { return fmt.Sprintf("GET %s: HTTP %d", e.URL, e.Code) }

func (c *Client) getJSON(raw string, headers map[string]string, v any) error {
	resp, err := c.do(http.MethodGet, raw, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) postJSON(raw string, headers map[string]string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	h := map[string]string{"Content-Type": "application/json"}
	for k, v := range headers {
		h[k] = v
	}
	resp, err := c.do(http.MethodPost, raw, strings.NewReader(string(b)), h)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// Hashes holds the expected digests of a file; at least one must be set.
type Hashes struct {
	SHA1   string
	SHA512 string
}

func (h Hashes) verifier() (hash.Hash, string, error) {
	switch {
	case h.SHA512 != "":
		return sha512.New(), strings.ToLower(h.SHA512), nil
	case h.SHA1 != "":
		return sha1.New(), strings.ToLower(h.SHA1), nil
	}
	return nil, "", errors.New("no hash to verify the download against")
}

// download fetches raw into dest atomically, verifying the hash.
func (c *Client) download(raw, dest string, want Hashes) error {
	h, expected, err := want.verifier()
	if err != nil {
		return err
	}
	resp, err := c.do(http.MethodGet, raw, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".dk-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != expected {
		return fmt.Errorf("%s: hash mismatch", filepath.Base(dest))
	}
	// CreateTemp makes 0600; a mod is not a secret.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// fileMatches reports whether path exists with the expected hash.
func fileMatches(path string, want Hashes) bool {
	h, expected, err := want.verifier()
	if err != nil {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == expected
}

// safeJoin joins a pack-supplied relative path under base, refusing anything
// that would escape it.
func safeJoin(base, rel string) (string, error) {
	rel = filepath.FromSlash(rel)
	if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, `\`) {
		return "", fmt.Errorf("unsafe path %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path %q", rel)
	}
	return filepath.Join(base, clean), nil
}
