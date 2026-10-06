package cloudagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/boring-design/elastic-fruit-runner/config"
)

// Credential is what Enroll returned. The agent sends AgentCredential as a
// bearer token on every later call to the cloud.
type Credential struct {
	AgentID         string `json:"agent_id"`
	AgentCredential string `json:"agent_credential"`
}

const credentialFileName = "agent-credential"

// ErrCredentialMissing means the credential file has not been written yet,
// so the host still needs to run the enroll command.
var ErrCredentialMissing = errors.New("agent credential file does not exist")

// CredentialPath returns the credential file path inside the data directory.
func CredentialPath() (string, error) {
	dataDir, err := config.DefaultDataDir()
	if err != nil {
		return "", fmt.Errorf("resolve agent credential path: %w", err)
	}
	return filepath.Join(dataDir, credentialFileName), nil
}

// LoadCredential reads and checks the credential file at path.
func LoadCredential(path string) (Credential, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Credential{}, fmt.Errorf("%w: %s", ErrCredentialMissing, path)
	}
	if err != nil {
		return Credential{}, fmt.Errorf("read agent credential %s: %w", path, err)
	}
	credential, err := ParseCredential(data)
	if err != nil {
		return Credential{}, fmt.Errorf("parse agent credential %s: %w", path, err)
	}
	return credential, nil
}

// ParseCredential decodes the JSON credential file content.
func ParseCredential(data []byte) (Credential, error) {
	var credential Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		return Credential{}, err
	}
	if credential.AgentID == "" {
		return Credential{}, errors.New("agent_id is empty")
	}
	if credential.AgentCredential == "" {
		return Credential{}, errors.New("agent_credential is empty")
	}
	return credential, nil
}

// SaveCredential writes the credential file readable only by the current user.
func SaveCredential(path string, credential Credential) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create credential directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent credential: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write agent credential %s: %w", path, err)
	}
	// WriteFile keeps the old mode when the file already exists.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set agent credential permissions %s: %w", path, err)
	}
	return nil
}
