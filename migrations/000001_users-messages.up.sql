CREATE TABLE IF NOT EXISTS User (
  id char(26) PRIMARY KEY NOT NULL,
  email varchar(320) UNIQUE NOT NULL,
  username varchar(320) UNIQUE NOT NULL,
  display_name varchar(320) NOT NULL,
  avatar varchar(320) NOT NULL
);

CREATE TABLE IF NOT EXISTS Message (
  id char(26) PRIMARY KEY NOT NULL,
  sender_id char(26) NOT NULL,
  conversation_id char(26) NOT NULL,
  message TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  INDEX (conversation_id),
  INDEX (created_at)
);
