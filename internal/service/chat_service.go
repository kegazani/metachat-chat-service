package service

import (
	"context"
	"fmt"

	"metachat/chat-service/internal/models"
	"metachat/chat-service/internal/repository"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ChatService interface {
	CreateChat(ctx context.Context, userID1, userID2 string) (*models.Chat, error)
	GetChat(ctx context.Context, chatID string) (*models.Chat, error)
	GetUserChats(ctx context.Context, userID string) ([]*models.Chat, error)
	SendMessage(ctx context.Context, chatID, senderID, content string) (*models.Message, error)
	GetChatMessages(ctx context.Context, chatID string, limit int, beforeMessageID string) ([]*models.Message, error)
	MarkMessagesAsRead(ctx context.Context, chatID, userID string) (int, error)
}

type chatService struct {
	repository repository.ChatRepository
	logger     *logrus.Logger
}

func NewChatService(repo repository.ChatRepository, logger *logrus.Logger) ChatService {
	return &chatService{
		repository: repo,
		logger:     logger,
	}
}

func (s *chatService) CreateChat(ctx context.Context, userID1, userID2 string) (*models.Chat, error) {
	if userID1 == userID2 {
		return nil, fmt.Errorf("cannot create chat with yourself")
	}

	existingChat, err := s.repository.GetChatByUsers(ctx, userID1, userID2)
	if err == nil && existingChat != nil {
		return existingChat, nil
	}

	chat := &models.Chat{
		ID:      uuid.New().String(),
		UserID1: userID1,
		UserID2: userID2,
	}

	err = s.repository.CreateChat(ctx, chat)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create chat")
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"chat_id":  chat.ID,
		"user_id1": userID1,
		"user_id2": userID2,
	}).Info("Chat created")

	return chat, nil
}

func (s *chatService) GetChat(ctx context.Context, chatID string) (*models.Chat, error) {
	chat, err := s.repository.GetChatByID(ctx, chatID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get chat")
		return nil, err
	}

	return chat, nil
}

func (s *chatService) GetUserChats(ctx context.Context, userID string) ([]*models.Chat, error) {
	chats, err := s.repository.GetUserChats(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get user chats")
		return nil, err
	}

	return chats, nil
}

func (s *chatService) SendMessage(ctx context.Context, chatID, senderID, content string) (*models.Message, error) {
	chat, err := s.repository.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("chat not found")
	}

	if chat.UserID1 != senderID && chat.UserID2 != senderID {
		return nil, fmt.Errorf("user is not a participant in this chat")
	}

	msg := &models.Message{
		ID:      uuid.New().String(),
		ChatID:  chatID,
		SenderID: senderID,
		Content: content,
	}

	err = s.repository.CreateMessage(ctx, msg)
	if err != nil {
		s.logger.WithError(err).Error("Failed to send message")
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"message_id": msg.ID,
		"chat_id":    chatID,
		"sender_id":  senderID,
	}).Info("Message sent")

	return msg, nil
}

func (s *chatService) GetChatMessages(ctx context.Context, chatID string, limit int, beforeMessageID string) ([]*models.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	messages, err := s.repository.GetChatMessages(ctx, chatID, limit, beforeMessageID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get chat messages")
		return nil, err
	}

	return messages, nil
}

func (s *chatService) MarkMessagesAsRead(ctx context.Context, chatID, userID string) (int, error) {
	chat, err := s.repository.GetChatByID(ctx, chatID)
	if err != nil {
		return 0, fmt.Errorf("chat not found")
	}

	if chat.UserID1 != userID && chat.UserID2 != userID {
		return 0, fmt.Errorf("user is not a participant in this chat")
	}

	count, err := s.repository.MarkMessagesAsRead(ctx, chatID, userID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to mark messages as read")
		return 0, err
	}

	return count, nil
}

