package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcServer "metachat/chat-service/internal/grpc"
	"metachat/chat-service/internal/repository"
	"metachat/chat-service/internal/service"

	pb "github.com/kegazani/metachat-proto/chat"
	"github.com/sirupsen/logrus"
)

func main() {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/app/config")

	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatalf("Failed to read config file: %v", err)
	}

	logger := logrus.New()
	logLevel := viper.GetString("logging.level")
	logFormat := viper.GetString("logging.format")

	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	if logFormat == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	dbHost := viper.GetString("database.host")
	dbPort := viper.GetInt("database.port")
	dbUser := viper.GetString("database.user")
	dbPassword := viper.GetString("database.password")
	dbName := viper.GetString("database.dbname")
	sslmode := viper.GetString("database.sslmode")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == 0 {
		dbPort = 5432
	}
	if dbUser == "" {
		dbUser = "postgres"
	}
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	if dbName == "" {
		dbName = "metachat"
	}
	if sslmode == "" {
		sslmode = "disable"
	}

	dsn := "postgres://" + dbUser + ":" + dbPassword + "@" + dbHost + ":" +
		strings.TrimSpace(strings.Replace(fmt.Sprintf("%d", dbPort), " ", "", -1)) + "/" + dbName + "?sslmode=" + sslmode

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}

	logger.Info("Connected to PostgreSQL database")

	chatRepo := repository.NewChatRepository(db)
	if err := chatRepo.InitializeTables(); err != nil {
		logger.Fatalf("Failed to initialize database tables: %v", err)
	}

	chatService := service.NewChatService(chatRepo, logger)
	grpcSrv := grpcServer.NewChatServer(chatService, logger)

	port := viper.GetString("server.port")
	if port == "" {
		port = "50055"
	}

	host := viper.GetString("server.host")
	if host == "" {
		host = "0.0.0.0"
	}

	address := net.JoinHostPort(host, port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logger.Fatalf("Failed to listen on %s: %v", address, err)
	}

	s := grpc.NewServer()
	pb.RegisterChatServiceServer(s, grpcSrv)

	if viper.GetBool("grpc.reflection_enabled") {
		reflection.Register(s)
		logger.Info("gRPC reflection enabled")
	}

	go func() {
		logger.Infof("Starting gRPC server on %s", address)
		if err := s.Serve(lis); err != nil {
			logger.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gRPC server...")

	shutdownTimeout := viper.GetDuration("grpc.shutdown_timeout")
	if shutdownTimeout == 0 {
		shutdownTimeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("gRPC server exited gracefully")
	case <-ctx.Done():
		logger.Info("gRPC server shutdown timeout")
	}

	logger.Info("Server exited")
}

