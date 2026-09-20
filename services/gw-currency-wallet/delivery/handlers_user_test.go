package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
)

// Fake UserService для тестов
type mockUserService struct {
	registerFunc func(ctx context.Context, username, email, password string) error
	loginFunc    func(ctx context.Context, username, password string) (string, error)
	called       bool
	loginCalled  bool
	username     string
	email        string
	password     string
}

func (m *mockUserService) Register(ctx context.Context, username, email, password string) error {
	m.called = true
	m.username = username
	m.email = email
	m.password = password

	if m.registerFunc != nil {
		return m.registerFunc(ctx, username, email, password)
	}

	return nil
}

func (m *mockUserService) Login(ctx context.Context, username, password string) (string, error) {
	m.loginCalled = true
	m.username = username
	m.password = password

	if m.loginFunc != nil {
		return m.loginFunc(ctx, username, password)
	}

	return "", nil
}

func newTestHandler(mockUsr *mockUserService) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(nil, mockUsr, logger)
}

func TestHandler_Register_Success(t *testing.T) {
	mockUsr := &mockUserService{}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "SeCrEtS-New902!",
		"email": "ivan123@ya.ru"
	}`

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if !mockUsr.called {
		t.Fatal("expected Register to be called")
	}

	if mockUsr.username != "ivan_ivanov" {
		t.Errorf("expected username %q, got %q", "ivan_ivanov", mockUsr.username)
	}

	if mockUsr.email != "ivan123@ya.ru" {
		t.Errorf("expected email %q, got %q", "ivan123@ya.ru", mockUsr.email)
	}

	if mockUsr.password != "SeCrEtS-New902!" {
		t.Errorf("expected password %q, got %q", "SeCrEtS-New902!", mockUsr.password)
	}
}

func TestHandler_Register_Success_Body_Answer(t *testing.T) {
	mockUsr := &mockUserService{}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "SeCrEtS-New902!",
		"email": "ivan123@ya.ru"
	}`

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}

	var response RegisterResponse

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	expected := "User registered successfully"

	if response.Message != expected {
		t.Errorf("expected message %q, got %q", expected, response.Message)
	}

	if !mockUsr.called {
		t.Fatal("expected UserService.Register to be called")
	}
}

func TestHandler_Register_InvalidJSON(t *testing.T) {
	mockUsr := &mockUserService{}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password":
	}`

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if mockUsr.called {
		t.Fatal("UserService.Register should not be called")
	}
}

func TestHandler_Register_InvalidEmail(t *testing.T) {
	mockUsr := &mockUserService{
		registerFunc: func(ctx context.Context, username, email, password string) error {
			return service.ErrEmailInvalid
		},
	}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "SeCrEtS-New902!",
		"email": "invalid-email"
	}`

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if !mockUsr.called {
		t.Fatal("expected Register to be called")
	}
}

func TestHandler_Register_UserAlreadyExists(t *testing.T) {
	mockUsr := &mockUserService{
		registerFunc: func(ctx context.Context, username, email, password string) error {
			return service.ErrUserAlreadyExists
		},
	}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "SeCrEtS-New902!",
		"email": "ivan123@ya.ru"
	}`

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandler_Register_InternalError(t *testing.T) {
	internalErr := errors.New("database connection failed")

	mockUsr := &mockUserService{
		registerFunc: func(ctx context.Context, username, email, password string) error {
			return internalErr
		},
	}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "SeCrEtS-New902!",
		"email": "ivan123@ya.ru"
	}`

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestHandler_Register_Errors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{
			name:       "empty email",
			serviceErr: service.ErrEmailEmpty,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid email",
			serviceErr: service.ErrEmailInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "user already exists",
			serviceErr: service.ErrUserAlreadyExists,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty username",
			serviceErr: service.ErrUsernameEmpty,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "username too short",
			serviceErr: service.ErrUsernameTooShort,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "username too long",
			serviceErr: service.ErrUsernameTooLong,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid username",
			serviceErr: service.ErrUsernameInvalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password too short",
			serviceErr: service.ErrPasswordTooShort,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password no uppercase",
			serviceErr: service.ErrPasswordNoUpper,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password no lowercase",
			serviceErr: service.ErrPasswordNoLower,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password no number",
			serviceErr: service.ErrPasswordNoNumber,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password no special",
			serviceErr: service.ErrPasswordNoSpecial,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "internal error",
			serviceErr: errors.New("database error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUsr := &mockUserService{
				registerFunc: func(ctx context.Context, busername, email, password string) error {
					return tt.serviceErr
				},
			}

			handler := newTestHandler(mockUsr)

			body := `{
				"username": "ivan_ivanov",
				"password": "SeCrEtS-New902!",
				"email": "ivan123@ya.ru"
			}`

			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestHandler_Login_Success(t *testing.T) {
	mockUsr := &mockUserService{
		loginFunc: func(ctx context.Context, username, password string) (string, error) {
			return "test-jwt-token", nil
		},
	}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "SeCrEtS-New902!"
	}`

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response LoginResponse

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Token != "test-jwt-token" {
		t.Errorf("expected token %q, got %q", "test-jwt-token", response.Token)
	}

	if !mockUsr.loginCalled {
		t.Fatal("expected UserService.Login to be called")
	}

	if mockUsr.username != "ivan_ivanov" {
		t.Errorf("expected username %q, got %q", "ivan_ivanov", mockUsr.username)
	}

	if mockUsr.password != "SeCrEtS-New902!" {
		t.Errorf("expected password %q, got %q", "SeCrEtS-New902!", mockUsr.password)
	}
}

func TestHandler_Login_InvalidJSON(t *testing.T) {
	mockUsr := &mockUserService{}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password":
	}`

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if mockUsr.loginCalled {
		t.Fatal("UserService.Login should not be called")
	}
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	mockUsr := &mockUserService{
		loginFunc: func(ctx context.Context, username string, password string) (string, error) {
			return "", errors.New("invalid credentials")
		},
	}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "wrong-password"
	}`

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if !mockUsr.loginCalled {
		t.Fatal("expected UserService.Login to be called")
	}
}

func TestHandler_Login_InvalidCredentials_Response(t *testing.T) {
	mockUsr := &mockUserService{
		loginFunc: func(ctx context.Context, username, password string) (string, error) {
			return "", errors.New("user not found in PostgreSQL")
		},
	}

	handler := newTestHandler(mockUsr)

	body := `{
		"username": "ivan_ivanov",
		"password": "wrong-password"
	}`

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))

	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	responseBody := rec.Body.String()

	if strings.Contains(responseBody, "PostgreSQL") {
		t.Error("internal error must not be exposed to client")
	}

	if !strings.Contains(responseBody, "Invalid username or password") {
		t.Errorf("expected error message in response, got %s", responseBody)
	}
}
