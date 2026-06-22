package dto

type Envelope struct {
	Data  any        `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Коды ошибок API. Используются как в ErrorBody.Code, так и в service.ErrXxx.
const (
	CodeValidation   = "validation_error"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeInternal     = "internal_error"
	CodeRateLimited  = "rate_limited"
	CodeBadRequest   = "bad_request"
)

type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

func NewData(data any) Envelope {
	return Envelope{Data: data}
}

func NewError(code, message string) Envelope {
	return Envelope{Error: &ErrorBody{Code: code, Message: message}}
}
