package routes

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"

	"github.com/azvaliev/pigeon/v2/kafka"
	"github.com/azvaliev/pigeon/v2/utils"
)

func EventsRoutes(api fiber.Router, db *sqlx.DB) {
	api.Get("/events", func(c *fiber.Ctx) error {
		// Determine all conversations user is part of
		conversations := &[]utils.Conversation{}
		err := db.Select(conversations, "SELECT conversation_id as id FROM ConversationMember WHERE user_id = ?", c.Locals("user-id"))

		if err != nil {
			return c.Status(500).Send(nil)
		}

		// Send SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		conversationGroup := &sync.WaitGroup{}
		conversationGroup.Add(len(*conversations))
		conversationContext, cancelConversationContext := context.WithCancel(context.Background())

		type ConversationContextError struct {
			err error
			mu  sync.Mutex
		}
		conversationContextError := &ConversationContextError{err: nil}

		defer cancelConversationContext()

		// Subscribe to all conversations
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
				err = kafkahelpers.ProcessMessages(conversationId, consumer, conversationContext, func(message *kafkahelpers.Message) bool {
					formattedMsg := fmt.Sprintf("data: %s\n\n", message.Message)
					_, err := c.Write(
						[]byte(formattedMsg),
					)
					if err != nil {
						return false
					}

					return true
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
		case <-c.Context().Done():
		case <-conversationContext.Done():
		}

		// Wait for all conversation listening to finish
		conversationGroup.Wait()

		if conversationContextError.err != nil {
			return c.Status(500).Send(nil)
		}

		return c.Status(200).Send(nil)
	})
}
