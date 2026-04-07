package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/boring-design/elastic-fruit-runner/config"
	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
	"github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1/controlplanev1connect"
	"github.com/boring-design/elastic-fruit-runner/internal/api"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

func TestNewServer_DefaultCORS(t *testing.T) {
	t.Parallel()
	svc := vitals.New(time.Now())
	srv := api.NewServer(nil, svc, 5*time.Minute, config.CORSConfig{})
	handler := srv.Handler()

	// Verify CORS defaults are applied via an OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST, OPTIONS")
	}
}

func TestNewServer_CustomCORS(t *testing.T) {
	t.Parallel()
	svc := vitals.New(time.Now())
	cors := config.CORSConfig{
		AllowOrigin:      "https://example.com",
		AllowMethods:     "GET, POST",
		AllowHeaders:     "Authorization",
		ExposeHeaders:    "X-Custom",
		AllowCredentials: true,
		MaxAge:           3600,
	}
	srv := api.NewServer(nil, svc, 5*time.Minute, cors)
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodOptions, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Fatalf("Access-Control-Max-Age = %q, want %q", got, "3600")
	}
}

func TestGetServiceInfo(t *testing.T) {
	t.Parallel()
	startTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	vitalsSvc := vitals.New(startTime)
	idleTimeout := 10 * time.Minute

	srv := api.NewServer(nil, vitalsSvc, idleTimeout, config.CORSConfig{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := controlplanev1connect.NewControlPlaneServiceClient(ts.Client(), ts.URL)
	resp, err := client.GetServiceInfo(context.Background(), connect.NewRequest(&controlplanev1.GetServiceInfoRequest{}))
	if err != nil {
		t.Fatalf("GetServiceInfo() error: %v", err)
	}

	if resp.Msg.IdleTimeoutSeconds != 600 {
		t.Errorf("IdleTimeoutSeconds = %d, want 600", resp.Msg.IdleTimeoutSeconds)
	}
	if !resp.Msg.StartedAt.AsTime().Equal(startTime) {
		t.Errorf("StartedAt = %v, want %v", resp.Msg.StartedAt.AsTime(), startTime)
	}
}

func TestGetMachineVitals(t *testing.T) {
	t.Parallel()
	vitalsSvc := vitals.New(time.Now())

	srv := api.NewServer(nil, vitalsSvc, 5*time.Minute, config.CORSConfig{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := controlplanev1connect.NewControlPlaneServiceClient(ts.Client(), ts.URL)
	resp, err := client.GetMachineVitals(context.Background(), connect.NewRequest(&controlplanev1.GetMachineVitalsRequest{}))
	if err != nil {
		t.Fatalf("GetMachineVitals() error: %v", err)
	}

	// Before Start(), vitals are all zero
	if resp.Msg.CpuUsagePercent != 0 {
		t.Errorf("CpuUsagePercent = %f, want 0", resp.Msg.CpuUsagePercent)
	}
}
