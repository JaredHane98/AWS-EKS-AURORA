package employeerds

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

const QUERY_EMPLOYEE_EXISTS = "SELECT EXISTS(SELECT 1 from %s WHERE id = $1);"

func (h *RDSHandle) CheckUserExists(c *fiber.Ctx) error {

	customContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "fail",
			"message": "User ID is required",
		})
	}

	sqlString := fmt.Sprintf(QUERY_EMPLOYEE_EXISTS, h.TableName)

	var found bool
	err := h.PostgresConn.QueryRow(customContext, sqlString, userID).Scan(&found)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	if !found {
		return c.Status(fiber.StatusNotFound).JSON(&fiber.Map{
			"status":  "fail",
			"message": "employee not found.",
		})
	}

	c.Locals("id", userID)
	return c.Next()
}

func (h *RDSHandle) HealthCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(&fiber.Map{
		"status":  "success",
		"message": "OK",
	})
}

func (h *RDSHandle) ReadinessCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(&fiber.Map{
		"status":  "success",
		"message": "READY",
	})
}
