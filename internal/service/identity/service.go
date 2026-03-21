package identity

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quantum-cluster/go-proj1/internal/config"
	"github.com/quantum-cluster/go-proj1/internal/repository"
	pb "github.com/quantum-cluster/go-proj1/protos/identity/v1"
)

var dummyHash []byte

func init() {
	var err error
	dummyHash, err = bcrypt.GenerateFromPassword([]byte(uuid.NewString()), bcrypt.DefaultCost)
	if err != nil {
		panic("failed to generate dummy hash")
	}
}

// Service implements the IdentityService gRPC server
type Service struct {
	pb.UnimplementedIdentityServiceServer
	repo   repository.UserRepository
	config *config.Config
	logger *slog.Logger
}

// NewService creates a new Identity Service
func NewService(repo repository.UserRepository, cfg *config.Config, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		config: cfg,
		logger: logger,
	}
}

func (s *Service) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	email := req.Email
	password := req.Password
	fullName := req.FullName

	if email == "" || len(password) < 6 || len(password) > 72 || fullName == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid arguments passed")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "Couldn't hash password successfully")
	}

	uniqueId := uuid.NewString()

	err = s.repo.CreateUser(ctx, uniqueId, email, hashedPassword, fullName)
	if err != nil {
		s.logger.Error("Failed to create user", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "Couldn't insert user into DB")
	}

	return &pb.RegisterResponse{
		Uuid: uniqueId,
	}, nil
}

func (s *Service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	email := req.Email
	password := req.Password

	if email == "" || len(password) < 6 || len(password) > 72 {
		return nil, status.Error(codes.InvalidArgument, "Invalid arguments passed")
	}

	var user *repository.User
	user, err := s.repo.GetUserByEmail(ctx, email)

	if errors.Is(err, repository.ErrUserNotFound) {
		user = &repository.User{
			ID:             uuid.NewString(),
			HashedPassword: dummyHash,
		}
		err = nil
	} else if err != nil {
		s.logger.Error("Database error during login", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "Database error during login")
	}

	if err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "incorrect username or password")
	}

	// Access Token
	now := time.Now()
	accessClaims := jwt.RegisteredClaims{
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	signedToken, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		s.logger.Error("Failed to sign access token", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "Something went wrong signing token")
	}

	// Refresh Token
	refreshClaims := jwt.RegisteredClaims{
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		s.logger.Error("Failed to sign refresh token", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "Something went wrong signing token")
	}

	return &pb.LoginResponse{
		AccessToken:  signedToken,
		RefreshToken: refreshToken,
	}, nil
}
