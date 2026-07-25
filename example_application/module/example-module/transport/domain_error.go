// Package transport 将 example 模块稳定的领域错误（定义于 service 与 repository）
// 桥接为 HTTP 框架的错误类型。它仅依赖 service 与 repository 导出的错误哨兵值，
// 从而把 HTTP 状态码映射隔离在业务逻辑层之外。
package transport

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/repository"
	"github.com/lamxy/fiberhouse/example_application/module/example-module/service"
)

// MapDomainError 将稳定的用例失败转换为 Fiber 与 Gin 错误处理适配器都能识别的
// HTTP 错误。
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
