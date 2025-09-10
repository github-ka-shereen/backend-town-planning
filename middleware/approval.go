package middleware

// import (
// 	"town-planning-backend/db/models"

// 	"github.com/gofiber/fiber/v2"
// )

// func RequireApprovalLevel(requiredLevel models.ApprovalLevel) fiber.Handler {
// 	return func(c *fiber.Ctx) error {
// 		user := c.Locals("user").(models.User)

// 		if !user.Role.CanApprove(requiredLevel) {
// 			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
// 				"error":   "Forbidden",
// 				"message": "Your role cannot approve at this level",
// 				"details": fiber.Map{
// 					"required_level": requiredLevel,
// 					"user_role":      user.Role,
// 				},
// 			})
// 		}

// 		return c.Next()
// 	}
// }
