package server

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecoverMiddlewareTurnsAPanicInto500(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	app := fiber.New()
	app.Use(recoverMiddleware(zap.New(core).Sugar()))
	app.Get("/boom", func(fiber.Ctx) error { panic("secret detail") })
	app.Get("/fine", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest("GET", "/boom", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(body), "secret detail", "the panic value is not sent to the client")
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].Message, "secret detail")

	// The server keeps serving.
	resp, err = app.Test(httptest.NewRequest("GET", "/fine", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
