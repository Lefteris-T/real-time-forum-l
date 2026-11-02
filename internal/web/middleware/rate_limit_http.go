package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const unknownClientKey = "unknown-client"

func clientKey(r *http.Request) string {
	return clientKeyWithTrustedProxies(r, nil)
}

func clientKeyWithTrustedProxies(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return unknownClientKey
	}

	address, err := netip.ParseAddr(host)
	if err != nil {
		return unknownClientKey
	}

	peer := address.Unmap()
	if !isTrustedProxy(peer, trusted) {
		return peer.String()
	}

	var forwarded []string
	for _, value := range r.Header.Values("X-Forwarded-For") {
		forwarded = append(forwarded, strings.Split(value, ",")...)
	}
	if len(forwarded) == 0 {
		return peer.String()
	}
	current := peer
	for index := len(forwarded) - 1; index >= 0; index-- {
		forwardedAddress, parseErr := netip.ParseAddr(strings.TrimSpace(forwarded[index]))
		if parseErr != nil {
			return peer.String()
		}
		current = forwardedAddress.Unmap()
		if !isTrustedProxy(current, trusted) {
			return current.String()
		}
	}
	return current.String()
}

func isTrustedProxy(address netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(address.Unmap()) {
			return true
		}
	}
	return false
}

type rateLimitRule string

const (
	rateLimitRuleNone     rateLimitRule = ""
	rateLimitRuleLogin    rateLimitRule = "login"
	rateLimitRuleRegister rateLimitRule = "register"
	rateLimitRulePost     rateLimitRule = "post"
	rateLimitRuleComment  rateLimitRule = "comment"
	rateLimitRuleReaction rateLimitRule = "reaction"
	rateLimitRuleMessage  rateLimitRule = "message"
	rateLimitRuleProfile  rateLimitRule = "profile"
)

func requestRateLimitRule(r *http.Request) rateLimitRule {
	if r.Method != http.MethodPost && r.Method != http.MethodPut &&
		r.Method != http.MethodPatch && r.Method != http.MethodDelete {
		return rateLimitRuleNone
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	switch {
	case len(parts) == 1 && parts[0] == "login":
		return rateLimitRuleLogin

	case len(parts) == 1 && parts[0] == "register":
		return rateLimitRuleRegister

	case len(parts) == 1 && parts[0] == "posts":
		return rateLimitRulePost
	case len(parts) == 2 && parts[0] == "api" && parts[1] == "posts":
		return rateLimitRulePost
	case len(parts) == 2 && parts[0] == "api" && parts[1] == "profile":
		return rateLimitRuleProfile
	case len(parts) == 2 && parts[0] == "api" && parts[1] == "messages":
		return rateLimitRuleMessage
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "conversations" && parts[2] != "" && parts[3] == "read":
		return rateLimitRuleMessage

	case len(parts) == 3 &&
		parts[0] == "posts" &&
		parts[1] != "" &&
		parts[2] == "comments":
		return rateLimitRuleComment
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "posts" && parts[2] != "" && parts[3] == "comments":
		return rateLimitRuleComment

	case len(parts) == 3 &&
		parts[0] == "posts" &&
		parts[1] != "" &&
		parts[2] == "react":
		return rateLimitRuleReaction

	case len(parts) == 3 &&
		parts[0] == "comments" &&
		parts[1] != "" &&
		parts[2] == "react":
		return rateLimitRuleReaction
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "posts" && parts[2] != "" && parts[3] == "react":
		return rateLimitRuleReaction

	default:
		return rateLimitRuleNone
	}
}

type HTTPRateLimiter struct {
	global            *RateLimiter
	byRule            map[rateLimitRule]*RateLimiter
	trustedProxyCIDRs []netip.Prefix
}

func NewHTTPRateLimiter(trustedProxyCIDRs ...[]netip.Prefix) (*HTTPRateLimiter, error) {
	return newHTTPRateLimiter(time.Now, trustedProxyCIDRs...)
}

func newHTTPRateLimiter(
	now func() time.Time,
	trustedProxyCIDRs ...[]netip.Prefix,
) (*HTTPRateLimiter, error) {
	const (
		inactiveAfter   = 10 * time.Minute
		cleanupInterval = time.Minute
	)

	limiter := &HTTPRateLimiter{
		byRule: make(map[rateLimitRule]*RateLimiter),
	}
	if len(trustedProxyCIDRs) > 0 {
		limiter.trustedProxyCIDRs = append([]netip.Prefix(nil), trustedProxyCIDRs[0]...)
	}

	created := make([]*RateLimiter, 0, 8)

	create := func(
		name string,
		requests int,
		burst int,
	) (*RateLimiter, error) {
		value, err := newRateLimiter(
			requests,
			time.Minute,
			burst,
			inactiveAfter,
			cleanupInterval,
			now,
		)
		if err != nil {
			for _, existing := range created {
				existing.Stop()
			}

			return nil, fmt.Errorf(
				"create %s rate limiter: %w",
				name,
				err,
			)
		}

		created = append(created, value)

		return value, nil
	}

	var err error

	limiter.global, err = create("global", 120, 120)
	if err != nil {
		return nil, err
	}

	policies := []struct {
		rule     rateLimitRule
		requests int
		burst    int
	}{
		{rateLimitRuleLogin, 5, 5},
		{rateLimitRuleRegister, 5, 5},
		{rateLimitRulePost, 20, 20},
		{rateLimitRuleComment, 30, 30},
		{rateLimitRuleReaction, 60, 60},
		{rateLimitRuleMessage, 30, 10},
		{rateLimitRuleProfile, 10, 5},
	}

	for _, policy := range policies {
		ruleLimiter, createErr := create(
			string(policy.rule),
			policy.requests,
			policy.burst,
		)
		if createErr != nil {
			return nil, createErr
		}

		limiter.byRule[policy.rule] = ruleLimiter
	}

	return limiter, nil
}
func (l *HTTPRateLimiter) Stop() {
	if l == nil {
		return
	}

	if l.global != nil {
		l.global.Stop()
	}

	for _, ruleLimiter := range l.byRule {
		ruleLimiter.Stop()
	}
}
func (l *HTTPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKeyWithTrustedProxies(r, l.trustedProxyCIDRs)
		rule := requestRateLimitRule(r)

		if !l.global.Allow(key) {
			writeRateLimitExceeded(w, time.Second, rule)
			return
		}

		if rule != rateLimitRuleNone {
			ruleLimiter, exists := l.byRule[rule]
			if exists {
				keys := []string{key}
				if rule == rateLimitRuleLogin {
					r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
					if err := r.ParseForm(); err == nil {
						identifier := strings.ToLower(strings.TrimSpace(r.FormValue("identifier")))
						if identifier != "" {
							digest := sha256.Sum256([]byte(identifier))
							keys = append(keys, "login-account:"+hex.EncodeToString(digest[:]))
						}
					}
				}
				if user, ok := CurrentUser(r.Context()); ok {
					keys = append(keys, fmt.Sprintf("user:%d", user.ID))
				}
				for _, limitKey := range keys {
					if !ruleLimiter.Allow(limitKey) {
						writeRateLimitExceeded(w, retryAfterForRule(rule), rule)
						return
					}
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
func retryAfterForRule(rule rateLimitRule) time.Duration {
	switch rule {
	case rateLimitRuleLogin, rateLimitRuleRegister:
		return 12 * time.Second

	case rateLimitRulePost:
		return 3 * time.Second

	case rateLimitRuleComment:
		return 2 * time.Second

	case rateLimitRuleReaction:
		return time.Second

	case rateLimitRuleMessage:
		return 2 * time.Second

	case rateLimitRuleProfile:
		return 6 * time.Second

	default:
		return time.Second
	}
}
func writeRateLimitExceeded(
	w http.ResponseWriter,
	retryAfter time.Duration,
	rule rateLimitRule,
) {
	seconds := int(retryAfter / time.Second)
	if seconds < 1 {
		seconds = 1
	}

	w.Header().Set(
		"Retry-After",
		strconv.Itoa(seconds),
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)

	returnPath, returnLabel := rateLimitReturnLink(rule)
	_, _ = fmt.Fprintf(
		w,
		`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Too Many Requests</title></head>
<body><main><h1>Too Many Requests</h1><p>Please wait %d seconds before trying again.</p><p><a href="%s">%s</a></p></main></body>
</html>
`,
		seconds,
		returnPath,
		returnLabel,
	)
}

func rateLimitReturnLink(rule rateLimitRule) (string, string) {
	switch rule {
	case rateLimitRuleLogin:
		return "/login", "Return to login"

	case rateLimitRuleRegister:
		return "/register", "Return to registration"

	default:
		return "/", "Return to the forum"
	}
}
