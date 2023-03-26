CREATE TABLE IF NOT EXISTS Conversation (
  id char(26) PRIMARY KEY NOT NULL
);

CREATE TABLE IF NOT EXISTS ConversationMember (
  conversation_id char(26) NOT NULL,
  user_id char(26) NOT NULL,
  PRIMARY KEY (conversation_id, user_id),
  INDEX (user_id)
);

