package dto

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest — входные данные для POST /api/v1/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthUser — публичное представление пользователя (без password_hash).
type AuthUser struct {
	ID        uint64 `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// AuthResponse — ответ на register/login. Токен — JWT.
type AuthResponse struct {
	User  AuthUser `json:"user"`
	Token string   `json:"token"`
}
