// Package transport bridges the example module's stable domain errors
// (defined in service and repository) to HTTP-framework error types. It
// depends on service and repository only for their exported error
// sentinels, keeping HTTP status-code mapping out of the business-logic
// layers.
package transport

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
)

// MapDomainError converts stable use-case failures into the HTTP errors
// understood by both Fiber and Gin error-handler adapters.
func MapDomainError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		return fiber.NewError(http.StatusBadRequest, service.ErrInvalidInput.Error())
	case errors.Is(err, repository.ErrInvalidID):
		return fiber.NewError(http.StatusBadRequest, repository.ErrInvalidID.Error())
	case errors.Is(err, repository.ErrNotFound):
		return fiber.NewError(http.StatusNotFound, repository.ErrNotFound.Error())
	case errors.Is(err, repository.ErrConflict):
		return fiber.NewError(http.StatusConflict, repository.ErrConflict.Error())
	case errors.Is(err, repository.ErrUnchanged):
		return fiber.NewError(http.StatusConflict, repository.ErrUnchanged.Error())
	default:
		return err
	}
}
