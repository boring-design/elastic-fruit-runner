package api

import (
	"testing"

	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
)

func TestToProtoJobResult_KnownValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want controlplanev1.JobResult
	}{
		{"running", controlplanev1.JobResult_JOB_RESULT_RUNNING},
		{"succeeded", controlplanev1.JobResult_JOB_RESULT_SUCCESS},
		{"Succeeded", controlplanev1.JobResult_JOB_RESULT_SUCCESS},
		{"SUCCEEDED", controlplanev1.JobResult_JOB_RESULT_SUCCESS},
		{"failed", controlplanev1.JobResult_JOB_RESULT_FAILURE},
		{"Failed", controlplanev1.JobResult_JOB_RESULT_FAILURE},
		{"canceled", controlplanev1.JobResult_JOB_RESULT_CANCELED},
		{"Canceled", controlplanev1.JobResult_JOB_RESULT_CANCELED},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := toProtoJobResult(tc.in)
			if got != tc.want {
				t.Errorf("toProtoJobResult(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestToProtoJobResult_UnknownNonEmpty_MapsToUnknown(t *testing.T) {
	t.Parallel()
	cases := []string{
		"Mysterious",
		"AbandonedByRunner",
		"in_progress",
		"queued",
	}
	for _, in := range cases {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got := toProtoJobResult(in)
			// Unknown non-empty values must map to JOB_RESULT_UNKNOWN, not
			// JOB_RESULT_UNSPECIFIED. UNSPECIFIED (enum 0) is the protobuf
			// default and gets omitted from the JSON wire format, which means
			// the frontend sees "missing field" — historically frontend then
			// defaulted completed jobs to FAILURE. UNKNOWN is a non-zero enum
			// so it survives serialization.
			if got != controlplanev1.JobResult_JOB_RESULT_UNKNOWN {
				t.Errorf("toProtoJobResult(%q) = %v, want JOB_RESULT_UNKNOWN", in, got)
			}
		})
	}
}

func TestToProtoJobResult_Empty_MapsToUnspecified(t *testing.T) {
	t.Parallel()
	got := toProtoJobResult("")
	if got != controlplanev1.JobResult_JOB_RESULT_UNSPECIFIED {
		t.Errorf("toProtoJobResult(\"\") = %v, want JOB_RESULT_UNSPECIFIED", got)
	}
}
