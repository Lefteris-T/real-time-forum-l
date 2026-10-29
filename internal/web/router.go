// Package web defines routing and the application-wide middleware chain.
package web

import (
	"forum/internal/web/middleware"
	"log"
	"net/http"
	"strings"
)

// Route pairs a path pattern with its allowed methods and handler.
type Route struct {
	Methods []string
	Pattern string
	Handler http.Handler
}

// Handlers groups constructed endpoints for router assembly.
type Handlers struct {
	Home            http.Handler
	Register        http.Handler
	Login           http.Handler
	Logout          http.Handler
	SessionStatus   http.Handler
	CategoryList    http.Handler
	ContactList     http.Handler
	MessageHistory  http.Handler
	PostAPI         http.Handler
	PostDetailAPI   http.Handler
	CommentAPI      http.Handler
	WebSocket       http.Handler
	PostCreation    http.Handler
	PostDetail      http.Handler
	CommentCreate   http.Handler
	PostReaction    http.Handler
	CommentReaction http.Handler
	Static          http.Handler
}

// NewRouter centralizes method enforcement and Allow-header responses.
func NewRouter(routes []Route) http.Handler {
	mux := http.NewServeMux()

	for _, route := range routes {
		mux.Handle(
			route.Pattern,
			methodHandler(
				route.Methods,
				route.Handler,
			),
		)
	}

	return mux
}

func methodHandler(
	allowedMethods []string,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(allowedMethods) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		for _, method := range allowedMethods {
			if r.Method == method {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set(
			"Allow",
			strings.Join(allowedMethods, ", "),
		)

		writeError(
			w,
			http.StatusMethodNotAllowed,
		)
	})
}
func writeError(
	w http.ResponseWriter,
	status int,
) {
	w.WriteHeader(status)

	_, _ = w.Write(
		[]byte(http.StatusText(status)),
	)
}

// NewForumRouter declares every public and protected forum route.
func NewForumRouter(h Handlers) http.Handler {
	routes := []Route{
		{
			Methods: []string{http.MethodGet},
			Pattern: "/",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					writeError(w, http.StatusNotFound)
					return
				}

				h.Home.ServeHTTP(w, r)
			}),
		},
		{
			Methods: []string{
				http.MethodGet,
				http.MethodPost,
			},
			Pattern: "/register",
			Handler: h.Register,
		},
		{
			Methods: []string{
				http.MethodGet,
				http.MethodPost,
			},
			Pattern: "/login",
			Handler: h.Login,
		},
		{
			Methods: []string{http.MethodPost},
			Pattern: "/logout",
			Handler: h.Logout,
		},
		{
			Methods: []string{http.MethodGet},
			Pattern: "/ws",
			Handler: middleware.RequireAuth(h.WebSocket),
		},
		{
			Methods: []string{http.MethodGet},
			Pattern: "/api/session",
			Handler: h.SessionStatus,
		},
		{
			Methods: []string{http.MethodGet},
			Pattern: "/api/categories",
			Handler: middleware.RequireAuth(h.CategoryList),
		},
		{
			Methods: []string{http.MethodGet},
			Pattern: "/api/contacts",
			Handler: middleware.RequireAuth(h.ContactList),
		},
		{
			Methods: nil,
			Pattern: "/api/conversations/",
			Handler: middleware.RequireAuth(h.MessageHistory),
		},
		{
			Methods: []string{http.MethodGet, http.MethodPost},
			Pattern: "/api/posts",
			Handler: middleware.RequireAuth(h.PostAPI),
		},
		{
			Methods: nil,
			Pattern: "/api/posts/",
			Handler: middleware.RequireAuth(postAPIRoutes(h.PostDetailAPI, h.CommentAPI)),
		},
		{
			Methods: []string{http.MethodGet},
			Pattern: "/posts/new",
			Handler: middleware.RequireAuth(h.PostCreation),
		},
		{

			Methods: []string{http.MethodPost},
			Pattern: "/posts",
			Handler: middleware.RequireAuth(h.PostCreation),
		},
		{
			Methods: []string{http.MethodGet},
			Pattern: "/static/",
			Handler: http.StripPrefix(
				"/static/",
				h.Static,
			),
		},
		{
			Methods: nil,
			Pattern: "/posts/",
			Handler: middleware.RequireAuth(
				postRoutes(
					h.PostDetail,
					h.CommentCreate,
					h.PostReaction,
				),
			),
		},
		{
			Methods: nil,
			Pattern: "/comments/",
			Handler: middleware.RequireAuth(
				commentRoutes(
					h.CommentReaction,
				),
			),
		},
	}
	return NewRouter(routes)
}

func postAPIRoutes(detail, comment http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/posts/")
		parts := strings.Split(path, "/")
		switch {
		case len(parts) == 1 && parts[0] != "":
			methodHandler([]string{http.MethodGet}, detail).ServeHTTP(w, r)
		case len(parts) == 2 && parts[0] != "" && parts[1] == "comments":
			methodHandler([]string{http.MethodPost}, comment).ServeHTTP(w, r)
		default:
			writeError(w, http.StatusNotFound)
		}
	})
}

func postRoutes(
	detail http.Handler,
	commentCreate http.Handler,
	postReaction http.Handler,
) http.Handler {
	// Dynamic post paths are parsed here so individual handlers receive a
	// consistent resource URL without duplicating route-shape checks.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/posts/")
		parts := strings.Split(path, "/")

		switch {
		case len(parts) == 1 && parts[0] != "":
			methodHandler(
				[]string{http.MethodGet},
				detail,
			).ServeHTTP(w, r)

		case len(parts) == 2 && parts[0] != "" && parts[1] == "comments":
			methodHandler(
				[]string{http.MethodPost},
				commentCreate,
			).ServeHTTP(w, r)

		case len(parts) == 2 && parts[0] != "" && parts[1] == "react":
			methodHandler(
				[]string{http.MethodPost},
				postReaction,
			).ServeHTTP(w, r)

		default:
			writeError(
				w,
				http.StatusNotFound,
			)
		}
	})
}
func commentRoutes(
	commentReaction http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(
			r.URL.Path,
			"/comments/",
		)

		parts := strings.Split(path, "/")

		switch {
		case len(parts) == 2 &&
			parts[0] != "" &&
			parts[1] == "react":

			methodHandler(
				[]string{http.MethodPost},
				commentReaction,
			).ServeHTTP(w, r)

		default:
			writeError(
				w,
				http.StatusNotFound,
			)
		}
	})
}

// WithMiddleware applies recovery inside request logging so even recovered
// panics receive a status and log entry.
func WithMiddleware(
	logger *log.Logger,
	next http.Handler,
) http.Handler {
	return middleware.RequestLogging(
		logger,
		middleware.Recovery(
			logger,
			middleware.SecurityHeaders(next),
		),
	)
}
