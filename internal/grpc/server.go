package grpc

import (
	"context"

	"metachat/chat-service/internal/models"
	"metachat/chat-service/internal/service"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/kegazani/metachat-proto/chat"
)

type ChatServer struct {
	pb.UnimplementedChatServiceServer
	service service.ChatService
	logger  *logrus.Logger
}

func NewChatServer(svc service.ChatService, logger *logrus.Logger) *ChatServer {
	return &ChatServer{
		service: svc,
		logger:  logger,
	}
}

func (s *ChatServer) CreateChat(ctx context.Context, req *pb.CreateChatRequest) (*pb.CreateChatResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"user_id1": req.UserId1,
		"user_id2": req.UserId2,
	}).Info("Creating chat via gRPC")

	chat, err := s.service.CreateChat(ctx, req.UserId1, req.UserId2)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create chat")
		return nil, status.Errorf(codes.Internal, "failed to create chat: %v", err)
	}

	return &pb.CreateChatResponse{
		Chat: s.chatToProto(chat),
	}, nil
}

func (s *ChatServer) GetChat(ctx context.Context, req *pb.GetChatRequest) (*pb.GetChatResponse, error) {
	s.logger.WithField("chat_id", req.ChatId).Info("Getting chat via gRPC")

	chat, err := s.service.GetChat(ctx, req.ChatId)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get chat")
		if err.Error() == "chat not found" {
			return nil, status.Errorf(codes.NotFound, "chat not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get chat: %v", err)
	}

	return &pb.GetChatResponse{
		Chat: s.chatToProto(chat),
	}, nil
}

func (s *ChatServer) GetUserChats(ctx context.Context, req *pb.GetUserChatsRequest) (*pb.GetUserChatsResponse, error) {
	s.logger.WithField("user_id", req.UserId).Info("Getting user chats via gRPC")

	chats, err := s.service.GetUserChats(ctx, req.UserId)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get user chats")
		return nil, status.Errorf(codes.Internal, "failed to get user chats: %v", err)
	}

	protoChats := make([]*pb.Chat, len(chats))
	for i, c := range chats {
		protoChats[i] = s.chatToProto(c)
	}

	return &pb.GetUserChatsResponse{
		Chats: protoChats,
	}, nil
}

func (s *ChatServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"chat_id":   req.ChatId,
		"sender_id": req.SenderId,
	}).Info("Sending message via gRPC")

	msg, err := s.service.SendMessage(ctx, req.ChatId, req.SenderId, req.Content)
	if err != nil {
		s.logger.WithError(err).Error("Failed to send message")
		if err.Error() == "chat not found" {
			return nil, status.Errorf(codes.NotFound, "chat not found")
		}
		if err.Error() == "user is not a participant in this chat" {
			return nil, status.Errorf(codes.PermissionDenied, "user is not a participant in this chat")
		}
		return nil, status.Errorf(codes.Internal, "failed to send message: %v", err)
	}

	return &pb.SendMessageResponse{
		Message: s.messageToProto(msg),
	}, nil
}

func (s *ChatServer) GetChatMessages(ctx context.Context, req *pb.GetChatMessagesRequest) (*pb.GetChatMessagesResponse, error) {
	s.logger.WithField("chat_id", req.ChatId).Info("Getting chat messages via gRPC")

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	messages, err := s.service.GetChatMessages(ctx, req.ChatId, limit, req.BeforeMessageId)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get chat messages")
		return nil, status.Errorf(codes.Internal, "failed to get chat messages: %v", err)
	}

	protoMessages := make([]*pb.Message, len(messages))
	for i, m := range messages {
		protoMessages[i] = s.messageToProto(m)
	}

	return &pb.GetChatMessagesResponse{
		Messages: protoMessages,
	}, nil
}

func (s *ChatServer) MarkMessagesAsRead(ctx context.Context, req *pb.MarkMessagesAsReadRequest) (*pb.MarkMessagesAsReadResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"chat_id": req.ChatId,
		"user_id": req.UserId,
	}).Info("Marking messages as read via gRPC")

	count, err := s.service.MarkMessagesAsRead(ctx, req.ChatId, req.UserId)
	if err != nil {
		s.logger.WithError(err).Error("Failed to mark messages as read")
		if err.Error() == "chat not found" {
			return nil, status.Errorf(codes.NotFound, "chat not found")
		}
		if err.Error() == "user is not a participant in this chat" {
			return nil, status.Errorf(codes.PermissionDenied, "user is not a participant in this chat")
		}
		return nil, status.Errorf(codes.Internal, "failed to mark messages as read: %v", err)
	}

	return &pb.MarkMessagesAsReadResponse{
		MarkedCount: int32(count),
	}, nil
}

func (s *ChatServer) chatToProto(chat *models.Chat) *pb.Chat {
	return &pb.Chat{
		Id:        chat.ID,
		UserId1:   chat.UserID1,
		UserId2:   chat.UserID2,
		CreatedAt: timestamppb.New(chat.CreatedAt),
		UpdatedAt: timestamppb.New(chat.UpdatedAt),
	}
}

func (s *ChatServer) messageToProto(msg *models.Message) *pb.Message {
	protoMsg := &pb.Message{
		Id:        msg.ID,
		ChatId:    msg.ChatID,
		SenderId:  msg.SenderID,
		Content:   msg.Content,
		CreatedAt: timestamppb.New(msg.CreatedAt),
	}

	if msg.ReadAt != nil {
		protoMsg.ReadAt = timestamppb.New(*msg.ReadAt)
	}

	return protoMsg
}
