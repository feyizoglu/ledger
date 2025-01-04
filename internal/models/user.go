package models

type User struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

type CreateUserRequest struct {
	Name string `json:"name"`
}

type UserBalance struct {
	UserID  int64   `json:"user_id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

type AddCreditRequest struct {
	Amount float64 `json:"amount"`
}
