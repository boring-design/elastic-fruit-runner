package management

import (
	"testing"

	"github.com/actions/scaleset"
)

func TestMergeJobMetadata(t *testing.T) {
	t.Parallel()
	existing := JobRecord{
		RunnerName:    "runner-1",
		RunnerSetName: "set-1",
		Backend:       "docker",
		DisplayName:   ProvisionalDisplayName,
	}

	t.Run("control plane values win", func(t *testing.T) {
		t.Parallel()
		incoming := &scaleset.JobStarted{
			RunnerName:     "runner-1",
			JobMessageBase: scaleset.JobMessageBase{JobID: "job-1", JobDisplayName: "build", OwnerName: "acme"},
		}
		setName, backendName, merged := mergeJobMetadata(existing, "set-2", "tart", incoming)
		if setName != "set-2" || backendName != "tart" {
			t.Fatalf("merge gave set %q backend %q, want set-2 and tart", setName, backendName)
		}
		if merged.JobID != "job-1" || merged.JobDisplayName != "build" || merged.OwnerName != "acme" {
			t.Fatalf("merged job = %+v, want job-1 build acme", merged.JobMessageBase)
		}
	})

	t.Run("empty values keep what the agent knew", func(t *testing.T) {
		t.Parallel()
		incoming := &scaleset.JobStarted{JobMessageBase: scaleset.JobMessageBase{JobID: "job-1"}}
		setName, backendName, merged := mergeJobMetadata(existing, "", "", incoming)
		if setName != "set-1" || backendName != "docker" {
			t.Fatalf("merge gave set %q backend %q, want set-1 and docker", setName, backendName)
		}
		if merged.RunnerName != "runner-1" || merged.JobDisplayName != ProvisionalDisplayName {
			t.Fatalf("merged job = %+v, want runner-1 and the provisional display name", merged)
		}
		if incoming.RunnerName != "" {
			t.Fatal("mergeJobMetadata changed its input")
		}
	})
}

func TestClosedWithoutJobNote(t *testing.T) {
	t.Parallel()
	log := "2026-10-06 10:00:00Z: Listening for Jobs\n2026-10-06 10:00:05Z: Running job: build\n"
	if !logTextShowsJobStarted(log) {
		t.Fatal("logTextShowsJobStarted() = false for a log with a job start line")
	}
	if logTextShowsJobStarted("Listening for Jobs\n") {
		t.Fatal("logTextShowsJobStarted() = true for a log without a job start line")
	}
	if closedWithoutJobNote(true) == closedWithoutJobNote(false) {
		t.Fatal("closedWithoutJobNote() gives the same note whether or not a job ran")
	}
}
