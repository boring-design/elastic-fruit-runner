package cloudagent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"connectrpc.com/connect"

	"github.com/boring-design/elastic-fruit-protocol/gen/agent/v1/agentv1connect"
)

// newAgentClient builds the Connect client for the cloud server. An empty
// credential builds a client that only works for Enroll.
func newAgentClient(serverURL, credential string) (agentv1connect.AgentServiceClient, error) {
	httpClient, err := newHTTPClient(serverURL)
	if err != nil {
		return nil, err
	}
	var options []connect.ClientOption
	if credential != "" {
		options = append(options, connect.WithInterceptors(bearerAuth{credential: credential}))
	}
	return agentv1connect.NewAgentServiceClient(httpClient, serverURL, options...), nil
}

// newHTTPClient returns an HTTP/2 capable client. Plain http is only allowed
// for localhost, and there the client speaks HTTP/2 without TLS right away
// so the command stream behaves the same as over https.
func newHTTPClient(serverURL string) (*http.Client, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse cloud server url %q: %w", serverURL, err)
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport is not an *http.Transport")
	}
	transport := base.Clone()
	if parsed.Scheme == "http" {
		protocols := new(http.Protocols)
		protocols.SetUnencryptedHTTP2(true)
		transport.Protocols = protocols
	}
	return &http.Client{Transport: transport}, nil
}

// bearerAuth adds the agent credential to every unary and streaming call.
// It is a struct so the secret never prints by accident.
type bearerAuth struct {
	credential string
}

func (b bearerAuth) headerValue() string {
	return "Bearer " + b.credential
}

func (b bearerAuth) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		request.Header().Set("Authorization", b.headerValue())
		return next(ctx, request)
	}
}

func (b bearerAuth) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", b.headerValue())
		return conn
	}
}

func (bearerAuth) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
