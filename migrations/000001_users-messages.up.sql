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
  recipient_id char(26) NOT NULL,
  message TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  INDEX sender_id_idx(sender_id),
  INDEX recipient_id_idx(recipient_id)
);
