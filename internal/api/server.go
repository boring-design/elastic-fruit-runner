package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/dashboard"
	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
	"github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1/controlplanev1connect"
	"github.com/boring-design/elastic-fruit-runner/internal/auth"
	"github.com/boring-design/elastic-fruit-runner/internal/buildinfo"
	"github.com/boring-design/elastic-fruit-runner/internal/configstate"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

var _ controlplanev1connect.ControlPlaneServiceHandler = (*Server)(nil)

// Server implements ControlPlaneServiceHandler.
type Server struct {
	managementService *management.Service
	vitalsService     *vitals.Service
	authService       *auth.Service
	configState       *configstate.Service
	databasePath      string
	idleTimeout       time.Duration
	cors              config.CORSConfig
}

// Dependencies contains optional console services.
type Dependencies struct {
	Auth         *auth.Service
	ConfigState  *configstate.Service
	DatabasePath string
}

// NewServer creates an API server backed by the management and vitals services.
func NewServer(managementService *management.Service, vitalsService *vitals.Service, idleTimeout time.Duration, cors config.CORSConfig, dependencies ...Dependencies) *Server {
	if cors.AllowOrigin == "" {
		cors.AllowOrigin = "*"
	}
	if cors.AllowMethods == "" {
		cors.AllowMethods = "GET, POST, OPTIONS"
	}
	if cors.AllowHeaders == "" {
		cors.AllowHeaders = "Content-Type, Connect-Protocol-Version, X-CSRF-Token"
	}
	if cors.ExposeHeaders == "" {
		cors.ExposeHeaders = "Connect-Protocol-Version"
	}
	server := &Server{
		managementService: managementService,
		vitalsService:     vitalsService,
		idleTimeout:       idleTimeout,
		cors:              cors,
	}
	if len(dependencies) > 0 {
		server.authService = dependencies[0].Auth
		server.configState = dependencies[0].ConfigState
		server.databasePath = dependencies[0].DatabasePath
	}
	return server
}

// Handler returns the HTTP handler for the Connect RPC service with CORS support.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	handlerOptions := []connect.HandlerOption{}
	if s.authService != nil {
		handlerOptions = append(handlerOptions, connect.WithInterceptors(s.authInterceptor()))
	}
	path, handler := controlplanev1connect.NewControlPlaneServiceHandler(s, handlerOptions...)
	mux.Handle(path, handler)
	mux.Handle("/", dashboard.Handler())
	return withCORS(mux, s.cors)
}

func (s *Server) GetSession(ctx context.Context, req *connect.Request[controlplanev1.GetSessionRequest]) (*connect.Response[controlplanev1.GetSessionResponse], error) {
	if s.authService == nil {
		return connect.NewResponse(&controlplanev1.GetSessionResponse{Authenticated: true}), nil
	}
	setupRequired, err := s.authService.SetupRequired(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	session, err := s.sessionFromHeader(ctx, req.Header())
	if err != nil {
		if !errors.Is(err, auth.ErrSessionNotFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&controlplanev1.GetSessionResponse{
			SetupRequired: setupRequired,
			Authenticated: false,
		}), nil
	}
	return connect.NewResponse(&controlplanev1.GetSessionResponse{
		SetupRequired: setupRequired,
		Authenticated: true,
		CsrfToken:     session.CSRFToken,
	}), nil
}

func (s *Server) SetupAdmin(ctx context.Context, req *connect.Request[controlplanev1.SetupAdminRequest]) (*connect.Response[controlplanev1.SetupAdminResponse], error) {
	if s.authService == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("console auth is disabled"))
	}
	session, err := s.authService.Setup(ctx, req.Msg.SetupCode, req.Msg.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAlreadySetup):
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		case errors.Is(err, auth.ErrInvalidSetupCode), errors.Is(err, auth.ErrInvalidPassword):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	response := connect.NewResponse(&controlplanev1.SetupAdminResponse{CsrfToken: session.CSRFToken})
	setSessionCookie(response.Header(), session)
	return response, nil
}

func (s *Server) Login(ctx context.Context, req *connect.Request[controlplanev1.LoginRequest]) (*connect.Response[controlplanev1.LoginResponse], error) {
	if s.authService == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("console auth is disabled"))
	}
	session, err := s.authService.Login(ctx, req.Msg.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrLoginBlocked):
			return nil, connect.NewError(connect.CodeResourceExhausted, err)
		case errors.Is(err, auth.ErrInvalidCredentials):
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	response := connect.NewResponse(&controlplanev1.LoginResponse{CsrfToken: session.CSRFToken})
	setSessionCookie(response.Header(), session)
	return response, nil
}

func (s *Server) Logout(ctx context.Context, req *connect.Request[controlplanev1.LogoutRequest]) (*connect.Response[controlplanev1.LogoutResponse], error) {
	session, err := s.sessionFromHeader(ctx, req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrSessionNotFound)
	}
	if req.Header().Get("X-CSRF-Token") != session.CSRFToken {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("CSRF token is not valid"))
	}
	if err := s.authService.Logout(ctx, session.Token); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	response := connect.NewResponse(&controlplanev1.LogoutResponse{})
	clearSessionCookie(response.Header())
	return response, nil
}

func (s *Server) GetServiceInfo(_ context.Context, _ *connect.Request[controlplanev1.GetServiceInfoRequest]) (*connect.Response[controlplanev1.GetServiceInfoResponse], error) {
	build := buildinfo.Current()
	return connect.NewResponse(&controlplanev1.GetServiceInfoResponse{
		BuildInfo:          toProtoBuildInfo(build),
		StartedAt:          timestamppb.New(s.vitalsService.StartedAt()),
		IdleTimeoutSeconds: int32(s.idleTimeout.Seconds()),
	}), nil
}

func (s *Server) GetDashboardSummary(_ context.Context, _ *connect.Request[controlplanev1.GetDashboardSummaryRequest]) (*connect.Response[controlplanev1.GetDashboardSummaryResponse], error) {
	response := &controlplanev1.GetDashboardSummaryResponse{GithubConnected: true}
	if s.managementService == nil {
		return connect.NewResponse(response), nil
	}

	runnerSets := s.managementService.ListRunnerSets()
	response.RunnerSetCount = int32(len(runnerSets))
	for _, runnerSet := range runnerSets {
		if !runnerSet.Connected {
			response.GithubConnected = false
		}
		for _, runner := range runnerSet.Runners {
			switch runner.State {
			case controller.StatePreparing:
				response.PreparingRunnerCount++
			case controller.StateIdle:
				response.IdleRunnerCount++
			case controller.StateBusy:
				response.BusyRunnerCount++
			}
		}
	}
	if len(runnerSets) == 0 {
		response.GithubConnected = false
	}

	for _, job := range s.managementService.ListJobRecords() {
		switch strings.ToLower(job.Result) {
		case "running":
			response.RunningJobCount++
		case "failed":
			response.FailedJobCount++
			response.CompletedJobCount++
		default:
			response.CompletedJobCount++
		}
	}
	return connect.NewResponse(response), nil
}

func toProtoBuildInfo(bi *debug.BuildInfo) *controlplanev1.BuildInfo {
	if bi == nil {
		return nil
	}

	deps := make([]*controlplanev1.Module, 0, len(bi.Deps))
	for _, dep := range bi.Deps {
		deps = append(deps, toProtoModule(dep))
	}
	settings := make([]*controlplanev1.BuildSetting, 0, len(bi.Settings))
	for _, setting := range bi.Settings {
		settings = append(settings, &controlplanev1.BuildSetting{
			Key:   setting.Key,
			Value: setting.Value,
		})
	}

	return &controlplanev1.BuildInfo{
		GoVersion: bi.GoVersion,
		Path:      bi.Path,
		Main:      toProtoModule(&bi.Main),
		Deps:      deps,
		Settings:  settings,
	}
}

func toProtoModule(module *debug.Module) *controlplanev1.Module {
	if module == nil {
		return nil
	}
	return &controlplanev1.Module{
		Path:    module.Path,
		Version: module.Version,
		Sum:     module.Sum,
		Replace: toProtoModule(module.Replace),
	}
}

func (s *Server) ListRunnerSets(_ context.Context, _ *connect.Request[controlplanev1.ListRunnerSetsRequest]) (*connect.Response[controlplanev1.ListRunnerSetsResponse], error) {
	if s.managementService == nil {
		return connect.NewResponse(&controlplanev1.ListRunnerSetsResponse{}), nil
	}
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

func (s *Server) ListJobRecords(_ context.Context, req *connect.Request[controlplanev1.ListJobRecordsRequest]) (*connect.Response[controlplanev1.ListJobRecordsResponse], error) {
	if s.managementService == nil {
		return connect.NewResponse(&controlplanev1.ListJobRecordsResponse{}), nil
	}
	cursor, _ := strconv.Atoi(req.Msg.Cursor)
	filter := management.JobFilter{
		Status:     req.Msg.Status,
		RunnerSet:  req.Msg.RunnerSet,
		Repository: req.Msg.Repository,
		Workflow:   req.Msg.Workflow,
		Cursor:     cursor,
		PageSize:   int(req.Msg.PageSize),
	}
	if req.Msg.From != nil {
		value := req.Msg.From.AsTime()
		filter.From = &value
	}
	if req.Msg.To != nil {
		value := req.Msg.To.AsTime()
		filter.To = &value
	}
	page := s.managementService.FindJobRecords(filter)
	jobs := page.Records
	records := make([]*controlplanev1.JobRecord, 0, len(jobs))
	for _, j := range jobs {
		records = append(records, toProtoJob(j))
	}
	return connect.NewResponse(&controlplanev1.ListJobRecordsResponse{
		JobRecords: records,
		NextCursor: page.NextCursor,
	}), nil
}

func (s *Server) GetJobDetail(_ context.Context, req *connect.Request[controlplanev1.GetJobDetailRequest]) (*connect.Response[controlplanev1.GetJobDetailResponse], error) {
	if s.managementService == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("job record was not found"))
	}
	job, err := s.managementService.GetJobRecord(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("job record was not found"))
	}
	return connect.NewResponse(&controlplanev1.GetJobDetailResponse{Job: toProtoJob(*job)}), nil
}

func (s *Server) GetJobLogs(_ context.Context, req *connect.Request[controlplanev1.GetJobLogsRequest]) (*connect.Response[controlplanev1.GetJobLogsResponse], error) {
	if s.managementService == nil {
		return connect.NewResponse(&controlplanev1.GetJobLogsResponse{}), nil
	}
	logs, next := s.managementService.GetJobLogs(req.Msg.JobId, req.Msg.AfterSequence, int(req.Msg.PageSize))
	lines := make([]*controlplanev1.JobLogLine, 0, len(logs))
	for _, line := range logs {
		lines = append(lines, &controlplanev1.JobLogLine{
			Sequence:   line.Sequence,
			RecordedAt: timestamppb.New(line.RecordedAt),
			Text:       line.Text,
		})
	}
	return connect.NewResponse(&controlplanev1.GetJobLogsResponse{Lines: lines, NextSequence: next}), nil
}

func (s *Server) GetJobResourceSamples(_ context.Context, req *connect.Request[controlplanev1.GetJobResourceSamplesRequest]) (*connect.Response[controlplanev1.GetJobResourceSamplesResponse], error) {
	if s.managementService == nil {
		return connect.NewResponse(&controlplanev1.GetJobResourceSamplesResponse{}), nil
	}
	samples := s.managementService.GetJobSamples(req.Msg.JobId)
	result := make([]*controlplanev1.ResourceSample, 0, len(samples))
	for _, sample := range samples {
		result = append(result, toProtoResourceSample(sample))
	}
	return connect.NewResponse(&controlplanev1.GetJobResourceSamplesResponse{Samples: result}), nil
}

func (s *Server) GetHostResourceSamples(_ context.Context, req *connect.Request[controlplanev1.GetHostResourceSamplesRequest]) (*connect.Response[controlplanev1.GetHostResourceSamplesResponse], error) {
	if s.managementService == nil {
		return connect.NewResponse(&controlplanev1.GetHostResourceSamplesResponse{}), nil
	}
	to := time.Now()
	from := to.Add(-time.Hour)
	if req.Msg.From != nil {
		from = req.Msg.From.AsTime()
	}
	if req.Msg.To != nil {
		to = req.Msg.To.AsTime()
	}
	samples, earliest := s.managementService.HostSamples(from, to)
	result := make([]*controlplanev1.ResourceSample, 0, len(samples))
	for _, sample := range samples {
		result = append(result, &controlplanev1.ResourceSample{
			RecordedAt:           timestamppb.New(sample.RecordedAt),
			Source:               "host",
			Accuracy:             controlplanev1.ResourceAccuracy_RESOURCE_ACCURACY_EXACT,
			CpuPercent:           sample.CPUPercent,
			MemoryUsedBytes:      sample.MemoryUsedBytes,
			MemoryAvailableBytes: sample.MemoryAvailableBytes,
			DiskUsedBytes:        sample.DiskUsedBytes,
			DiskAvailableBytes:   sample.DiskAvailableBytes,
			DiskReadBytes:        sample.DiskReadBytes,
			DiskWriteBytes:       sample.DiskWriteBytes,
			LoadOne:              sample.LoadOne,
			TemperatureCelsius:   sample.TemperatureCelsius,
		})
	}
	response := &controlplanev1.GetHostResourceSamplesResponse{Samples: result}
	if earliest != nil {
		response.EarliestAt = timestamppb.New(*earliest)
	}
	return connect.NewResponse(response), nil
}

func toProtoJob(job management.JobRecord) *controlplanev1.JobRecord {
	record := &controlplanev1.JobRecord{
		Id:            job.ID,
		RunnerName:    job.RunnerName,
		RunnerSetName: job.RunnerSetName,
		Result:        toProtoJobResult(job.Result),
		StartedAt:     timestamppb.New(job.StartedAt),
		Owner:         job.Owner,
		Repository:    job.Repository,
		WorkflowRef:   job.WorkflowRef,
		DisplayName:   job.DisplayName,
		WorkflowRunId: job.WorkflowRunID,
		EventName:     job.EventName,
		Labels:        job.Labels,
		Backend:       toProtoBackend(job.Backend),
	}
	if job.Owner != "" && job.Repository != "" && job.WorkflowRunID > 0 {
		record.ActionsUrl = "https://github.com/" + job.Owner + "/" + job.Repository + "/actions/runs/" + strconv.FormatInt(job.WorkflowRunID, 10)
	}
	if job.CompletedAt != nil {
		record.CompletedAt = timestamppb.New(*job.CompletedAt)
	}
	if job.QueuedAt != nil {
		record.QueuedAt = timestamppb.New(*job.QueuedAt)
	}
	if job.ScaleSetAssignedAt != nil {
		record.ScaleSetAssignedAt = timestamppb.New(*job.ScaleSetAssignedAt)
	}
	if job.RunnerAssignedAt != nil {
		record.RunnerAssignedAt = timestamppb.New(*job.RunnerAssignedAt)
	}
	return record
}

func toProtoResourceSample(sample management.ResourceSample) *controlplanev1.ResourceSample {
	accuracy := controlplanev1.ResourceAccuracy_RESOURCE_ACCURACY_EXACT
	if sample.Accuracy == "estimate" {
		accuracy = controlplanev1.ResourceAccuracy_RESOURCE_ACCURACY_ESTIMATE
	}
	return &controlplanev1.ResourceSample{
		RecordedAt:           timestamppb.New(sample.RecordedAt),
		Source:               sample.Source,
		Accuracy:             accuracy,
		CpuPercent:           sample.CPUPercent,
		MemoryUsedBytes:      sample.MemoryUsedBytes,
		MemoryAvailableBytes: sample.MemoryAvailableBytes,
		DiskUsedBytes:        sample.DiskUsedBytes,
		DiskAvailableBytes:   sample.DiskAvailableBytes,
		DiskReadBytes:        sample.DiskReadBytes,
		DiskWriteBytes:       sample.DiskWriteBytes,
		NetworkReceiveBytes:  sample.NetworkReceiveBytes,
		NetworkSendBytes:     sample.NetworkSendBytes,
	}
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

func (s *Server) GetConfigStatus(_ context.Context, _ *connect.Request[controlplanev1.GetConfigStatusRequest]) (*connect.Response[controlplanev1.GetConfigStatusResponse], error) {
	if s.configState == nil {
		return connect.NewResponse(&controlplanev1.GetConfigStatusResponse{}), nil
	}
	return connect.NewResponse(s.configStatusResponse()), nil
}

func (s *Server) configStatusResponse() *controlplanev1.GetConfigStatusResponse {
	if s.configState == nil {
		return &controlplanev1.GetConfigStatusResponse{}
	}
	snapshot := s.configState.Get()
	response := &controlplanev1.GetConfigStatusResponse{
		Path:             snapshot.Path,
		ActiveHash:       snapshot.ActiveHash,
		DiskHash:         snapshot.DiskHash,
		State:            toProtoConfigState(snapshot.State),
		ActiveLoadedAt:   timestamppb.New(snapshot.ActiveLoadedAt),
		ValidationErrors: snapshot.ValidationErrors,
		ActiveYaml:       snapshot.ActiveYAML,
		DiskYaml:         snapshot.DiskYAML,
		RestartCommands: []string{
			"brew services restart elastic-fruit-runner",
			"docker compose restart elastic-fruit-runner",
			"sudo systemctl restart elastic-fruit-runner",
		},
	}
	if snapshot.DiskModifiedAt != nil {
		response.DiskModifiedAt = timestamppb.New(*snapshot.DiskModifiedAt)
	}
	return response
}

func (s *Server) ValidateConfig(_ context.Context, req *connect.Request[controlplanev1.ValidateConfigRequest]) (*connect.Response[controlplanev1.ValidateConfigResponse], error) {
	if s.configState == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("config service is unavailable"))
	}
	result := s.configState.Validate([]byte(req.Msg.Yaml))
	return connect.NewResponse(toProtoValidation(result)), nil
}

func (s *Server) SaveConfig(ctx context.Context, req *connect.Request[controlplanev1.SaveConfigRequest]) (*connect.Response[controlplanev1.SaveConfigResponse], error) {
	if err := s.requireCSRF(ctx, req.Header()); err != nil {
		return nil, err
	}
	if s.configState == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("config service is unavailable"))
	}
	validation := s.configState.Validate([]byte(req.Msg.Yaml))
	if len(validation.Errors) > 0 {
		return connect.NewResponse(&controlplanev1.SaveConfigResponse{Validation: toProtoValidation(validation)}), nil
	}
	if len(validation.Warnings) > 0 && !req.Msg.ConfirmWarnings {
		return connect.NewResponse(&controlplanev1.SaveConfigResponse{Validation: toProtoValidation(validation)}), nil
	}
	result, err := s.configState.Save([]byte(req.Msg.Yaml), "console")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controlplanev1.SaveConfigResponse{
		Validation: toProtoValidation(result),
		Status:     s.configStatusResponse(),
	}), nil
}

func (s *Server) ListConfigRevisions(_ context.Context, _ *connect.Request[controlplanev1.ListConfigRevisionsRequest]) (*connect.Response[controlplanev1.ListConfigRevisionsResponse], error) {
	if s.configState == nil {
		return connect.NewResponse(&controlplanev1.ListConfigRevisionsResponse{}), nil
	}
	revisions, err := s.configState.Revisions()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result := make([]*controlplanev1.ConfigRevision, 0, len(revisions))
	for _, revision := range revisions {
		result = append(result, &controlplanev1.ConfigRevision{
			Id:         revision.ID,
			CreatedAt:  timestamppb.New(revision.CreatedAt),
			Source:     revision.Source,
			ConfigHash: revision.Hash,
		})
	}
	return connect.NewResponse(&controlplanev1.ListConfigRevisionsResponse{Revisions: result}), nil
}

func (s *Server) RestoreConfigRevision(ctx context.Context, req *connect.Request[controlplanev1.RestoreConfigRevisionRequest]) (*connect.Response[controlplanev1.RestoreConfigRevisionResponse], error) {
	if err := s.requireCSRF(ctx, req.Header()); err != nil {
		return nil, err
	}
	if s.configState == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("config service is unavailable"))
	}
	if err := s.configState.Restore(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&controlplanev1.RestoreConfigRevisionResponse{Status: s.configStatusResponse()}), nil
}

func toProtoValidation(result config.ValidationResult) *controlplanev1.ValidateConfigResponse {
	response := &controlplanev1.ValidateConfigResponse{NormalizedYaml: result.Normalized}
	for _, issue := range result.Errors {
		response.Errors = append(response.Errors, &controlplanev1.ConfigValidationIssue{Path: issue.Path, Message: issue.Message})
	}
	for _, issue := range result.Warnings {
		response.Warnings = append(response.Warnings, &controlplanev1.ConfigValidationIssue{Path: issue.Path, Message: issue.Message})
	}
	return response
}

func (s *Server) GetSystemInfo(_ context.Context, _ *connect.Request[controlplanev1.GetSystemInfoRequest]) (*connect.Response[controlplanev1.GetSystemInfoResponse], error) {
	response := &controlplanev1.GetSystemInfoResponse{
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		DatabasePath: s.databasePath,
	}
	if s.databasePath != "" {
		if info, err := os.Stat(s.databasePath); err == nil {
			response.DatabaseSizeBytes = info.Size()
		}
	}
	return connect.NewResponse(response), nil
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
	switch strings.ToLower(r) {
	case "running":
		return controlplanev1.JobResult_JOB_RESULT_RUNNING
	case "succeeded":
		return controlplanev1.JobResult_JOB_RESULT_SUCCESS
	case "failed":
		return controlplanev1.JobResult_JOB_RESULT_FAILURE
	case "canceled":
		return controlplanev1.JobResult_JOB_RESULT_CANCELED
	default:
		return controlplanev1.JobResult_JOB_RESULT_UNSPECIFIED
	}
}

func toProtoConfigState(state configstate.State) controlplanev1.ConfigSyncState {
	switch state {
	case configstate.StateInSync:
		return controlplanev1.ConfigSyncState_CONFIG_SYNC_STATE_IN_SYNC
	case configstate.StateRestartRequired:
		return controlplanev1.ConfigSyncState_CONFIG_SYNC_STATE_RESTART_REQUIRED
	case configstate.StateDiskInvalid:
		return controlplanev1.ConfigSyncState_CONFIG_SYNC_STATE_DISK_INVALID
	default:
		return controlplanev1.ConfigSyncState_CONFIG_SYNC_STATE_UNSPECIFIED
	}
}

func (s *Server) authInterceptor() connect.Interceptor {
	publicProcedures := map[string]struct{}{
		controlplanev1connect.ControlPlaneServiceGetSessionProcedure: {},
		controlplanev1connect.ControlPlaneServiceSetupAdminProcedure: {},
		controlplanev1connect.ControlPlaneServiceLoginProcedure:      {},
	}
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if _, public := publicProcedures[req.Spec().Procedure]; public {
				return next(ctx, req)
			}
			if _, err := s.sessionFromHeader(ctx, req.Header()); err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, auth.ErrSessionNotFound)
			}
			return next(ctx, req)
		}
	})
}

func (s *Server) sessionFromHeader(ctx context.Context, header http.Header) (auth.Session, error) {
	request := &http.Request{Header: header}
	cookie, err := request.Cookie(auth.SessionCookieName)
	if err != nil {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	return s.authService.FindSession(ctx, cookie.Value)
}

func (s *Server) requireCSRF(ctx context.Context, header http.Header) error {
	session, err := s.sessionFromHeader(ctx, header)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, auth.ErrSessionNotFound)
	}
	if header.Get("X-CSRF-Token") != session.CSRFToken {
		return connect.NewError(connect.CodePermissionDenied, errors.New("CSRF token is not valid"))
	}
	return nil
}

func setSessionCookie(header http.Header, session auth.Session) {
	cookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	header.Add("Set-Cookie", cookie.String())
}

func clearSessionCookie(header http.Header) {
	cookie := &http.Cookie{
		Name:     auth.SessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	header.Add("Set-Cookie", cookie.String())
}

// withCORS wraps a handler with CORS headers based on the provided configuration.
func withCORS(h http.Handler, cors config.CORSConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", cors.AllowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", cors.AllowMethods)
		w.Header().Set("Access-Control-Allow-Headers", cors.AllowHeaders)
		w.Header().Set("Access-Control-Expose-Headers", cors.ExposeHeaders)
		if cors.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if cors.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cors.MaxAge))
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
