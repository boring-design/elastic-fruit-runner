package configstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	// Register the SQLite driver.
	_ "modernc.org/sqlite"

	"github.com/boring-design/elastic-fruit-runner/config"
)

type Revision struct {
	ID        int64
	CreatedAt time.Time
	Source    string
	Hash      string
}

func openRevisionDB(path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create config revision directory %s: %w", filepath.Dir(path), err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open config revision database %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), `
		PRAGMA busy_timeout=5000;
		CREATE TABLE IF NOT EXISTS config_revisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME NOT NULL,
			source TEXT NOT NULL,
			config_hash TEXT NOT NULL,
			config_yaml BLOB NOT NULL,
			active INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_config_revisions_created_at
		ON config_revisions (created_at DESC);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create config revision tables: %w", err)
	}
	if path != ":memory:" {
		_ = os.Chmod(path, 0o600)
	}
	return db, nil
}

func LoadLastActive(databasePath string) ([]byte, error) {
	db, err := openRevisionDB(databasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data []byte
	err = db.QueryRowContext(context.Background(), `
		SELECT config_yaml FROM config_revisions
		WHERE active = 1 ORDER BY created_at DESC LIMIT 1`).Scan(&data)
	if err != nil {
		return nil, fmt.Errorf("read last active config: %w", err)
	}
	return data, nil
}

func (s *Service) Validate(data []byte) config.ValidationResult {
	current, _ := os.ReadFile(s.path)
	merged, err := mergeMaskedSecrets(data, current)
	if err != nil {
		return config.ValidationResult{
			Errors: []config.ValidationIssue{{Path: "$", Message: err.Error()}},
		}
	}
	return safeValidation(config.ValidateYAML(merged))
}

func (s *Service) Save(data []byte, source string) (config.ValidationResult, error) {
	current, _ := os.ReadFile(s.path)
	merged, err := mergeMaskedSecrets(data, current)
	if err != nil {
		return config.ValidationResult{}, err
	}
	result := config.ValidateYAML(merged)
	if len(result.Errors) > 0 {
		return safeValidation(result), nil
	}
	if s.path == "" {
		return result, errors.New("config file path is not set")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return safeValidation(result), fmt.Errorf("create config directory %s: %w", dir, err)
	}
	file, err := os.CreateTemp(dir, ".config-save")
	if err != nil {
		return result, fmt.Errorf("create temporary config in %s: %w", dir, err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return result, fmt.Errorf("set temporary config permissions: %w", err)
	}
	if _, err := file.Write(merged); err != nil {
		file.Close()
		return result, fmt.Errorf("write temporary config: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return result, fmt.Errorf("sync temporary config: %w", err)
	}
	if err := file.Close(); err != nil {
		return result, fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return result, fmt.Errorf("replace config file %s: %w", s.path, err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return result, fmt.Errorf("set config file permissions %s: %w", s.path, err)
	}
	if err := s.saveRevision(merged, source, false); err != nil {
		return result, err
	}
	s.refresh()
	return safeValidation(result), nil
}

func safeValidation(result config.ValidationResult) config.ValidationResult {
	result.Config = nil
	if result.Normalized != "" {
		result.Normalized = redactYAML([]byte(result.Normalized))
	}
	return result
}

func (s *Service) Revisions() ([]Revision, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id, created_at, source, config_hash
		FROM config_revisions ORDER BY created_at DESC LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("list config revisions: %w", err)
	}
	defer rows.Close()
	var revisions []Revision
	for rows.Next() {
		var revision Revision
		if err := rows.Scan(&revision.ID, &revision.CreatedAt, &revision.Source, &revision.Hash); err != nil {
			return nil, fmt.Errorf("read config revision: %w", err)
		}
		revisions = append(revisions, revision)
	}
	return revisions, nil
}

func (s *Service) Restore(revisionID int64) error {
	if s.db == nil {
		return errors.New("config revision storage is unavailable")
	}
	var data []byte
	if err := s.db.QueryRowContext(context.Background(), `
		SELECT config_yaml FROM config_revisions WHERE id = ?`, revisionID).Scan(&data); err != nil {
		return fmt.Errorf("read config revision %d: %w", revisionID, err)
	}
	result, err := s.Save(data, "restore")
	if err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("revision %d is not valid: %s", revisionID, result.Errors[0].String())
	}
	return nil
}

func (s *Service) saveRevision(data []byte, source string, active bool) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("start config revision save: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	ctx := context.Background()
	if active {
		if _, err := tx.ExecContext(ctx, `UPDATE config_revisions SET active = 0`); err != nil {
			return fmt.Errorf("clear active config revision: %w", err)
		}
	}
	activeValue := 0
	if active {
		activeValue = 1
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO config_revisions (created_at, source, config_hash, config_yaml, active)
		VALUES (?, ?, ?, ?, ?)`,
		time.Now(), source, hash(data), data, activeValue,
	); err != nil {
		return fmt.Errorf("insert config revision: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM config_revisions WHERE id NOT IN (
			SELECT id FROM config_revisions ORDER BY created_at DESC LIMIT 10
		)`); err != nil {
		return fmt.Errorf("trim config revisions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit config revision: %w", err)
	}
	return nil
}

func mergeMaskedSecrets(draft, current []byte) ([]byte, error) {
	var draftNode yaml.Node
	if err := yaml.Unmarshal(draft, &draftNode); err != nil {
		return nil, fmt.Errorf("parse edited config: %w", err)
	}
	if len(current) == 0 {
		return draft, nil
	}
	var currentNode yaml.Node
	if !parseYAML(current, &currentNode) {
		return mergeMaskedLines(draft, current), nil
	}
	mergeSecretNodes(&draftNode, &currentNode)
	result, err := yaml.Marshal(&draftNode)
	if err != nil {
		return nil, fmt.Errorf("encode edited config: %w", err)
	}
	return result, nil
}

func parseYAML(data []byte, node *yaml.Node) bool {
	return yaml.Unmarshal(data, node) == nil
}

func mergeMaskedLines(draft, current []byte) []byte {
	var secrets []string
	for _, line := range strings.Split(string(current), "\n") {
		if index := strings.Index(line, "pat_token:"); index >= 0 {
			secrets = append(secrets, strings.TrimSpace(line[index+len("pat_token:"):]))
		}
	}
	lines := strings.Split(string(draft), "\n")
	secretIndex := 0
	for index, line := range lines {
		keyIndex := strings.Index(line, "pat_token:")
		if keyIndex < 0 {
			continue
		}
		value := strings.TrimSpace(line[keyIndex+len("pat_token:"):])
		if (value == "" || strings.Trim(value, "*\"'") == "") && secretIndex < len(secrets) {
			lines[index] = line[:keyIndex] + "pat_token: " + secrets[secretIndex]
		}
		secretIndex++
	}
	return []byte(strings.Join(lines, "\n"))
}

func mergeSecretNodes(draft, current *yaml.Node) {
	if draft.Kind != current.Kind {
		return
	}
	if draft.Kind == yaml.MappingNode {
		currentValues := make(map[string]*yaml.Node)
		for index := 0; index+1 < len(current.Content); index += 2 {
			currentValues[current.Content[index].Value] = current.Content[index+1]
		}
		for index := 0; index+1 < len(draft.Content); index += 2 {
			key := draft.Content[index].Value
			draftValue := draft.Content[index+1]
			currentValue := currentValues[key]
			if currentValue == nil {
				continue
			}
			if key == "pat_token" && (draftValue.Value == "" || strings.Trim(draftValue.Value, "*") == "") {
				draft.Content[index+1] = copyNode(currentValue)
				continue
			}
			mergeSecretNodes(draftValue, currentValue)
		}
		return
	}
	for index := range draft.Content {
		if index < len(current.Content) {
			mergeSecretNodes(draft.Content[index], current.Content[index])
		}
	}
}

func copyNode(node *yaml.Node) *yaml.Node {
	result := *node
	result.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		result.Content[index] = copyNode(child)
	}
	return &result
}
