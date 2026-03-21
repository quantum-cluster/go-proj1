package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/crypto/bcrypt"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quantum-cluster/go-proj1/internal/interceptor"

	pb "github.com/quantum-cluster/go-proj1/protos/identity/v1"
)

type identityServer struct {
	pb.UnimplementedIdentityServiceServer
	db *pgxpool.Pool
}

type User struct {
	id             string
	email          string
	hashedPassword []byte
	fullName       string
}

var (
	// Our server secret that will be moved to a different file which will not be commited to Git.
	serverSecret = "not a real secret"

	// Dummy hash
	dummyHash []byte
)

func InitPool(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return pgxpool.New(ctx, databaseURL)
}

func init() {
	var err error
	dummyHash, err = bcrypt.GenerateFromPassword([]byte(uuid.NewString()), bcrypt.DefaultCost)

	if err != nil {
		panic("failed to generate dummy hash")
	}
}

func main() {
	interceptors := []grpc.UnaryServerInterceptor{
		interceptor.LoggingInterceptor,
		interceptor.AuthInterceptor,
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	pool, err := InitPool("postgres://postgres:postgres@localhost:5432/postgres")
	if err != nil {
		log.Default().Fatalf("Can't connect to DB: %v", err)
	}
	defer pool.Close()

	pb.RegisterIdentityServiceServer(grpcServer, &identityServer{db: pool})

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Default().Fatalf("Can't listen: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	wg.Go(func() {
		log.Default().Println("Starting server on port :50051")

		if err := grpcServer.Serve(listener); err != nil {
			log.Default().Printf("Terminating server: %v", err)
		}
	})

	<-ctx.Done()
	log.Default().Println("Context expired. Exiting gracefully...")
	grpcServer.GracefulStop()
	wg.Wait()
	log.Default().Println("Process complete")
}

func (is *identityServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	email := req.Email
	password := req.Password
	fullName := req.FullName

	if len(email) == 0 || len(password) < 6 || len(fullName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Invalid arguments passed")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "Couldn't hash password successfully")
	}

	uniqueId := uuid.New().String()

	_, execErr := is.db.Exec(ctx, "INSERT INTO users (id, email, hashed_password, full_name) VALUES ($1, $2, $3, $4)", uniqueId, email, hashedPassword, fullName)
	if execErr != nil {
		return nil, status.Error(codes.Internal, "Couldn't insert user into DB")
	}

	return &pb.RegisterResponse{
		Uuid: uniqueId,
	}, nil
}

func (is *identityServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	email := req.Email
	password := req.Password

	if len(email) == 0 || len(password) < 6 {
		return nil, status.Error(codes.InvalidArgument, "Invalid arguments passed")
	}

	user := User{}
	userRow := is.db.QueryRow(ctx, "SELECT id, email, hashed_password, full_name FROM users WHERE email = $1", email)
	userRowErr := userRow.Scan(&user.id, &user.email, &user.hashedPassword, &user.fullName)
	if errors.Is(userRowErr, pgx.ErrNoRows) {
		user = User{
			id:             uuid.NewString(),
			hashedPassword: dummyHash,
		}

		userRowErr = nil
	} else if userRowErr != nil {
		return nil, status.Error(codes.Internal, "Database error during login")
	}

	if err := bcrypt.CompareHashAndPassword(user.hashedPassword, []byte(password)); err != nil || userRowErr != nil {
		return nil, status.Error(codes.Unauthenticated, "incorrect username or password")
	}

	// Access Token
	claims := jwt.MapClaims{
		"sub": user.id,
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(serverSecret))
	if err != nil {
		return nil, status.Error(codes.Internal, "Something went wrong")
	}

	// Refresh Token
	claims = jwt.MapClaims{
		"sub": user.id,
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refreshToken, err := token.SignedString([]byte(serverSecret))
	if err != nil {
		return nil, status.Error(codes.Internal, "Something went wrong")
	}

	return &pb.LoginResponse{
		AccessToken:  signedToken,
		RefreshToken: refreshToken,
	}, nil
}
