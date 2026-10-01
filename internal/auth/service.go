package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/storage/sqlc"
)

const (
	SessionCookieName = "elastic_fruit_runner_session"
	sessionLifetime   = 24 * time.Hour
)

var (
	ErrAlreadySetup       = errors.New("admin password is already set")
	ErrInvalidCredentials = errors.New("password is not valid")
	ErrInvalidPassword    = errors.New("password does not meet requirements")
	ErrLoginBlocked       = errors.New("too many failed login attempts")
	ErrSessionNotFound    = errors.New("session is not valid")
)

// Session is an authenticated console session.
type Session struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

// Service stores one admin password and short lived sessions.
type Service struct {
	db      *sql.DB
	queries *sqlcdb.Queries

	mu             sync.Mutex
	failedAttempts []time.Time
	blockedUntil   time.Time
}

// New creates the auth service on top of an already opened and migrated database.
func New(db *sql.DB) *Service {
	return &Service{db: db, queries: sqlcdb.New(db)}
}

// SetupRequired reports whether an admin password exists.
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	count, err := s.queries.CountAdmin(ctx)
	if err != nil {
		return false, fmt.Errorf("check admin setup state: %w", err)
	}
	return count == 0, nil
}

// Setup creates the admin password when none exists.
func (s *Service) Setup(ctx context.Context, password string) (Session, error) {
	required, err := s.SetupRequired(ctx)
	if err != nil {
		return Session{}, err
	}
	if !required {
		return Session{}, ErrAlreadySetup
	}
	if err := validatePassword(password); err != nil {
		return Session{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, fmt.Errorf("hash admin password: %w", err)
	}
	err = s.queries.UpsertAdminPassword(ctx, sqlcdb.UpsertAdminPasswordParams{
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return Session{}, fmt.Errorf("save admin password: %w", err)
	}
	return s.createSession(ctx)
}

// Login verifies the admin password and creates a session.
func (s *Service) Login(ctx context.Context, password string) (Session, error) {
	if err := s.checkLoginLimit(); err != nil {
		return Session{}, err
	}

	passwordHash, err := s.queries.GetAdminPasswordHash(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, fmt.Errorf("read admin password: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword(passwordHash, []byte(password)); err != nil {
		s.recordLoginFailure()
		return Session{}, ErrInvalidCredentials
	}
	s.clearLoginFailures()
	return s.createSession(ctx)
}

// FindSession returns an active session for a raw cookie token.
func (s *Service) FindSession(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrSessionNotFound
	}
	tokenHash := hashToken(token)
	stored, err := s.queries.GetSession(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("read console session: %w", err)
	}
	expiresAt := time.Unix(stored.ExpiresAt, 0)
	if !expiresAt.After(time.Now()) {
		_ = s.queries.DeleteSession(ctx, tokenHash)
		return Session{}, ErrSessionNotFound
	}
	return Session{Token: token, CSRFToken: stored.CsrfToken, ExpiresAt: expiresAt}, nil
}

// Logout deletes one session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.queries.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete console session: %w", err)
	}
	return nil
}

// Reset removes the admin password and every session.
func (s *Service) Reset(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start admin reset: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	queries := s.queries.WithTx(tx)
	if err := queries.DeleteAllSessions(ctx); err != nil {
		return fmt.Errorf("delete console sessions during admin reset: %w", err)
	}
	if err := queries.DeleteAdmin(ctx); err != nil {
		return fmt.Errorf("delete admin password during reset: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit admin reset: %w", err)
	}
	return nil
}

func (s *Service) createSession(ctx context.Context) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, fmt.Errorf("create session token: %w", err)
	}
	csrfToken, err := randomToken(24)
	if err != nil {
		return Session{}, fmt.Errorf("create CSRF token: %w", err)
	}
	expiresAt := time.Now().Add(sessionLifetime)
	if err := s.queries.DeleteExpiredSessions(ctx, time.Now().Unix()); err != nil {
		return Session{}, fmt.Errorf("delete expired console sessions: %w", err)
	}
	err = s.queries.InsertSession(ctx, sqlcdb.InsertSessionParams{
		TokenHash: hashToken(token),
		CsrfToken: csrfToken,
		ExpiresAt: expiresAt.Unix(),
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return Session{}, fmt.Errorf("save console session: %w", err)
	}
	return Session{Token: token, CSRFToken: csrfToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) checkLoginLimit() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.blockedUntil) {
		return ErrLoginBlocked
	}
	cutoff := time.Now().Add(-5 * time.Minute)
	kept := s.failedAttempts[:0]
	for _, attempt := range s.failedAttempts {
		if attempt.After(cutoff) {
			kept = append(kept, attempt)
		}
	}
	s.failedAttempts = kept
	return nil
}

func (s *Service) recordLoginFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedAttempts = append(s.failedAttempts, time.Now())
	if len(s.failedAttempts) >= 5 {
		s.blockedUntil = time.Now().Add(time.Minute)
		s.failedAttempts = nil
	}
}

func (s *Service) clearLoginFailures() {
	s.mu.Lock()
	s.failedAttempts = nil
	s.blockedUntil = time.Time{}
	s.mu.Unlock()
}

// bcrypt cannot hash more than 72 bytes, this is the only password rule.
func validatePassword(password string) error {
	if len(password) > 72 {
		return fmt.Errorf("%w: password must have at most 72 bytes, got %d", ErrInvalidPassword, len(password))
	}
	return nil
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
