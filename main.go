package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/azvaliev/pigeon/v2/utils"
	"github.com/azvaliev/pigeon/v2/routes"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/mustache"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

type Message struct {
	Id          string    `db:"id"`
	SenderId    string    `db:"sender_id"`
	RecipientId string    `db:"recipient_id"`
	Message     string    `db:"message"`
	CreatedAt   time.Time `db:"created_at"`
}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("error loading .env file")
	}

	sqlConnectionString := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s",
		os.Getenv("MYSQL_USER"),
		os.Getenv("MYSQL_PASSWORD"),
		os.Getenv("MYSQL_HOST"),
		os.Getenv("MYSQL_DATABASE"),
	)
	db := sqlx.MustConnect(
		"mysql",
		sqlConnectionString,
	)

	err = db.Ping()
	if err != nil {
		log.Fatalln(err)
	}

	engine := mustache.New("./views", ".mustache")

	app := fiber.New(fiber.Config{
		EnablePrintRoutes: true,
		Views:             engine,
	})

	routes.AuthRoutes(app.Group("/auth"), db)

  app.Get("/", func(c *fiber.Ctx) error {
    return c.Send([]byte("Home Page"))
  })

	api := app.Group("/api")

	api.Get("/users", func(c *fiber.Ctx) error {
		baseStatement := "SELECT id, email, username, display_name, avatar FROM User"
		baseParam := ""

		if c.Query("id") != "" {
			baseStatement += " WHERE id = ?"
			baseParam = c.Query("id")
		} else if c.Query("email") != "" {
			baseStatement += " WHERE email = ?"
			baseParam = c.Query("email")
		} else if c.Query("username") != "" {
			baseStatement += " WHERE username = ?"
			baseParam = c.Query("username")
		}

		if baseParam == "" {
			return c.Status(fiber.StatusBadRequest).Send([]byte("No query parameters provided"))
		}

		user := &utils.User{}
		err := db.Get(user, baseStatement, baseParam)

		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).Send([]byte("User not found"))
		}

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(err)
		}

		return c.JSON(user)
	})

	log.Fatal(app.Listen(":4872"))
}

