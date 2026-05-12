// Copyright 2026 bibihez. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// persistentJar is a minimal http.CookieJar that survives across CLI
// invocations by serializing to a single JSON file. The motivation is
// DataDome: every fresh process otherwise looks like a new visitor and
// gets a higher bot score. Carrying the `datadome` session cookie across
// runs lets the second-and-later requests look like a returning browser.
type persistentJar struct {
	mu      sync.Mutex
	cookies map[cookieKey]storedCookie
	path    string
}

type cookieKey struct {
	Domain string
	Path   string
	Name   string
}

type storedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"http_only,omitempty"`
}

func newPersistentJar(path string) *persistentJar {
	j := &persistentJar{
		cookies: make(map[cookieKey]storedCookie),
		path:    path,
	}
	j.load()
	return j
}

func (j *persistentJar) load() {
	data, err := os.ReadFile(j.path)
	if err != nil {
		return
	}
	var stored []storedCookie
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}
	now := time.Now()
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range stored {
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		j.cookies[cookieKey{Domain: c.Domain, Path: c.Path, Name: c.Name}] = c
	}
}

func (j *persistentJar) save() {
	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return
	}
	stored := make([]storedCookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		stored = append(stored, c)
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}
	tmp := j.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, j.path)
}

func (j *persistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	j.mu.Lock()
	dirty := false
	for _, c := range cookies {
		domain := c.Domain
		if domain == "" {
			domain = u.Host
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		k := cookieKey{Domain: domain, Path: path, Name: c.Name}
		if c.MaxAge < 0 {
			if _, ok := j.cookies[k]; ok {
				delete(j.cookies, k)
				dirty = true
			}
			continue
		}
		expires := c.Expires
		if c.MaxAge > 0 {
			expires = time.Now().Add(time.Duration(c.MaxAge) * time.Second)
		}
		j.cookies[k] = storedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   domain,
			Path:     path,
			Expires:  expires,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		}
		dirty = true
	}
	j.mu.Unlock()
	if dirty {
		j.save()
	}
}

func (j *persistentJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	host := strings.ToLower(u.Host)
	var out []*http.Cookie
	for _, c := range j.cookies {
		if !domainMatches(host, c.Domain) {
			continue
		}
		if !pathMatches(u.Path, c.Path) {
			continue
		}
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		if c.Secure && u.Scheme != "https" {
			continue
		}
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	return out
}

func domainMatches(host, domain string) bool {
	domain = strings.ToLower(strings.TrimPrefix(domain, "."))
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

func pathMatches(reqPath, cookiePath string) bool {
	if cookiePath == "" || cookiePath == "/" {
		return true
	}
	if reqPath == cookiePath {
		return true
	}
	return strings.HasPrefix(reqPath, cookiePath+"/") || strings.HasPrefix(reqPath, cookiePath)
}
