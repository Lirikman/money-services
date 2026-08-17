package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
	repository "github.com/Lirikman/money_services/services/gw-currency-wallet/repository/postgres"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepo struct {
	getUserByUsernameFunc func(ctx context.Context, username string) (*models.User, error)
	getUserByEmailFunc    func(ctx context.Context, email string) (*models.User, error)
	createFunc            func(ctx context.Context, user *models.User) error
}

func (m *mockUserRepo) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return m.getUserByUsernameFunc(ctx, username)
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return m.getUserByEmailFunc(ctx, email)
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	return m.createFunc(ctx, user)
}

func TestUserService_Register(t *testing.T) {
	// Определяем структуру для тестовых случаев
	type fields struct {
		mockRepo *mockUserRepo
	}
	type args struct {
		username string
		email    string
		password string
	}

	tests := []struct {
		name    string
		setup   func(f *fields) // Настройка поведения мока перед каждым тестом
		args    args
		wantErr error
	}{
		{
			name:  "Empty email",
			setup: func(f *fields) {},
			args: args{
				username: "validuser",
				email:    "",
				password: "Password123!",
			},
			wantErr: service.ErrEmailEmpty,
		},
		{
			name:  "Invalid email",
			setup: func(f *fields) {},
			args: args{
				username: "validuser",
				email:    "invalid-email",
				password: "Password123!",
			},
			wantErr: service.ErrEmailInvalid,
		},
		{
			name:  "Empty username",
			setup: func(f *fields) {},
			args: args{
				username: "",
				email:    "existing@example.com",
				password: "Pass_word$123!",
			},
			wantErr: service.ErrUsernameEmpty,
		},
		{
			name:  "Invalid username",
			setup: func(f *fields) {},
			args: args{
				username: "YA@#%Hacker",
				email:    "existing@example.com",
				password: "Pass_word$123!",
			},
			wantErr: service.ErrUsernameInvalid,
		},
		{
			name:  "Short username",
			setup: func(f *fields) {},
			args: args{
				username: "Iv",
				email:    "existing@example.com",
				password: "Password123!",
			},
			wantErr: service.ErrUsernameTooShort,
		},
		{
			name:  "Long username",
			setup: func(f *fields) {},
			args: args{
				username: "MonsterSuperLongNameTestHackerBlackWorld",
				email:    "existing@example.com",
				password: "Password123!",
			},
			wantErr: service.ErrUsernameTooLong,
		},
		{
			name:  "Short password",
			setup: func(f *fields) {},
			args: args{
				username: "new_user",
				email:    "existing@example.com",
				password: "1234",
			},
			wantErr: service.ErrPasswordTooShort,
		},
		{
			name:  "Insecure password 1",
			setup: func(f *fields) {},
			args: args{
				username: "new_user",
				email:    "existing@example.com",
				password: "a1d2n3i4n",
			},
			wantErr: service.ErrPasswordNoUpper,
		},
		{
			name:  "Insecure password 1",
			setup: func(f *fields) {},
			args: args{
				username: "new_user",
				email:    "existing@example.com",
				password: "ADMIN1234",
			},
			wantErr: service.ErrPasswordNoLower,
		},
		{
			name:  "Insecure password 2",
			setup: func(f *fields) {},
			args: args{
				username: "new_user",
				email:    "existing@example.com",
				password: "AdminHelloYa",
			},
			wantErr: service.ErrPasswordNoNumber,
		},
		{
			name:  "Insecure password 3",
			setup: func(f *fields) {},
			args: args{
				username: "new_user",
				email:    "existing@example.com",
				password: "Admin2023HelloYa",
			},
			wantErr: service.ErrPasswordNoSpecial,
		},
		{
			name: "A user with this username already exists",
			setup: func(f *fields) {
				f.mockRepo.getUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return nil, repository.ErrUserNotFound
				}
				f.mockRepo.getUserByUsernameFunc = func(ctx context.Context, username string) (*models.User, error) {
					return &models.User{Username: username}, nil
				}
			},
			args: args{
				username: "test_user",
				email:    "test@example.com",
				password: "PasSword&123!",
			},
			wantErr: service.ErrUserAlreadyExists,
		},
		{
			name: "A user with this email already exists",
			setup: func(f *fields) {
				f.mockRepo.getUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return &models.User{Email: email}, nil
				}
				f.mockRepo.getUserByUsernameFunc = func(ctx context.Context, username string) (*models.User, error) {
					return nil, repository.ErrUserNotFound
				}
			},
			args: args{
				username: "new_user",
				email:    "existing@example.com",
				password: "PasSword&123!",
			},
			wantErr: service.ErrUserAlreadyExists,
		},
		{
			name: "Successful registration",
			setup: func(f *fields) {
				// Имитируем, что юзер по username не найден (ошибка "not found")
				f.mockRepo.getUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return nil, repository.ErrUserNotFound
				}

				f.mockRepo.getUserByUsernameFunc = func(ctx context.Context, username string) (*models.User, error) {
					return nil, repository.ErrUserNotFound
				}

				// Перехватываем данные в методе Create
				f.mockRepo.createFunc = func(ctx context.Context, user *models.User) error {
					// Проверяем, что поля записались корректно
					if user.Username != "BestUser" {
						t.Errorf("want username 'BestUser', but got '%s'", user.Username)
					}
					if user.Email != "new@example.com" {
						t.Errorf("want email 'new@example.com', but got '%s'", user.Email)
					}

					// originalPassword берем из аргументов теста ("PaSword&123!")
					err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("PaSword&123!"))
					if err != nil {
						t.Errorf("password hash does not match the original password: %v", err)
					}

					return nil
				}
			},
			args: args{
				username: "BestUser",
				email:    "new@example.com",
				password: "PaSword&123!",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fields{
				mockRepo: &mockUserRepo{},
			}

			tt.setup(&f)
			s := service.NewUserService(f.mockRepo, "super-secret")
			err := s.Register(context.Background(), tt.args.username, tt.args.email, tt.args.password)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserService_Login(t *testing.T) {
	type fields struct {
		mockRepo *mockUserRepo
		secret   string
	}
	type args struct {
		username string
		password string
	}

	//Генерируем валидный хэш для успешного сценария
	correctPassword := "PaSsword?-123!"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.DefaultCost)

	// Тестовая структура пользователя
	existingUser := &models.User{
		ID:           42,
		Username:     "active_user",
		Email:        "new_user@example.com",
		PasswordHash: string(hashedPassword),
	}

	tests := []struct {
		name            string
		fields          fields
		args            args
		wantTokenAssert func(t *testing.T, tokenStr string, secret []byte) // Функция для проверки токена
		wantErr         error
	}{
		{
			name: "User not found",
			fields: fields{
				secret: "secret-key",
				mockRepo: &mockUserRepo{
					getUserByUsernameFunc: func(ctx context.Context, username string) (*models.User, error) {
						return nil, repository.ErrUserNotFound
					},
				},
			},
			args: args{
				username: "unknown_user",
				password: correctPassword,
			},
			wantErr: service.ErrInvalidCredentials,
		},
		{
			name: "Incorrect password",
			fields: fields{
				secret: "secret-key",
				mockRepo: &mockUserRepo{
					getUserByUsernameFunc: func(ctx context.Context, username string) (*models.User, error) {
						return existingUser, nil
					},
				},
			},
			args: args{
				username: "active_user",
				password: "WrongPassword!",
			},
			wantErr: service.ErrInvalidCredentials,
		},
		{
			name: "Successful login and JWT generation",
			fields: fields{
				secret: "secret-key",
				mockRepo: &mockUserRepo{
					getUserByUsernameFunc: func(ctx context.Context, username string) (*models.User, error) {
						return existingUser, nil
					},
				},
			},
			args: args{
				username: "active_user",
				password: correctPassword,
			},
			wantErr: nil,
			wantTokenAssert: func(t *testing.T, tokenStr string, secret []byte) {
				// Проверяем, что токен не пустой
				if tokenStr == "" {
					t.Error("expected jwt token, got empty string")
					return
				}

				// Парсим и валидируем созданный JWT-токен
				token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
					return secret, nil
				})

				if err != nil {
					t.Fatalf("failed to parse token: %v", err)
				}

				// Проверяем claims (полезную нагрузку) внутри токена
				if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
					// Проверяем user_id
					userID, ok := claims["user_id"].(float64) // JWT парсит числа как float64
					if !ok || int64(userID) != 42 {
						t.Errorf("want user_id = 42, got %v", claims["user_id"])
					}

					// Проверяем время жизни токена (должно быть в будущем)
					exp, ok := claims["exp"].(float64)
					if !ok || int64(exp) < time.Now().Unix() {
						t.Error("the token 'exp' is invalid or has expired")
					}
				} else {
					t.Error("the token is invalid or there are no claims")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := service.NewUserService(tt.fields.mockRepo, tt.fields.secret)

			gotToken, err := s.Login(context.Background(), tt.args.username, tt.args.password)

			// Проверка ошибки
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Проверка токена
			if tt.wantTokenAssert != nil {
				tt.wantTokenAssert(t, gotToken, []byte(tt.fields.secret))
			}
		})
	}
}
