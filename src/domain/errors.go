package domain

import "fmt"

type Code string

const (
	CodeInvalidRequest   Code = "invalid_request"
	CodeNotFound         Code = "not_found"
	CodeInsufficient     Code = "insufficient_balance"
	CodeRouteUnavailable Code = "route_unavailable"
	CodeLimitExceeded    Code = "limit_exceeded"
	CodeQueueEmpty       Code = "queue_empty"
	CodeConflict         Code = "conflict"
	CodeInternal         Code = "internal"
)

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code Code, message string) Error {
	return Error{Code: code, Message: message}
}

func Invalid(message string) Error {
	return NewError(CodeInvalidRequest, message)
}

func NotFound(message string) Error {
	return NewError(CodeNotFound, message)
}

func Insufficient(message string) Error {
	return NewError(CodeInsufficient, message)
}

func RouteUnavailable(message string) Error {
	return NewError(CodeRouteUnavailable, message)
}

func LimitExceeded(message string) Error {
	return NewError(CodeLimitExceeded, message)
}

func QueueEmpty(message string) Error {
	return NewError(CodeQueueEmpty, message)
}

func Conflict(message string) Error {
	return NewError(CodeConflict, message)
}

func Internal(message string) Error {
	return NewError(CodeInternal, message)
}

func AsDomainError(err error) (Error, bool) {
	if err == nil {
		return Error{}, false
	}
	if converted, ok := err.(Error); ok {
		return converted, true
	}
	return Error{}, false
}
