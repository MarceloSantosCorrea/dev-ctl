package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Register creates a new user. If it's the first user, adopts orphan projects.
func (s *Service) Register(ctx context.Context, name, email, password string) (*User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))

	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	userID := uuid.New().String()

	// Check if this is the first user
	var userCount int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO users (id, name, email, password_hash) VALUES (?, ?, ?, ?)",
		userID, name, email, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("email already registered")
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}

	// First user adopts orphan projects
	if userCount == 0 {
		s.db.ExecContext(ctx, "UPDATE projects SET user_id = ? WHERE user_id = ''", userID)
	}

	return &User{
		ID:        userID,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}, nil
}

// Login validates credentials and creates a session.
func (s *Service) Login(ctx context.Context, email, password string) (string, *User, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	var user User
	var passwordHash string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, email, password_hash, created_at FROM users WHERE email = ?", email,
	).Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &user.CreatedAt)
	if err != nil {
		return "", nil, fmt.Errorf("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", nil, fmt.Errorf("invalid email or password")
	}

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", nil, fmt.Errorf("generating token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, user.ID, expiresAt,
	)
	if err != nil {
		return "", nil, fmt.Errorf("creating session: %w", err)
	}

	// Clean up expired sessions
	s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now())

	return token, &user, nil
}

// Logout removes a session.
func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}

// ValidateSession checks if a token is valid and returns the associated user.
func (s *Service) ValidateSession(ctx context.Context, token string) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.name, u.email, u.created_at
		 FROM sessions s JOIN users u ON s.user_id = u.id
		 WHERE s.token = ? AND s.expires_at > ?`,
		token, time.Now(),
	).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}
	return &user, nil
}

// Context helpers

type contextKey string

const userContextKey contextKey = "auth_user"

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}

// Cookie helpers

const sessionCookieName = "devctl_session"

func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60, // 7 days
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
