package controller

import (
	"strconv"
	"strings"
)

// repositoryFromJobMessage builds an "owner/repo" identifier from the
// scaleset.JobMessageBase fields. Returns an empty string if either part
// is missing so downstream consumers can render a graceful fallback.
func repositoryFromJobMessage(owner, repo string) string {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return ""
	}
	return owner + "/" + repo
}

// workflowNameFromJobMessage returns the human-readable workflow name.
// scaleset.JobDisplayName is preferred (e.g. "Unit Test"); when absent we
// fall back to extracting the workflow file basename from JobWorkflowRef
// (which has the form "<owner>/<repo>/.github/workflows/<file>@<ref>").
func workflowNameFromJobMessage(displayName, workflowRef string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		return displayName
	}
	workflowRef = strings.TrimSpace(workflowRef)
	if workflowRef == "" {
		return ""
	}
	if at := strings.IndexByte(workflowRef, '@'); at >= 0 {
		workflowRef = workflowRef[:at]
	}
	if slash := strings.LastIndexByte(workflowRef, '/'); slash >= 0 {
		return workflowRef[slash+1:]
	}
	return workflowRef
}

// workflowRunIDFromJobMessage formats the numeric workflow run ID as a
// string. Returns empty when the upstream value is zero (unset) so the
// frontend can hide the link gracefully.
func workflowRunIDFromJobMessage(workflowRunID int64) string {
	if workflowRunID == 0 {
		return ""
	}
	return strconv.FormatInt(workflowRunID, 10)
}
