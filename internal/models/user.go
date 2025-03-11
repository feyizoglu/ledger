package models

type User struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Role    string  `json:"role"`
	Balance float64 `json:"balance"`
}

type CreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UserBalance struct {
	UserID  int64   `json:"user_id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

type AddCreditRequest struct {
	Amount float64 `json:"amount"`
}

type TransferRequest struct {
	FromUserID int64   `json:"from_user_id"`
	ToUserID   int64   `json:"to_user_id"`
	Amount     float64 `json:"amount"`
}

type WithdrawRequest struct {
	Amount float64 `json:"amount"`
}
