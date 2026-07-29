package configstate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/boring-design/elastic-fruit-runner/config"
	"go.yaml.in/yaml/v3"
)

// State is the relation between the running config and the disk file.
type State string

const (
	StateInSync          State = "in_sync"
	StateRestartRequired State = "restart_required"
	StateDiskInvalid     State = "disk_invalid"
)

// Snapshot is a safe view of config state for the console.
type Snapshot struct {
	Path             string
	ActiveHash       string
	DiskHash         string
	State            State
	DiskModifiedAt   *time.Time
	ActiveLoadedAt   time.Time
	ValidationErrors []string
	ActiveYAML       string
	DiskYAML         string
}

// Service watches the config file without applying changes.
type Service struct {
	path           string
	activeHash     string
	activeYAML     string
	activeLoadedAt time.Time

	mu       sync.RWMutex
	snapshot Snapshot
}

// New creates a config state service from the config loaded at startup.
func New(cfg *config.Config, loadedAt time.Time) *Service {
	activeData := cfg.LoadedYAML
	activeHash := cfg.LoadedHash
	if len(activeData) == 0 {
		activeData, _ = yaml.Marshal(cfg)
		activeHash = hash(activeData)
	}
	service := &Service{
		path:           cfg.FilePath,
		activeHash:     activeHash,
		activeYAML:     redactYAML(activeData),
		activeLoadedAt: loadedAt,
	}
	service.refresh()
	return service
}

// Start refreshes disk state until the context ends.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh()
		}
	}
}

// Get returns the latest config state snapshot.
func (s *Service) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := s.snapshot
	result.ValidationErrors = append([]string(nil), s.snapshot.ValidationErrors...)
	return result
}

func (s *Service) refresh() {
	next := Snapshot{
		Path:           s.path,
		ActiveHash:     s.activeHash,
		State:          StateInSync,
		ActiveLoadedAt: s.activeLoadedAt,
		ActiveYAML:     s.activeYAML,
	}

	if s.path == "" {
		next.DiskHash = s.activeHash
		next.DiskYAML = s.activeYAML
		s.set(next)
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		next.State = StateDiskInvalid
		next.ValidationErrors = []string{fmt.Sprintf("read config file %s: %v", s.path, err)}
		s.set(next)
		return
	}

	next.DiskHash = hash(data)
	next.DiskYAML = redactYAML(data)
	if info, statErr := os.Stat(s.path); statErr == nil {
		modifiedAt := info.ModTime()
		next.DiskModifiedAt = &modifiedAt
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		next.State = StateDiskInvalid
		next.ValidationErrors = []string{fmt.Sprintf("parse config file %s: %v", s.path, err)}
		s.set(next)
		return
	}

	if next.DiskHash != s.activeHash {
		next.State = StateRestartRequired
	}
	s.set(next)
}

func (s *Service) set(snapshot Snapshot) {
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func redactYAML(data []byte) string {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return ""
	}
	redactNode(&document)
	redacted, err := yaml.Marshal(&document)
	if err != nil {
		return ""
	}
	return string(redacted)
}

func redactNode(node *yaml.Node) {
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			key := node.Content[index]
			value := node.Content[index+1]
			if key.Value == "pat_token" {
				value.Kind = yaml.ScalarNode
				value.Tag = "!!str"
				value.Value = "********"
				value.Content = nil
				continue
			}
			redactNode(value)
		}
		return
	}
	for _, child := range node.Content {
		redactNode(child)
	}
}
