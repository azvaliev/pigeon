package routes

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/azvaliev/pigeon/v2/kafka"
	"github.com/azvaliev/pigeon/v2/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

type MessageData struct {
	ConversationId string `json:"conversation_id" validate:"required,len=26"`
	Message        string `json:"message" validate:"required"`
}

func EventsRoutes(api fiber.Router, db *sqlx.DB) {
	api.Use("/events", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}

		return fiber.ErrUpgradeRequired
	})

	api.Get("/events", websocket.New(func(c *websocket.Conn) {
		// Determine all conversations user is part of
		conversations := &[]utils.Conversation{}
		err := db.Select(
			conversations,
			"SELECT conversation_id as id FROM ConversationMember WHERE user_id = ?",
			c.Locals("user-id"),
		)

		if err != nil {
			c.Close()
			return
		}

		// Create context for recieving messages to write
		messageContext, cancelMessageContext := context.WithCancel(context.Background())
		go func() {
			defer cancelMessageContext()

			for {
				select {
				case <-messageContext.Done():
					return
				default:

					messageType, message, err := c.ReadMessage()
					if err != nil {
						cancelMessageContext()
						return
					}

					if messageType == websocket.TextMessage {
						// Parse message into JSON
						messageData := &MessageData{}
						err = json.Unmarshal(message, messageData)
						if err != nil {
							log.Printf("Error reading message \"%s\" - error %s\n", string(message), err)
							continue
						}

						// Validate message
						validationErrors := utils.ValidateStruct(messageData)
						if validationErrors != nil {
							continue
						}

						// Check if conversation exists and the user is a part of it
						isPartOfConversation := 0
						err = db.Get(
							&isPartOfConversation,
							"SELECT COUNT(*) FROM ConversationMember WHERE conversation_id = ? AND user_id = ?",
							messageData.ConversationId,
							c.Locals("user-id"),
						)

						if err != nil || isPartOfConversation == 0 {
							log.Printf("User %s is not part of conversation %s\n", c.Locals("user-id"), messageData.ConversationId)
							continue
						}

						messageId := ulid.Make().String()

						// Create message
						_, err = db.Exec(
							"INSERT INTO Message (id, sender_id, conversation_id, message) VALUES (?, ?, ?, ?)",
							messageId,
							c.Locals("user-id"),
							messageData.ConversationId,
							messageData.Message,
						)

						if err != nil {
							log.Printf("Failed to create message: %s\n", err)
							continue
						}

						// Create a kafka producer
						producer := kafkahelpers.CreateProducer(messageData.ConversationId)

						// Send message to kafka
						err = kafkahelpers.PostMessage(producer, messageContext, &kafkahelpers.Message{
							From:    c.Locals("user-id").(string),
							To:      messageData.ConversationId,
							Message: messageData.Message,
						})

						producer.Close()

						// Rollback message creation if failed to send to kafka
						if err != nil {
							_, err = db.Exec(
								"DELETE FROM Message WHERE id = ?",
								messageId,
							)
							continue
						}
					}
				}
			}
		}()

		// Create context for all conversation listening
		conversationGroup := &sync.WaitGroup{}
		conversationGroup.Add(len(*conversations))
		conversationContext, cancelConversationContext := context.WithCancel(context.Background())

		type ConversationContextError struct {
			err error
			mu  sync.Mutex
		}
		conversationContextError := &ConversationContextError{err: nil}

		defer cancelConversationContext()

		// Listen to all conversations
		for _, conversation := range *conversations {
			go func(conversationId string) {
				defer conversationGroup.Done()

				// Create consumer for conversation
				consumer, err := kafkahelpers.CreateConsumer(conversationId)
				if err != nil {
					conversationContextError.mu.Lock()
					conversationContextError.err = err
					conversationContextError.mu.Unlock()
					cancelConversationContext()
					return
				}

				// Process conversation messages
				err = kafkahelpers.ProcessMessages(conversationId, consumer, conversationContext, func(message *kafkahelpers.Message) error {
					err := c.WriteMessage(websocket.TextMessage, []byte(message.Message))
					if err != nil {
						return err
					}

					return nil
				})

				// If error occurred, cancel context and store error
				if err != nil {
					conversationContextError.mu.Lock()
					conversationContextError.err = err
					conversationContextError.mu.Unlock()
					cancelConversationContext()
				}

				return
			}(conversation.Id)
		}

		select {
		case <-conversationContext.Done():
		}

		// If error occurred, log it
		if conversationContextError.err != nil {
			log.Printf("Error while listening to conversation: %s\n", conversationContextError.err)
		}

		// Stop recieving messages
		cancelMessageContext()

		// Wait for all conversation listening to finish
		conversationGroup.Wait()

		c.Close()
		return
	}))
}
