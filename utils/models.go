package utils

import "time"

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

type User struct {
	Id          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name" db:"display_name"`
	Avatar      string `json:"avatar"`
}
