package middleware

import "net/http"

// SecurityHeaders adds baseline browser security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set(
			"X-Content-Type-Options",
			"nosniff",
		)
		w.Header().Set(
			"X-Frame-Options",
			"DENY",
		)
		w.Header().Set(
			"Referrer-Policy",
			"same-origin",
		)
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self' ws: wss:",
		)
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")

		next.ServeHTTP(w, r)
	})
}
