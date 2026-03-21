package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents a user model in the database
type User struct {
	ID             string
	Email          string
	HashedPassword []byte
	FullName       string
}

// UserRepository defines the interface for data access logic regarding users
type UserRepository interface {
	CreateUser(ctx context.Context, id, email string, hashedPassword []byte, fullName string) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
}

type pgUserRepository struct {
	db *pgxpool.Pool
}

// NewPgUserRepository creates a new PostgreSQL implementation of UserRepository
func NewPgUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgUserRepository{db: pool}
}

func (r *pgUserRepository) CreateUser(ctx context.Context, id, email string, hashedPassword []byte, fullName string) error {
	_, err := r.db.Exec(ctx, "INSERT INTO users (id, email, hashed_password, full_name) VALUES ($1, $2, $3, $4)", id, email, hashedPassword, fullName)
	return err
}

func (r *pgUserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.QueryRow(ctx, "SELECT id, email, hashed_password, full_name FROM users WHERE email = $1", email).
		Scan(&user.ID, &user.Email, &user.HashedPassword, &user.FullName)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
