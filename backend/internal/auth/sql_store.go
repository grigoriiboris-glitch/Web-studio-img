package auth

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
 )

type SQLStore struct { db *sql.DB }

func NewSQLStore(db *sql.DB) (*SQLStore, error) {
    if db == nil { return nil, errors.New("auth store requires database") }
    return &SQLStore{db: db}, nil
}

func (s *SQLStore) FindUserByEmail(ctx context.Context, email string) (User, error) {
    var user User; var password sql.NullString
    err := s.db.QueryRowContext(ctx, "SELECT id, email, name, password_hash FROM users WHERE lower(email) = lower($1) LIMIT 1", email).Scan(&user.ID, &user.Email, &user.Name, &password)
    if errors.Is(err, sql.ErrNoRows) { return User{}, ErrInvalidCredentials }
    if err != nil { return User{}, fmt.Errorf("find user: %w", err) }
    user.PasswordHash = password.String
    return user, nil
}

func (s *SQLStore) CreateUser(ctx context.Context, email, name, passwordHash string) (User, error) {
    var user User
    err := s.db.QueryRowContext(ctx, "INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id, email, name, password_hash", email, name, passwordHash).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash)
    if err != nil { return User{}, fmt.Errorf("create user: %w", err) }
    return user, nil
}

func (s *SQLStore) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (uuid.UUID, error) {
    var id uuid.UUID
    err := s.db.QueryRowContext(ctx, "INSERT INTO auth_sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id", userID, tokenHash, expiresAt).Scan(&id)
    if err != nil { return uuid.Nil, fmt.Errorf("create auth session: %w", err) }
    return id, nil
}

func (s *SQLStore) IsSessionActive(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
    var active bool
    err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM auth_sessions WHERE id = $1 AND revoked_at IS NULL AND expires_at > $2)", sessionID, now).Scan(&active)
    if err != nil { return false, fmt.Errorf("check auth session: %w", err) }
    return active, nil
}

func (s *SQLStore) RevokeSession(ctx context.Context, sessionID uuid.UUID, now time.Time) error {
    _, err := s.db.ExecContext(ctx, "UPDATE auth_sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL", sessionID, now)
    if err != nil { return fmt.Errorf("revoke auth session: %w", err) }
    return nil
}
