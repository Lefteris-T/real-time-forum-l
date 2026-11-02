// Package app wires infrastructure, services, handlers, and server lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"forum/internal/config"
	"forum/internal/database"
	"forum/internal/repository"
	"forum/internal/service"
	sessionpkg "forum/internal/session"
	"forum/internal/web"
	"forum/internal/web/handler"
	"forum/internal/web/middleware"
	"forum/internal/web/view"
)

const shutdownTimeout = 5 * time.Second

// Run starts the configured server and shuts it down when ctx is cancelled.
// Startup and shutdown failures are returned to the command package.
func Run(ctx context.Context, cfg config.Config) error {
	tlsConfig := newTLSConfig()

	if cfg.HTTPSEnabled {
		certificate, err := loadTLSCertificate(
			cfg.TLSCertFile,
			cfg.TLSKeyFile,
		)
		if err != nil {
			return err
		}

		tlsConfig.Certificates = append(
			tlsConfig.Certificates,
			certificate,
		)
	}

	appHandler, cleanup, err := buildHandler(cfg)
	if err != nil {
		return err
	}

	defer cleanup()

	server := newHTTPServer(
		cfg.Address,
		appHandler,
		tlsConfig,
	)

	serverErrors := make(chan error, 1)

	go func() {
		var serveError error

		if cfg.HTTPSEnabled {
			serveError = server.ListenAndServeTLS("", "")
		} else {
			serveError = server.ListenAndServe()
		}

		serverErrors <- normalizeServerError(serveError)
	}()

	select {
	case err := <-serverErrors:
		return err

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf(
				"shutdown server: %w",
				err,
			)
		}

		err := <-serverErrors
		return err
	}
}

func buildHandler(
	cfg config.Config,
) (http.Handler, func(), error) {
	// Build dependencies from the outside in: repositories feed services,
	// services feed handlers, and handlers are mounted on the router.
	databaseDir := filepath.Dir(cfg.DatabasePath)

	if err := os.MkdirAll(databaseDir, 0700); err != nil {
		return nil, nil, fmt.Errorf(
			"create database directory: %w",
			err,
		)
	}
	if err := secureDatabaseDirectory(databaseDir); err != nil {
		return nil, nil, fmt.Errorf("secure database directory: %w", err)
	}
	if err := secureDatabaseFile(cfg.DatabasePath); err != nil {
		return nil, nil, fmt.Errorf("secure database file: %w", err)
	}
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"open database: %w",
			err,
		)
	}
	rateLimiter, err := middleware.NewHTTPRateLimiter(cfg.TrustedProxyCIDRs)
	if err != nil {
		db.Close()

		return nil, nil, fmt.Errorf(
			"create HTTP rate limiter: %w",
			err,
		)
	}
	webSocketHub := handler.NewWebSocketHub()

	cleanup := func() {
		webSocketHub.Shutdown()
		rateLimiter.Stop()
		db.Close()
	}

	if err := database.Migrate(
		db,
		resolveProjectPath("migrations"),
	); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf(
			"migrate database: %w",
			err,
		)
	}

	users := repository.NewUserRepository(db)
	sessions := repository.NewSessionRepository(db)
	categories := repository.NewCategoryRepository(db)
	conversations := repository.NewConversationRepository(db)
	posts := repository.NewPostRepository(db)
	comments := repository.NewCommentRepository(db)
	messages := repository.NewMessageRepository(db)
	reactions := repository.NewReactionRepository(db)

	passwords := service.NewPasswordManager()

	authService := service.NewAuthService(
		users,
		passwords,
	)

	loginService := service.NewLoginService(
		users,
		passwords,
		sessions,
		cfg.SessionDuration,
	)

	postService := service.NewPostService(
		posts,
		categories,
	)

	commentService := service.NewCommentService(
		comments,
	)

	messageService := service.NewMessageService(
		conversations,
		messages,
		webSocketHub,
	)

	profileService := service.NewProfileService(users)

	reactionService := service.NewReactionService(
		reactions,
	)

	sessionManager := sessionpkg.NewManager(
		cfg.CookieName,
		cfg.SessionDuration,
		cfg.SecureCookie,
	)

	renderer, err := view.NewRenderer(
		resolveProjectPath("templates"),
	)
	if err != nil {
		cleanup()

		return nil, nil, fmt.Errorf(
			"create renderer: %w",
			err,
		)
	}

	registerHandler := handler.NewRegisterHandler(authService)

	loginHandler := handler.NewLoginHandler(
		loginService,
		sessionManager,
		webSocketHub,
	)

	logoutHandler := handler.NewLogoutHandler(
		loginService,
		sessionManager,
		webSocketHub,
	)

	spaHandler := handler.NewSPAHandler(
		resolveProjectPath("static/index.html"),
	)

	postDetailHandler := handler.NewPostDetailHandler(
		posts,
		renderer,
	)

	postCreationHandler := handler.NewPostCreationHandler(
		postService,
		categories,
		renderer,
	)

	commentHandler := handler.NewCommentSubmissionHandler(
		commentService,
	)

	postReactionHandler := handler.NewPostReactionHandler(
		reactionService,
	)

	commentReactionHandler := handler.NewCommentReactionHandler(
		reactionService,
		comments,
	)
	// Routing describes HTTP shape only; business and persistence rules stay in
	// their respective service and repository layers.
	router := web.NewForumRouter(
		web.Handlers{
			Home:            spaHandler,
			Register:        registerHandler,
			Login:           loginHandler,
			Logout:          logoutHandler,
			SessionStatus:   handler.NewSessionStatusHandler(),
			CategoryList:    handler.NewCategoryListHandler(categories),
			ContactList:     handler.NewContactListHandler(conversations, webSocketHub),
			MessageHistory:  handler.NewMessageHistoryHandler(messages),
			MessageRead:     handler.NewMessageReadHandler(messages),
			MessageSend:     handler.NewMessageSendHandler(messageService),
			MessageImage:    handler.NewMessageImageHandler(messages),
			Profile:         handler.NewProfileHandler(profileService),
			PostAPI:         handler.NewPostAPIHandler(postService, posts),
			PostDetailAPI:   handler.NewPostDetailAPIHandler(posts),
			CommentAPI:      handler.NewCommentAPIHandler(commentService),
			WebSocket:       handler.NewWebSocketHandler(webSocketHub, cfg.PublicOrigin),
			PostCreation:    postCreationHandler,
			PostDetail:      postDetailHandler,
			CommentCreate:   commentHandler,
			PostReaction:    postReactionHandler,
			CommentReaction: commentReactionHandler,
			Static: web.NewStaticFileHandler(
				resolveProjectPath("static"),
			),
		},
	)

	// Authentication runs before routing so all handlers receive the same
	// request-scoped current-user behavior.
	authenticate := middleware.NewAuthentication(
		sessionManager,
		sessions,
		users,
	)

	appHandler := rateLimiter.Middleware(router)
	appHandler = middleware.CSRFProtection(cfg.PublicOrigin)(appHandler)
	appHandler = authenticate(appHandler)

	logger := log.New(
		os.Stdout,
		"",
		log.LstdFlags,
	)

	appHandler = web.WithMiddleware(
		logger,
		appHandler,
	)

	return appHandler, cleanup, nil
}

func secureDatabaseDirectory(path string) error {
	cleanPath := filepath.Clean(path)
	if cleanPath == filepath.Clean(os.TempDir()) {
		return nil
	}
	info, err := os.Lstat(cleanPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("database directory must be a real directory")
	}
	if info.Mode().Perm()&0077 == 0 {
		return nil
	}
	if filepath.Base(cleanPath) == "data" || isDedicatedTempDirectory(cleanPath) {
		return os.Chmod(cleanPath, 0700)
	}
	return fmt.Errorf("database directory permissions must be private (0700)")
}

func isDedicatedTempDirectory(path string) bool {
	tempRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(tempRoot, cleanPath)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func secureDatabaseFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0600)
		if errors.Is(createErr, os.ErrExist) {
			return secureDatabaseFile(path)
		}
		if createErr != nil {
			return createErr
		}
		return file.Close()
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("database path must be a regular file, not a symlink")
	}
	return os.Chmod(path, 0600)
}

func normalizeServerError(err error) error {
	// ErrServerClosed is expected during graceful shutdown.
	if errors.Is(
		err,
		http.ErrServerClosed,
	) {
		return nil
	}

	return err
}

func resolveProjectPath(name string) string {
	// Package tests may run below the project root, unlike the built command.
	if _, err := os.Stat(name); err == nil {
		return name
	}

	return filepath.Join("..", "..", name)
}
