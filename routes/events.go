package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"

  "github.com/azvaliev/pigeon/v2/kafka"
)

func EventsRoutes(api fiber.Router, db *sqlx.DB) {
  api.Get("/events", func (c *fiber.Ctx) (error) {
     
  })
}
