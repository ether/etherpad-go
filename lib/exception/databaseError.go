package exception

type DatabaseError struct {
	*AppError
}

func NewDatabaseError(message string, cause error) *DatabaseError {
	return &DatabaseError{
		AppError: &AppError{
			Code:    "DATABASE_ERROR",
			Message: message,
			Cause:   cause,
		},
	}
}
