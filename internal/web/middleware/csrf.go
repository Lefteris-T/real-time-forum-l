package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

const maxUnsafeRequestBody = 1 << 20
const maxPrivateMessageRequestBody = 6 << 20

// CSRFProtection checks browser metadata and the request origin on every
// unsafe method. publicOrigin is required when TLS terminates at a proxy.
func CSRFProtection(publicOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			values := r.Header.Values("Origin")
			if len(values) == 0 {
				referers := r.Header.Values("Referer")
				if len(referers) == 1 {
					if referer, err := url.Parse(strings.TrimSpace(referers[0])); err == nil && referer.Host != "" {
						values = []string{referer.Scheme + "://" + referer.Host}
					}
				}
			}
			if len(values) != 1 || !matchesRequestOrigin(values[0], expectedRequestOrigin(r, publicOrigin)) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			bodyLimit := int64(maxUnsafeRequestBody)
			if r.URL.Path == "/api/messages" {
				bodyLimit = maxPrivateMessageRequestBody
			}
			if r.ContentLength > bodyLimit {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)

			next.ServeHTTP(w, r)
		})
	}
}

func expectedRequestOrigin(r *http.Request, configured string) string {
	if configured != "" {
		return configured
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func matchesRequestOrigin(candidate, expected string) bool {
	got, err := url.Parse(strings.TrimSpace(candidate))
	if err != nil || got.Opaque != "" || got.User != nil || got.Host == "" || got.Scheme == "" ||
		got.RawQuery != "" || got.Fragment != "" || (got.Path != "" && got.Path != "/") {
		return false
	}
	want, err := url.Parse(expected)
	if err != nil || want.Host == "" || want.Scheme == "" {
		return false
	}
	return strings.EqualFold(got.Scheme, want.Scheme) && strings.EqualFold(got.Host, want.Host)
}
