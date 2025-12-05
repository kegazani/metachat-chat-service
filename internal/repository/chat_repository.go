package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metachat/chat-service/internal/models"
)

type ChatRepository interface {
	CreateChat(ctx context.Context, chat *models.Chat) error
	GetChatByID(ctx context.Context, id string) (*models.Chat, error)
	GetChatByUsers(ctx context.Context, userID1, userID2 string) (*models.Chat, error)
	GetUserChats(ctx context.Context, userID string) ([]*models.Chat, error)
	UpdateChat(ctx context.Context, chat *models.Chat) error
	CreateMessage(ctx context.Context, msg *models.Message) error
	GetChatMessages(ctx context.Context, chatID string, limit int, beforeMessageID string) ([]*models.Message, error)
	MarkMessagesAsRead(ctx context.Context, chatID, userID string) (int, error)
	InitializeTables() error
}

type chatRepository struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) ChatRepository {
	return &chatRepository{
		db: db,
	}
}

func (r *chatRepository) InitializeTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS chats (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id1 UUID NOT NULL,
		user_id2 UUID NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(user_id1, user_id2)
	);

	CREATE TABLE IF NOT EXISTS messages (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		sender_id UUID NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		read_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
	CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	CREATE INDEX IF NOT EXISTS idx_chats_user1 ON chats(user_id1);
	CREATE INDEX IF NOT EXISTS idx_chats_user2 ON chats(user_id2);
	`

	_, err := r.db.Exec(query)
	return err
}

func (r *chatRepository) CreateChat(ctx context.Context, chat *models.Chat) error {
	query := `
	INSERT INTO chats (id, user_id1, user_id2, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (user_id1, user_id2) DO UPDATE SET updated_at = NOW()
	RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, query,
		chat.ID, chat.UserID1, chat.UserID2, chat.CreatedAt, chat.UpdatedAt,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("chat already exists")
		}
		return err
	}

	chat.ID = id
	chat.CreatedAt = createdAt
	chat.UpdatedAt = updatedAt
	return nil
}

func (r *chatRepository) GetChatByID(ctx context.Context, id string) (*models.Chat, error) {
	query := `
	SELECT id, user_id1, user_id2, created_at, updated_at
	FROM chats
	WHERE id = $1
	`

	var chat models.Chat
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&chat.ID, &chat.UserID1, &chat.UserID2, &chat.CreatedAt, &chat.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chat not found")
		}
		return nil, err
	}

	return &chat, nil
}

func (r *chatRepository) GetChatByUsers(ctx context.Context, userID1, userID2 string) (*models.Chat, error) {
	query := `
	SELECT id, user_id1, user_id2, created_at, updated_at
	FROM chats
	WHERE (user_id1 = $1 AND user_id2 = $2) OR (user_id1 = $2 AND user_id2 = $1)
	LIMIT 1
	`

	var chat models.Chat
	err := r.db.QueryRowContext(ctx, query, userID1, userID2).Scan(
		&chat.ID, &chat.UserID1, &chat.UserID2, &chat.CreatedAt, &chat.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chat not found")
		}
		return nil, err
	}

	return &chat, nil
}

func (r *chatRepository) GetUserChats(ctx context.Context, userID string) ([]*models.Chat, error) {
	query := `
	SELECT id, user_id1, user_id2, created_at, updated_at
	FROM chats
	WHERE user_id1 = $1 OR user_id2 = $1
	ORDER BY updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*models.Chat
	for rows.Next() {
		var chat models.Chat
		err := rows.Scan(
			&chat.ID, &chat.UserID1, &chat.UserID2, &chat.CreatedAt, &chat.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		chats = append(chats, &chat)
	}

	return chats, rows.Err()
}

func (r *chatRepository) UpdateChat(ctx context.Context, chat *models.Chat) error {
	query := `
	UPDATE chats
	SET updated_at = NOW()
	WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, chat.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("chat not found")
	}

	return nil
}

func (r *chatRepository) CreateMessage(ctx context.Context, msg *models.Message) error {
	query := `
	INSERT INTO messages (id, chat_id, sender_id, content, created_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id, created_at
	`

	var id string
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx, query,
		msg.ID, msg.ChatID, msg.SenderID, msg.Content, msg.CreatedAt,
	).Scan(&id, &createdAt)

	if err != nil {
		return err
	}

	msg.ID = id
	msg.CreatedAt = createdAt

	updateChatQuery := `UPDATE chats SET updated_at = NOW() WHERE id = $1`
	r.db.ExecContext(ctx, updateChatQuery, msg.ChatID)

	return nil
}

func (r *chatRepository) GetChatMessages(ctx context.Context, chatID string, limit int, beforeMessageID string) ([]*models.Message, error) {
	var query string
	var args []interface{}

	if beforeMessageID != "" {
		query = `
		SELECT id, chat_id, sender_id, content, created_at, read_at
		FROM messages
		WHERE chat_id = $1 AND id < $2
		ORDER BY created_at DESC
		LIMIT $3
		`
		args = []interface{}{chatID, beforeMessageID, limit}
	} else {
		query = `
		SELECT id, chat_id, sender_id, content, created_at, read_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2
		`
		args = []interface{}{chatID, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var msg models.Message
		var readAt sql.NullTime
		err := rows.Scan(
			&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Content, &msg.CreatedAt, &readAt,
		)
		if err != nil {
			return nil, err
		}
		if readAt.Valid {
			msg.ReadAt = &readAt.Time
		}
		messages = append(messages, &msg)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

func (r *chatRepository) MarkMessagesAsRead(ctx context.Context, chatID, userID string) (int, error) {
	query := `
	UPDATE messages
	SET read_at = NOW()
	WHERE chat_id = $1 AND sender_id != $2 AND read_at IS NULL
	RETURNING id
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	return count, rows.Err()
}

