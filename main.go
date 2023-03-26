package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/azvaliev/pigeon/v2/kafka"
	"github.com/azvaliev/pigeon/v2/routes"
	"github.com/azvaliev/pigeon/v2/utils"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/template/mustache"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/oklog/ulid/v2"
)

type Message struct {
	Id             string    `db:"id"`
	SenderId       string    `db:"sender_id"`
	ConversationId string    `db:"conversation_id"`
	Message        string    `db:"message"`
	CreatedAt      time.Time `db:"created_at"`
}

type Conversation struct {
	Id string `db:"id"`
}

type ConversationMember struct {
	ConversationId string `db:"conversation_id"`
	UserId         string `db:"user_id"`
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

	app.Use(compress.New())

	routes.AuthRoutes(app.Group("/auth"), db)

	app.Get("/api/users", func(c *fiber.Ctx) error {
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

	// Authenticate user and set user data to locals
	app.Use(func(c *fiber.Ctx) error {
		jwtCookie := c.Cookies(utils.AUTH_TOKEN_JWT_COOKIE_NAME)
		if jwtCookie == "" {
			return c.Redirect("/auth", fiber.StatusSeeOther)
		}

		user, err := utils.VerifyJWTCookie(jwtCookie)
		if err != nil {
			return c.Redirect("/auth", fiber.StatusSeeOther)
		}

		c.Locals("user-id", user.Id)
		c.Locals("user-email", user.Email)
		c.Locals("user-username", user.Username)
		c.Locals("user-display-name", user.DisplayName)
		c.Locals("user-avatar", user.Avatar)

		return c.Next()
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Send([]byte("Home Page"))
	})

	api := app.Group("/api")

	type PostMessageRequest struct {
		ConversationId string `form:"conversation_id" validate:"required,len=26"`
		Message        string `form:"message" validate:"required"`
	}

	api.Post("/messages", func(c *fiber.Ctx) error {
		c.Accepts("multipart/form-data")
		userId := c.Locals("user-id").(string)

		// Create post message request from form data
		postMessageRequest := &PostMessageRequest{}
		if err := c.BodyParser(postMessageRequest); err != nil {
			return c.Status(fiber.StatusBadRequest).Send([]byte("Could not parse message"))
		}

		// Validate post message request
		if err := utils.ValidateStruct(postMessageRequest); err != nil {
			return c.Status(fiber.StatusBadRequest).Send([]byte("Invalid Message"))
		}

		// Check if conversation exists and the user is a part of it
		isPartOfConversation := 0
		err := db.Get(
			&isPartOfConversation,
			"SELECT COUNT(*) FROM ConversationMember WHERE conversation_id = ? AND user_id = ?",
			postMessageRequest.ConversationId,
			userId,
		)

		if err != nil {
			fmt.Printf("Failed to check if user is part of conversation: %s\n", err)
			return c.Status(fiber.StatusInternalServerError).Send([]byte("An unknown error occured. Please try again later"))
		}

		if isPartOfConversation == 0 {
			return c.Status(fiber.StatusForbidden).Send([]byte("This conversation either does not exist or you are not a part of it"))
		}

		messageId := ulid.Make().String()

		// Create message
		_, err = db.Exec(
			"INSERT INTO Message (id, sender_id, conversation_id, message) VALUES (?, ?, ?, ?)",
			messageId,
			userId,
			postMessageRequest.ConversationId,
			postMessageRequest.Message,
		)

		if err != nil {
			fmt.Printf("Failed to create message: %s\n", err)
			return c.Status(fiber.StatusInternalServerError).Send([]byte("Failed to send message. Please try again later"))
		}

		// Send message to Kafka
		producer := kafkahelpers.CreateProducer(postMessageRequest.ConversationId)
		err = kafkahelpers.PostMessage(
			producer,
			&kafkahelpers.Message{
				From:    userId,
				To:      postMessageRequest.ConversationId,
				Message: postMessageRequest.Message,
			},
		)

		// Rollback message creation if cannot send to Kafka
		if err != nil {
			_, err = db.Exec(
				"DELETE FROM Message WHERE id = ?",
				messageId,
			)
			return c.Status(fiber.StatusInternalServerError).Send([]byte("Failed to send message. Please try again later"))
		}

		return c.Status(201).Render("app/message", fiber.Map{
			"content": postMessageRequest.Message,
			"sender":  c.Locals("user-display-name"),
		})
	})

	log.Fatal(app.Listen(":4872"))
}
