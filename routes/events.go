package routes

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/azvaliev/pigeon/v2/kafka"
	"github.com/azvaliev/pigeon/v2/utils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
)

func EventsRoutes(api fiber.Router, db *sqlx.DB) {
	api.Get("/events", adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Context().Value("user-id"))

		// Determine all conversations user is part of
		conversations := &[]utils.Conversation{}
		err := db.Select(
			conversations,
			"SELECT conversation_id as id FROM ConversationMember WHERE user_id = ?",
			r.Context().Value("user-id"),
		)

		if err != nil {
			w.WriteHeader(500)
			return
		}

		// Send SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

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
					_, err := fmt.Fprintf(w, "data: %s\n\n", message.Message)
					if err != nil {
						return false
					}

					if f, ok := w.(http.Flusher); ok {
						f.Flush()
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
		case <-w.(http.CloseNotifier).CloseNotify():
		case <-conversationContext.Done():
		}

		// Wait for all conversation listening to finish
		conversationGroup.Wait()

		if conversationContextError.err != nil {
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(200)
		return
	}))
}
