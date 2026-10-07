package cloudagent

import "strings"

// placeholderJobIDPrefix marks a local job record opened for a runner before
// the control plane named the job. Real GitHub job ids never start with it.
const placeholderJobIDPrefix = "pending-"

// placeholderJobID derives the local record id for a runner that has no job
// id yet.
func placeholderJobID(runnerName string) string {
	return placeholderJobIDPrefix + runnerName
}

// isPlaceholderJobID reports whether a job id belongs to a provisional record
// the cloud does not know about.
func isPlaceholderJobID(jobID string) bool {
	return strings.HasPrefix(jobID, placeholderJobIDPrefix)
}
