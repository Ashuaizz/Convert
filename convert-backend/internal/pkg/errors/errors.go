package errors

type Code string

const (
	CodeOK                   Code = "OK"
	CodeInvalidArgument      Code = "INVALID_ARGUMENT"
	CodeUnauthorized         Code = "UNAUTHORIZED"
	CodeForbidden            Code = "FORBIDDEN"
	CodeNotFound             Code = "NOT_FOUND"
	CodePayloadTooLarge      Code = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMediaType Code = "UNSUPPORTED_MEDIA_TYPE"
	CodeProcessorUnavailable Code = "PROCESSOR_UNAVAILABLE"
	CodeDeadlineExceeded     Code = "DEADLINE_EXCEEDED"
	CodeJobFailed            Code = "JOB_FAILED"
)

type AppError struct {
	Code    Code
	Message string
}

func (e AppError) Error() string {
	return string(e.Code) + ": " + e.Message
}

func New(code Code, message string) AppError {
	return AppError{Code: code, Message: message}
}
