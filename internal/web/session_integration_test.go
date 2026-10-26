package web_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"forum/internal/database"
	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/service"
	"forum/internal/session"
	"forum/internal/web/handler"
	"forum/internal/web/middleware"
)

func TestSingleActiveBrowserSession(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	defer db.Close()

	err = database.Migrate(
		db,
		filepath.Join("..", "..", "migrations"),
	)
	if err != nil {
		t.Fatalf("database.Migrate(): %v", err)
	}

	users := repository.NewUserRepository(db)
	sessions := repository.NewSessionRepository(db)

	passwords := service.NewPasswordManager()

	passwordHash, err := passwords.Hash("strong-password-123")
	if err != nil {
		t.Fatalf("Hash(): %v", err)
	}

	userID, err := users.Create(model.User{
		Email: "lefteris@example.com", Username: "lefteris", Age: 30,
		Gender: "test", FirstName: "Test", LastName: "User", PasswordHash: passwordHash,
	})
	if err != nil {
		t.Fatalf("users.Create(): %v", err)
	}

	sessionManager := session.NewManager(
		"forum_session",
		24*time.Hour,
		false,
	)

	loginService := service.NewLoginService(
		users,
		passwords,
		sessions,
		24*time.Hour,
	)

	loginHandler := handler.NewLoginHandler(
		loginService,
		sessionManager,
	)

	mux := http.NewServeMux()

	mux.Handle(
		"/login",
		loginHandler,
	)

	mux.HandleFunc(
		"/",
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			user, ok := middleware.CurrentUser(r.Context())
			if !ok {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("guest"))
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(user.Username))
		},
	)

	authentication := middleware.NewAuthentication(
		sessionManager,
		sessions,
		users,
	)

	server := httptest.NewServer(
		authentication(mux),
	)
	defer server.Close()

	browserA := newBrowser(t)
	browserB := newBrowser(t)

	assertBrowserState(
		t,
		browserA,
		server.URL,
		"guest",
	)

	assertBrowserState(
		t,
		browserB,
		server.URL,
		"guest",
	)

	loginBrowser(
		t,
		browserA,
		server.URL,
	)

	assertBrowserState(
		t,
		browserA,
		server.URL,
		"lefteris",
	)

	loginBrowser(
		t,
		browserB,
		server.URL,
	)

	assertBrowserState(
		t,
		browserA,
		server.URL,
		"guest",
	)

	assertBrowserState(
		t,
		browserB,
		server.URL,
		"lefteris",
	)

	sessionB, err := sessions.ByID(
		browserSessionID(
			t,
			browserB,
			server.URL,
		),
	)
	if err != nil {
		t.Fatalf("browser B session not found: %v", err)
	}

	if sessionB.UserID != userID {
		t.Fatalf(
			"session user ID = %d, want %d",
			sessionB.UserID,
			userID,
		)
	}
}

func newBrowser(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New(): %v", err)
	}

	return &http.Client{
		Jar: jar,
	}
}

func loginBrowser(
	t *testing.T,
	client *http.Client,
	serverURL string,
) {
	t.Helper()

	form := url.Values{}
	form.Set(
		"identifier",
		"lefteris@example.com",
	)
	form.Set(
		"password",
		"strong-password-123",
	)

	resp, err := client.PostForm(
		serverURL+"/login",
		form,
	)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf(
			"login final status = %d, want %d",
			resp.StatusCode,
			http.StatusOK,
		)
	}
}

func assertBrowserState(
	t *testing.T,
	client *http.Client,
	serverURL string,
	want string,
) {
	t.Helper()

	resp, err := client.Get(serverURL)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(): %v", err)
	}

	if string(body) != want {
		t.Fatalf(
			"browser state = %q, want %q",
			string(body),
			want,
		)
	}
}

func browserSessionID(
	t *testing.T,
	client *http.Client,
	serverURL string,
) string {
	t.Helper()

	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("url.Parse(): %v", err)
	}

	for _, cookie := range client.Jar.Cookies(u) {
		if cookie.Name == "forum_session" {
			return cookie.Value
		}
	}

	t.Fatal("forum_session cookie not found")

	return ""
}
