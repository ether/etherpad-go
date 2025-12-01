package exception

import "fmt"

type PadNotFoundError struct {
	*AppError
	PadId string
}

func NewPadNotFoundError(padId string) *PadNotFoundError {
	return &PadNotFoundError{
		AppError: &AppError{
			Code:    "PAD_NOT_FOUND",
			Message: fmt.Sprintf("pad with id '%s' does not exist", padId),
		},
		PadId: padId,
	}
}
