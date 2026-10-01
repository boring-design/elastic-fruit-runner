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

	"github.com/boring-design/elastic-fruit-runner/config"
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/storage/sqlc"
)

type Revision struct {
	ID        int64
	CreatedAt time.Time
	Source    string
	Hash      string
}

// LoadLastActive returns the YAML of the config revision that was last applied at startup.
func LoadLastActive(ctx context.Context, db *sql.DB) ([]byte, error) {
	data, err := sqlcdb.New(db).GetLastActiveConfigYAML(ctx)
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

// ParseWithSecrets fills masked secrets from the disk file and returns the
// full config. The result holds real credentials and must never be logged.
func (s *Service) ParseWithSecrets(data []byte) (*config.Config, []config.ValidationIssue) {
	current, _ := os.ReadFile(s.path)
	merged, err := mergeMaskedSecrets(data, current)
	if err != nil {
		return nil, []config.ValidationIssue{{Path: "$", Message: err.Error()}}
	}
	result := config.ValidateYAML(merged)
	if len(result.Errors) > 0 {
		return nil, result.Errors
	}
	return result.Config, nil
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
	rows, err := s.queries.ListRecentConfigRevisions(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list config revisions: %w", err)
	}
	var revisions []Revision
	for _, row := range rows {
		revisions = append(revisions, Revision{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			Source:    row.Source,
			Hash:      row.ConfigHash,
		})
	}
	return revisions, nil
}

func (s *Service) Restore(revisionID int64) error {
	data, err := s.queries.GetConfigRevisionYAML(context.Background(), revisionID)
	if err != nil {
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
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start config revision save: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	queries := s.queries.WithTx(tx)
	if active {
		if err := queries.ClearActiveConfigRevision(ctx); err != nil {
			return fmt.Errorf("clear active config revision: %w", err)
		}
	}
	var activeValue int64
	if active {
		activeValue = 1
	}
	err = queries.InsertConfigRevision(ctx, sqlcdb.InsertConfigRevisionParams{
		CreatedAt:  time.Now(),
		Source:     source,
		ConfigHash: hash(data),
		ConfigYaml: data,
		Active:     activeValue,
	})
	if err != nil {
		return fmt.Errorf("insert config revision: %w", err)
	}
	if err := queries.TrimConfigRevisions(ctx); err != nil {
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
