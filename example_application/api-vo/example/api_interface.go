package example

import (
	"github.com/gofiber/fiber/v2"
)

// HealthApiIFace 健康检查
type HealthApiIFace interface {
	Liveness(c *fiber.Ctx) error
}
