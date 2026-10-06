package cloudagent

import (
	"strings"
	"testing"
	"time"

	agentv1 "github.com/boring-design/elastic-fruit-protocol/gen/agent/v1"
)

func TestNextReconnectDelay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		current time.Duration
		want    time.Duration
	}{
		{time.Second, 2 * time.Second},
		{8 * time.Second, 16 * time.Second},
		{16 * time.Second, 30 * time.Second},
		{30 * time.Second, 30 * time.Second},
	}
	for _, tc := range cases {
		if got := nextReconnectDelay(tc.current); got != tc.want {
			t.Errorf("nextReconnectDelay(%s) = %s, want %s", tc.current, got, tc.want)
		}
	}
}

func TestWithJitterStaysInRange(t *testing.T) {
	t.Parallel()
	delay := 10 * time.Second
	for range 100 {
		got := withJitter(delay)
		if got < delay || got >= delay+delay/2 {
			t.Fatalf("withJitter(%s) = %s, want between %s and %s", delay, got, delay, delay+delay/2)
		}
	}
}

func TestJobResultString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		result    agentv1.JobResult
		want      string
		wantKnown bool
	}{
		{agentv1.JobResult_JOB_RESULT_SUCCEEDED, "succeeded", true},
		{agentv1.JobResult_JOB_RESULT_FAILED, "failed", true},
		{agentv1.JobResult_JOB_RESULT_CANCELED, "canceled", true},
		{agentv1.JobResult_JOB_RESULT_UNSPECIFIED, "", false},
	}
	for _, tc := range cases {
		got, known := jobResultString(tc.result)
		if got != tc.want || known != tc.wantKnown {
			t.Errorf("jobResultString(%s) = (%q, %v), want (%q, %v)", tc.result, got, known, tc.want, tc.wantKnown)
		}
	}
}

func TestParseCredential(t *testing.T) {
	t.Parallel()
	credential, err := ParseCredential([]byte(`{"agent_id": "agent-1", "agent_credential": "secret"}`))
	if err != nil {
		t.Fatalf("ParseCredential() error: %v", err)
	}
	if credential.AgentID != "agent-1" || credential.AgentCredential != "secret" {
		t.Fatalf("ParseCredential() = %+v, want agent-1 and secret", credential)
	}

	cases := []struct {
		name        string
		input       string
		wantContain string
	}{
		{"not json", "agent-1", "invalid character"},
		{"missing id", `{"agent_credential": "secret"}`, "agent_id is empty"},
		{"missing credential", `{"agent_id": "agent-1"}`, "agent_credential is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseCredential([]byte(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.wantContain) {
				t.Fatalf("ParseCredential(%q) error = %v, want it to contain %q", tc.input, err, tc.wantContain)
			}
		})
	}
}

func TestSaveAndLoadCredentialRoundTrip(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/agent-credential"
	want := Credential{AgentID: "agent-1", AgentCredential: "secret"}
	if err := SaveCredential(path, want); err != nil {
		t.Fatalf("SaveCredential() error: %v", err)
	}
	got, err := LoadCredential(path)
	if err != nil {
		t.Fatalf("LoadCredential() error: %v", err)
	}
	if got != want {
		t.Fatalf("LoadCredential() = %+v, want %+v", got, want)
	}
}
