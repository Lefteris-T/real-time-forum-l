package middleware

import (
	"bufio"
	"log"
	"net"
	"net/http"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader remembers the final status code for request logging.
func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.ResponseWriter.Write(body)
}

// Hijack forwards the optional capability required by WebSocket upgrades and
// records their successful HTTP status for request logs.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, readWriter, err := hijacker.Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, readWriter, err
}

// Unwrap lets http.ResponseController reach optional capabilities supported by
// the underlying server writer.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// RequestLogging records method, path, status, and elapsed request time.
func RequestLogging(
	logger *log.Logger,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{
			ResponseWriter: w,
		}

		next.ServeHTTP(sw, r)

		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}

		logger.Printf(
			"request method=%s path=%s status=%d",
			r.Method,
			r.URL.Path,
			status,
		)
	})
}
