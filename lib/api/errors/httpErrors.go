package errors

var InternalApiError = Error{
	Message: "Internal API Error",
	Error:   1,
}

var InvalidRevisionError = Error{
	Message: "Invalid revision number",
	Error:   400,
}

var RevisionHigherThanHeadError = Error{
	Message: "Revision number is higher than head",
	Error:   400,
}

var InvalidRequestError = Error{
	Message: "Invalid request",
	Error:   400,
}

func NewInvalidParamError(paramName string) Error {
	return Error{
		Message: "Invalid parameter: " + paramName,
		Error:   400,
	}
}

func NewMissingParamError(paramName string) Error {
	return Error{
		Message: "Missing parameter: " + paramName,
		Error:   400,
	}
}

var PadNotFoundError = Error{
	Message: "Pad not found",
	Error:   404,
}

var AuthorNotFoundError = Error{
	Message: "Author not found",
	Error:   404,
}

var RevisionNotFoundError = Error{
	Message: "Revision not found",
	Error:   404,
}

var InternalServerError = Error{
	Message: "Internal server error",
	Error:   500,
}

var TextConversionError = Error{
	Message: "Failed to convert text",
	Error:   500,
}

var DataRetrievalError = Error{
	Message: "Failed to retrieve data",
	Error:   500,
}

var UnauthorizedError = Error{
	Message: "Unauthorized access",
	Error:   401,
}

var ForbiddenError = Error{
	Message: "Access forbidden",
	Error:   403,
}

var PadAlreadyExistsError = Error{
	Message: "Pad already exists",
	Error:   409,
}

var ValidationError = Error{
	Message: "Validation failed",
	Error:   422,
}

var InvalidParameterError = Error{
	Message: "Invalid parameter provided",
	Error:   422,
}
