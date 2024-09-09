package main

import (
	"fmt"
	"log"
	employeerds "main/internal"

	"github.com/gofiber/fiber/v2"
)

func main() {

	log.Printf("A pod has been created")

	RDSHandle := employeerds.NewRDSHandleMust()

	defer RDSHandle.PostgresConn.Close()

	app := fiber.New()

	app.Get("/health", RDSHandle.HealthCheck)
	app.Get("/readiness", RDSHandle.ReadinessCheck)

	app.Post("/employee", RDSHandle.CreateEmployee)
	app.Get("/employee/:id", RDSHandle.GetEmployee)
	app.Put("/employee/:id", RDSHandle.CheckUserExists, RDSHandle.UpdateEmployee)
	app.Delete("/employee/:id", RDSHandle.CheckUserExists, RDSHandle.RemoveEmployee)

	app.All("*", func(c *fiber.Ctx) error {
		errorMessage := fmt.Sprintf("Route '%s' does not exist", c.OriginalURL())

		return c.Status(fiber.StatusNotFound).JSON(&fiber.Map{
			"status":  "fail",
			"message": errorMessage,
		})
	})

	log.Fatal(app.Listen(":8080"))
}
