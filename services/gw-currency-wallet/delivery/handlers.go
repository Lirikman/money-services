package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	"github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	svc *service.WalletService
	log *slog.Logger
}

func NewHandler(svc *service.WalletService, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

type authReq struct {
	Username string `json:"username" example:"ivan_ivanov"`
	Password string `json:"password" example:"SeCrEtS-New902!"`
	Email    string `json:"email" example:"ivani_iv123@ya.ru"`
}

type RegisterResponse struct {
	Message string `json:"message" example:"User registered successfully"`
}

var ErrUserAlreadyExists = errors.New("username or email already exists")

// Register godoc
// @Summary      Create a new user
// @Description  Creates a new user account using name, email, and password.
// @Accept       json
// @Produce      json
// @Param        request  body      authReq  true     "Registration details"
// @Success      201      {object}  RegisterResponse  "Successful registration"
// @Failure      400      {object}  ErrorResponse     "Invalid data format or user already exists"
// @Failure      500      {object}  ErrorResponse     "Internal Server Error"
// @Router       /register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 100*1024)

	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode registration request",
			slog.String("err", err.Error()),
			slog.String("remote_addr", r.RemoteAddr),
		)
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.log.Info("registering new user", slog.String("username", req.Username))

	if _, err := h.svc.Register(r.Context(), req.Username, req.Email, req.Password); err != nil {
		if pgErr, ok := errors.AsType[*pq.Error](err); ok {
			switch pgErr.Code {
			case "23505":
				h.log.Warn("registration rejected: user already exists",
					slog.String("username", req.Username),
					slog.String("email", req.Email),
				)
				responseError(w, http.StatusBadRequest, "Username or email already exists")
				return
			case "23514":
				h.log.Warn("registration rejected: Incorrect data entered",
					slog.String("username", req.Username),
				)
				responseError(w, http.StatusBadRequest, "Incorrect data entered")
				return
			}
		}

		h.log.Error("failed to register user",
			slog.String("username", req.Username),
			slog.String("err", err.Error()),
		)
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.log.Info("user registered successfully", slog.String("username", req.Username))

	responseJSON(w, http.StatusCreated, RegisterResponse{
		Message: "User registered successfully",
	})
}

type LoginResponse struct {
	Token string `json:"token" example:"JWT_TOKEN"`
}

// Login Godoc
// @Summary      User authorization
// @Description  Authenticates a user with a username and password, returning a JWT token.
// @Accept       json
// @Produce      json
// @Param        request  body      authReq  true  "Login details"
// @Success      200      {object}  LoginResponse "Successful authorization (returns token)"
// @Failure      400      {object}  ErrorResponse "Invalid request body format"
// @Failure      401      {object}  ErrorResponse "Incorrect username or password"
// @Router       /login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req authReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode login request",
			slog.String("err", err.Error()),
			slog.String("remote_addr", r.RemoteAddr),
		)
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.log.Info("user attempting login", slog.String("username", req.Username))

	token, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		h.log.Warn("unauthorized login attempt",
			slog.String("username", req.Username),
			slog.String("err", err.Error()),
		)
		responseError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	h.log.Info("user logged in successfully", slog.String("username", req.Username))

	responseJSON(w, http.StatusOK, LoginResponse{
		Token: token,
	})
}

type BalanceResponse struct {
	Balance map[string]string `json:"balance" example:"EUR:100.00,RUB:3000.00,USD:500.00"`
}

// Balance Godoc
// @Summary      Getting a user's balance
// @Description  Returns the current balance of the user based on their ID from the authorization token.
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true     "Authorization token (Bearer <token>)"
// @Success      200            {object}  BalanceResponse  "Successful balance receipt"
// @Failure      400            {object}  ErrorResponse    "Invalid request or missing user ID"
// @Failure      500            {object}  ErrorResponse    "Internal Server Error"
// @Router       /balance [get]
func (h *Handler) Balance(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		h.log.Error("userID missing or invalid type in context")
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.log.Info("processing get balance attempt",
		slog.Int64("username", userID),
	)

	wall, err := h.svc.GetBalances(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get balances",
			slog.Int64("user_id", userID),
			slog.String("err", err.Error()),
		)
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseJSON(w, http.StatusOK, BalanceResponse{
		Balance: wall,
	})
}

type walletReq struct {
	Currency string  `json:"currency" example:"USD"`
	Amount   float64 `json:"amount" example:"3500"`
}

type DeptWithResponse struct {
	Message    string            `json:"message" example:"deposit successful or withdrawal successful"`
	NewBalance map[string]string `json:"new_balance" example:"EUR:100.00,RUB:3000.00,USD:500.00"`
}

// Deposit Godoc
// @Summary      Balance replenishment
// @Description  Tops up the user's balance with the specified amount in the selected currency (supports: USD, EUR, RUB).
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string          true  "Authorization token (Bearer <token>)"
// @Param        request        body      walletReq       true  "Details for replenishment (amount and currency)"
// @Success      200            {object}  DeptWithResponse      "Balance has been successfully replenished"
// @Failure      400            {object}  ErrorResponse         "Invalid request format, negative amount, or invalid currency"
// @Failure      500            {object}  ErrorResponse         "Internal Server Error"
// @Router       /wallet/deposit [post]
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		h.log.Error("userID missing or invalid type in context")
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	var req walletReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode deposit request",
			slog.Int64("user_id", userID),
			slog.Any("err", err),
		)
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Amount <= 0 {
		h.log.Warn("incorrect top-up amount",
			slog.Int64("user_id", userID),
			slog.Float64("amount", req.Amount),
		)
		responseError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	h.log.Info("processing deposit attempt",
		slog.Int64("user_id", userID),
		slog.Float64("amount", req.Amount),
		slog.String("currency", req.Currency),
	)

	if err := h.svc.Deposit(r.Context(), userID, req.Currency, req.Amount); err != nil {
		h.log.Error("deposit transaction failed",
			slog.Int64("user_id", userID),
			slog.Float64("amount", req.Amount),
			slog.String("currency", req.Currency),
			slog.Any("err", err),
		)
		responseError(w, http.StatusBadRequest, "Invalid amount or currency")
		return
	}

	h.log.Info("deposit successful",
		slog.Int64("user_id", userID),
		slog.Float64("amount", req.Amount),
		slog.String("currency", req.Currency),
	)

	newBalance, err := h.svc.GetBalances(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get wallet balance after deposit",
			slog.Int64("user_id", userID),
			slog.Any("err", err),
		)
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseJSON(w, http.StatusOK, DeptWithResponse{
		Message:    "Account topped up successfully",
		NewBalance: newBalance,
	})
}

// Withdraw Godoc
// @Summary      Withdrawal of funds
// @Description  Writes off the specified amount in the selected currency from the user's balance
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string          true  "Authorization token (Bearer <token>)"
// @Param        request        body      walletReq       true  "Withdrawal details (amount and currency)"
// @Success      200            {object}  DeptWithResponse      "Funds have been successfully withdrawn"
// @Failure      400            {object}  ErrorResponse         "Invalid request format, negative amount, invalid currency, or insufficient funds"
// @Failure      500            {object}  ErrorResponse         "Internal Server Error"
// @Router       /wallet/withdraw [post]
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		h.log.Error("userID missing or invalid type in context")
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	var req walletReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode withdraw request",
			slog.Int64("user_id", userID),
			slog.String("err", err.Error()),
		)
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Amount <= 0 {
		h.log.Warn("incorrect write-off amount",
			slog.Int64("user_id", userID),
			slog.Float64("amount", req.Amount),
		)
		responseError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	h.log.Info("processing withdraw attempt",
		slog.Int64("user_id", userID),
		slog.Float64("amount", req.Amount),
		slog.String("currency", req.Currency),
	)

	if err := h.svc.Withdraw(r.Context(), userID, req.Currency, req.Amount); err != nil {
		h.log.Warn("withdraw transaction rejected",
			slog.Int64("user_id", userID),
			slog.Float64("amount", req.Amount),
			slog.String("currency", req.Currency),
			slog.String("err", err.Error()),
		)
		responseError(w, http.StatusBadRequest, "Insufficient funds or invalid amount")
		return
	}

	h.log.Info("withdraw successful",
		slog.Int64("user_id", userID),
		slog.Float64("amount", req.Amount),
		slog.String("currency", req.Currency),
	)

	newBalance, err := h.svc.GetBalances(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get wallet balance after withdraw",
			slog.Int64("user_id", userID),
			slog.Any("err", err),
		)
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseJSON(w, http.StatusOK, DeptWithResponse{
		Message:    "Withdrawal successful",
		NewBalance: newBalance,
	})
}

type exchangeReq struct {
	FromCurrency string  `json:"from_currency" example:"USD"`
	ToCurrency   string  `json:"to_currency" example:"RUB"`
	Amount       float64 `json:"amount" example:"300.00"`
}

type ExchangeResponse struct {
	Message         string            `json:"message" example:"currency exchange successful"`
	ExchangedAmount float64           `json:"exchanged_amount" example:"300"`
	NewBalance      map[string]string `json:"new_balance" example:"EUR:100.00,RUB:3000.00,USD:500.00"`
}

// ExchangeCurrency Godoc
// @Summary      Currency exchange
// @Description  Converts the specified amount from one currency to another within the user's wallet
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string          true  "Authorization token (Bearer <token>)"
// @Param        request        body      exchangeReq     true  "Exchange details (amount, from which currency, to which)"
// @Success      200            {object}  ExchangeResponse      "The exchange was successfully completed"
// @Failure      400            {object}  ErrorResponse         "Invalid request format, invalid currencies, or insufficient funds"
// @Failure      500            {object}  ErrorResponse         "Internal Server Error"
// @Router       /exchange [post]
func (h *Handler) ExchangeCurrency(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		h.log.Error("userID missing or invalid type in context")
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	var req exchangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode exchange request",
			slog.Int64("user_id", userID),
			slog.String("err", err.Error()),
		)
		responseError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.log.Info("processing currency exchange attempt",
		slog.Int64("user_id", userID),
		slog.Float64("amount", req.Amount),
		slog.String("from_currency", req.FromCurrency),
		slog.String("to_currency", req.ToCurrency),
	)

	if req.Amount <= 0 {
		h.log.Warn("incorrect exchange amount",
			slog.Int64("user_id", userID),
			slog.Float64("amount", req.Amount),
		)
		responseError(w, http.StatusBadRequest, "Amount must be positive")
		return
	}

	if err := h.svc.Exchange(r.Context(), userID, req.FromCurrency, req.ToCurrency, req.Amount); err != nil {
		h.log.Error("currency exchange failed",
			slog.Int64("user_id", userID),
			slog.Float64("amount", req.Amount),
			slog.String("from_currency", req.FromCurrency),
			slog.String("to_currency", req.ToCurrency),
			slog.String("err", err.Error()),
		)

		// Проверяем, является ли ошибка ошибкой gRPC
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			// Сервер недоступен
			case codes.Unavailable:
				responseError(w, http.StatusServiceUnavailable, "Currency service is temporarily unavailable")
				return

			// Превышен таймаут выполнения запроса
			case codes.DeadlineExceeded:
				responseError(w, http.StatusGatewayTimeout, "Currency service timeout")
				return

			// Некорректные аргументы
			case codes.InvalidArgument:
				responseError(w, http.StatusBadRequest, "Insufficient funds or invalid currencies")
				return
			}
		}
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.log.Info("currency exchange successful",
		slog.Int64("user_id", userID),
		slog.Float64("amount", req.Amount),
		slog.String("from_currency", req.FromCurrency),
		slog.String("to_currency", req.ToCurrency),
	)

	newBalance, err := h.svc.GetBalances(r.Context(), userID)
	if err != nil {
		h.log.Error("failed to get wallet balance after exchange",
			slog.Int64("user_id", userID),
			slog.Any("err", err),
		)
		responseError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseJSON(w, http.StatusOK, ExchangeResponse{
		Message:         "Exchange successful",
		ExchangedAmount: req.Amount,
		NewBalance:      newBalance,
	})
}

type GetRateResponse struct {
	Rates map[string]float32 `json:"rates" example:"RUB/USD:0.1,EUR/USD:0.12,USD/RUB:1.12,USD/EUR:0.02"`
}

// GetRates Godoc
// @Summary      Obtaining exchange rates
// @Description  Returns current currency exchange rates
// @Produce      json
// @Param        Authorization  header    string  true  "Authorization token (Bearer <token>)"
// @Success      200  {object}  GetRateResponse   "Successfully receiving exchange rates"
// @Failure      500  {object}  ErrorResponse  	  "Internal Server Error"
// @Router       /exchange/rates [get]
func (h *Handler) GetRates(w http.ResponseWriter, r *http.Request) {
	rate, err := h.svc.GetRates(r.Context(), &pb.Empty{})
	if err != nil {
		h.log.Error("failed to get currency rates",
			slog.String("err", err.Error()),
		)
		// Проверяем, является ли ошибка ошибкой gRPC
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			// Сервер недоступен
			case codes.Unavailable:
				responseError(w, http.StatusServiceUnavailable, "Currency service is temporarily unavailable")
				return

			// Превышен таймаут выполнения запроса
			case codes.DeadlineExceeded:
				responseError(w, http.StatusGatewayTimeout, "Currency service timeout")
				return
			}
		}
		responseError(w, http.StatusInternalServerError, "Failed to retrieve exchange rates")
		return
	}

	responseJSON(w, http.StatusOK, GetRateResponse{Rates: rate.GetRates()})
}

func responseJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

type ErrorResponse struct {
	Error string `json:"error" example:"operation failed"`
}

func responseError(w http.ResponseWriter, status int, msg string) {
	responseJSON(w, status, ErrorResponse{Error: msg})
}
