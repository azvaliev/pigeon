package routes

import (
	"fmt"

	"github.com/azvaliev/pigeon/v2/kafka"
	"github.com/azvaliev/pigeon/v2/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

type PostMessageRequest struct {
	ConversationId string `form:"conversation_id" validate:"required,len=26"`
	Message        string `form:"message" validate:"required"`
}

func MessagesRoutes(api fiber.Router, db *sqlx.DB) {
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
}
