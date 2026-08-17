package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo      repository.UserRepository
	jwtSecret []byte
}

// Создание нового сервиса пользователей
func NewUserService(repo repository.UserRepository, secret string) *UserService {
	return &UserService{repo: repo, jwtSecret: []byte(secret)}
}

// Объявляем ошибки
var (
	ErrEmailEmpty         = errors.New("email cannot be empty")
	ErrEmailInvalid       = errors.New("invalid email format")
	ErrUserAlreadyExists  = errors.New("Username or email already exists")
	ErrUsernameEmpty      = errors.New("username cannot be empty")
	ErrUsernameTooShort   = errors.New("username must be at least 3 characters long")
	ErrUsernameTooLong    = errors.New("username cannot be longer than 30 characters")
	ErrUsernameInvalid    = errors.New("username can only contain alphanumeric characters, underscores, and hyphens")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters long")
	ErrPasswordNoUpper    = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLower    = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoNumber   = errors.New("password must contain at least one number")
	ErrPasswordNoSpecial  = errors.New("password must contain at least one special character")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// регулярные выражения для проверки username, email
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Регистрация нового пользователя
func (s *UserService) Register(ctx context.Context, username, email, password string) error {
	if err := ValidateEmail(email); err != nil {
		return err
	}
	if err := ValidateUsername(username); err != nil {
		return err
	}

	if err := ValidatePassword(password); err != nil {
		return err
	}

	_, err := s.repo.GetUserByUsername(ctx, username)
	if err == nil {
		return ErrUserAlreadyExists
	}

	_, err = s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		return ErrUserAlreadyExists
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hashed),
	}

	return s.repo.Create(ctx, user)
}

// Авторизация пользователя
func (s *UserService) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString(s.jwtSecret)
}

// Валидация имени пользователя
func ValidateUsername(username string) error {
	if username == "" {
		return ErrUsernameEmpty
	}
	if len(username) < 3 {
		return ErrUsernameTooShort
	}
	if len(username) > 30 {
		return ErrUsernameTooLong
	}

	if !usernameRegex.MatchString(username) {
		return ErrUsernameInvalid
	}

	return nil
}

// Валидация электронной почты
func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmailEmpty
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return ErrEmailInvalid
	}
	addr := parsed.Address
	parts := strings.Split(addr, "@")
	if len(parts) != 2 {
		return ErrEmailInvalid
	}
	domain := parts[1]
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return ErrEmailInvalid
	}
	return nil
}

// Валидация пароля
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	// Проходим по каждому символу в пароле
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		// Проверяем на спецсимволы
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUpper
	}
	if !hasLower {
		return ErrPasswordNoLower
	}
	if !hasNumber {
		return ErrPasswordNoNumber
	}
	if !hasSpecial {
		return ErrPasswordNoSpecial
	}

	return nil
}
