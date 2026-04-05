package api

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
	"github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1/controlplanev1connect"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

var _ controlplanev1connect.ControlPlaneServiceHandler = (*Server)(nil)

// Server implements ControlPlaneServiceHandler.
type Server struct {
	managementService *management.Service
	vitalsService     *vitals.Service
	idleTimeout       time.Duration
	corsOrigin        string
}

// NewServer creates an API server backed by the management and vitals services.
// corsOrigin controls the Access-Control-Allow-Origin header; defaults to "*".
func NewServer(managementService *management.Service, vitalsService *vitals.Service, idleTimeout time.Duration, corsOrigin string) *Server {
	if corsOrigin == "" {
		corsOrigin = "*"
	}
	return &Server{
		managementService: managementService,
		vitalsService:     vitalsService,
		idleTimeout:       idleTimeout,
		corsOrigin:        corsOrigin,
	}
}

// Handler returns the HTTP handler for the Connect RPC service with CORS support.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	path, handler := controlplanev1connect.NewControlPlaneServiceHandler(s)
	mux.Handle(path, handler)
	return withCORS(mux, s.corsOrigin)
}

func (s *Server) GetServiceInfo(_ context.Context, _ *connect.Request[controlplanev1.GetServiceInfoRequest]) (*connect.Response[controlplanev1.GetServiceInfoResponse], error) {
	return connect.NewResponse(&controlplanev1.GetServiceInfoResponse{
		Version:            controller.Version,
		CommitSha:          controller.CommitSHA,
		StartedAt:          timestamppb.New(s.vitalsService.StartedAt()),
		IdleTimeoutSeconds: int32(s.idleTimeout.Seconds()),
	}), nil
}

func (s *Server) ListRunnerSets(_ context.Context, _ *connect.Request[controlplanev1.ListRunnerSetsRequest]) (*connect.Response[controlplanev1.ListRunnerSetsResponse], error) {
	views := s.managementService.ListRunnerSets()
	sets := make([]*controlplanev1.RunnerSet, 0, len(views))
	for _, v := range views {
		runners := make([]*controlplanev1.Runner, 0, len(v.Runners))
		for _, r := range v.Runners {
			runners = append(runners, &controlplanev1.Runner{
				Name:  r.Name,
				State: toProtoRunnerState(r.State),
				Since: timestamppb.New(r.Since),
			})
		}
		sets = append(sets, &controlplanev1.RunnerSet{
			Name:       v.Info.Name,
			Backend:    toProtoBackend(v.Info.Backend),
			Image:      v.Info.Image,
			Labels:     v.Info.Labels,
			MaxRunners: int32(v.Info.MaxRunners),
			Scope:      v.Scope,
			Connected:  v.Connected,
			Runners:    runners,
		})
	}
	return connect.NewResponse(&controlplanev1.ListRunnerSetsResponse{
		RunnerSets: sets,
	}), nil
}

func (s *Server) ListJobRecords(_ context.Context, _ *connect.Request[controlplanev1.ListJobRecordsRequest]) (*connect.Response[controlplanev1.ListJobRecordsResponse], error) {
	jobs := s.managementService.ListJobRecords()
	records := make([]*controlplanev1.JobRecord, 0, len(jobs))
	for _, j := range jobs {
		rec := &controlplanev1.JobRecord{
			Id:            j.ID,
			RunnerName:    j.RunnerName,
			RunnerSetName: j.RunnerSetName,
			Result:        toProtoJobResult(j.Result),
			StartedAt:     timestamppb.New(j.StartedAt),
		}
		if j.CompletedAt != nil {
			rec.CompletedAt = timestamppb.New(*j.CompletedAt)
		}
		records = append(records, rec)
	}
	return connect.NewResponse(&controlplanev1.ListJobRecordsResponse{
		JobRecords: records,
	}), nil
}

func (s *Server) GetMachineVitals(_ context.Context, _ *connect.Request[controlplanev1.GetMachineVitalsRequest]) (*connect.Response[controlplanev1.GetMachineVitalsResponse], error) {
	v := s.vitalsService.GetVitals()
	return connect.NewResponse(&controlplanev1.GetMachineVitalsResponse{
		CpuUsagePercent:    v.CPUUsagePercent,
		MemoryUsagePercent: v.MemoryUsagePercent,
		DiskUsagePercent:   v.DiskUsagePercent,
		TemperatureCelsius: v.TemperatureCelsius,
	}), nil
}

func toProtoRunnerState(s controller.RunnerState) controlplanev1.RunnerState {
	switch s {
	case controller.StatePreparing:
		return controlplanev1.RunnerState_RUNNER_STATE_PREPARING
	case controller.StateIdle:
		return controlplanev1.RunnerState_RUNNER_STATE_IDLE
	case controller.StateBusy:
		return controlplanev1.RunnerState_RUNNER_STATE_BUSY
	default:
		return controlplanev1.RunnerState_RUNNER_STATE_UNSPECIFIED
	}
}

func toProtoBackend(b string) controlplanev1.Backend {
	switch b {
	case "tart":
		return controlplanev1.Backend_BACKEND_TART
	case "docker":
		return controlplanev1.Backend_BACKEND_DOCKER
	default:
		return controlplanev1.Backend_BACKEND_UNSPECIFIED
	}
}

func toProtoJobResult(r string) controlplanev1.JobResult {
	switch r {
	case "running":
		return controlplanev1.JobResult_JOB_RESULT_RUNNING
	case "Succeeded":
		return controlplanev1.JobResult_JOB_RESULT_SUCCESS
	case "Failed":
		return controlplanev1.JobResult_JOB_RESULT_FAILURE
	default:
		return controlplanev1.JobResult_JOB_RESULT_UNSPECIFIED
	}
}

// withCORS wraps a handler with CORS headers. The allowed origin is
// configurable via the cors_origin config field (defaults to "*").
func withCORS(h http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
