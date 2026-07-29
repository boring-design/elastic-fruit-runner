package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	// Register the SQLite driver.
	_ "modernc.org/sqlite"
)

const (
	SessionCookieName = "elastic_fruit_runner_session"
	sessionLifetime   = 24 * time.Hour
)

var (
	ErrAlreadySetup       = errors.New("admin password is already set")
	ErrInvalidSetupCode   = errors.New("setup code is not valid")
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
	db *sql.DB

	mu               sync.Mutex
	setupCodeHash    []byte
	pendingSetupCode string
	failedAttempts   []time.Time
	blockedUntil     time.Time
}

// Open opens auth storage in the main SQLite database.
func Open(dbPath string) (*Service, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("open auth database: database path is empty")
	}
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create auth database directory %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open auth database %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), "PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set auth database busy timeout for %s: %w", dbPath, err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate auth database %s: %w", dbPath, err)
	}
	if dbPath != ":memory:" {
		if err := os.Chmod(dbPath, 0o600); err != nil {
			db.Close()
			return nil, fmt.Errorf("set auth database permissions %s: %w", dbPath, err)
		}
	}

	service := &Service{db: db}
	setupRequired, err := service.SetupRequired(context.Background())
	if err != nil {
		db.Close()
		return nil, err
	}
	if setupRequired {
		code, err := randomToken(12)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("create admin setup code: %w", err)
		}
		sum := sha256.Sum256([]byte(code))
		service.setupCodeHash = sum[:]
		service.pendingSetupCode = code
	}
	return service, nil
}

// Close closes the auth database.
func (s *Service) Close() error {
	return s.db.Close()
}

// SetupCode returns the setup code created for this process.
func (s *Service) SetupCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingSetupCode
}

// SetupRequired reports whether an admin password exists.
func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM console_admin WHERE id = 1").Scan(&count); err != nil {
		return false, fmt.Errorf("check admin setup state: %w", err)
	}
	return count == 0, nil
}

// Setup creates the admin password after setup code verification.
func (s *Service) Setup(ctx context.Context, code, password string) (Session, error) {
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

	sum := sha256.Sum256([]byte(code))
	s.mu.Lock()
	codeHash := append([]byte(nil), s.setupCodeHash...)
	s.mu.Unlock()
	if len(codeHash) == 0 || subtle.ConstantTimeCompare(sum[:], codeHash) != 1 {
		return Session{}, ErrInvalidSetupCode
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, fmt.Errorf("hash admin password: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO console_admin (id, password_hash, created_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET password_hash = excluded.password_hash, created_at = excluded.created_at
	`, passwordHash, time.Now().Unix()); err != nil {
		return Session{}, fmt.Errorf("save admin password: %w", err)
	}

	s.mu.Lock()
	s.setupCodeHash = nil
	s.pendingSetupCode = ""
	s.mu.Unlock()
	return s.createSession(ctx)
}

// Login verifies the admin password and creates a session.
func (s *Service) Login(ctx context.Context, password string) (Session, error) {
	if err := s.checkLoginLimit(); err != nil {
		return Session{}, err
	}

	var passwordHash []byte
	if err := s.db.QueryRowContext(ctx, "SELECT password_hash FROM console_admin WHERE id = 1").Scan(&passwordHash); err != nil {
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
	var csrfToken string
	var expiresAtUnix int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT csrf_token, expires_at
		FROM console_sessions
		WHERE token_hash = ?
	`, tokenHash).Scan(&csrfToken, &expiresAtUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("read console session: %w", err)
	}
	expiresAt := time.Unix(expiresAtUnix, 0)
	if !expiresAt.After(time.Now()) {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM console_sessions WHERE token_hash = ?", tokenHash)
		return Session{}, ErrSessionNotFound
	}
	return Session{Token: token, CSRFToken: csrfToken, ExpiresAt: expiresAt}, nil
}

// Logout deletes one session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM console_sessions WHERE token_hash = ?", hashToken(token)); err != nil {
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
	if _, err := tx.ExecContext(ctx, "DELETE FROM console_sessions"); err != nil {
		return fmt.Errorf("delete console sessions during admin reset: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM console_admin"); err != nil {
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
	if _, err := s.db.ExecContext(ctx, "DELETE FROM console_sessions WHERE expires_at <= ?", time.Now().Unix()); err != nil {
		return Session{}, fmt.Errorf("delete expired console sessions: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO console_sessions (token_hash, csrf_token, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, hashToken(token), csrfToken, expiresAt.Unix(), time.Now().Unix()); err != nil {
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

func validatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("%w: password must have at least 12 characters", ErrInvalidPassword)
	}
	if len(password) > 256 {
		return fmt.Errorf("%w: password must have at most 256 characters", ErrInvalidPassword)
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

func migrate(db *sql.DB) error {
	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS console_admin (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			password_hash BLOB NOT NULL,
			created_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS console_sessions (
			token_hash TEXT PRIMARY KEY,
			csrf_token TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_console_sessions_expires_at
		ON console_sessions (expires_at);
	`)
	if err != nil {
		return fmt.Errorf("create console auth tables: %w", err)
	}
	return nil
}
