package server

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"go.uber.org/zap"
)

// recoverMiddleware turns a panic in any HTTP handler (or in a plugin hook it
// runs) into a 500 for that request and logs it, instead of letting it end the
// process. Without it, fiber lets the panic crash the whole server.
func recoverMiddleware(logger *zap.SugaredLogger) fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e any) {
			logger.Errorf("panic while handling %s %s: %v\n%s", c.Method(), c.Path(), e, debug.Stack())
		},
		PanicHandler: func(c fiber.Ctx, r any) error {
			return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprint("internal server error"))
		},
	})
}
