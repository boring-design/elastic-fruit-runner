Feature: Management service full stack

  The management service creates GitHub clients, backends, and controllers
  from configuration. The API server exposes runner set status and job
  history over Connect RPC.

  This feature exercises the full stack: management.Service → controller →
  API server, using real GitHub credentials and Docker backend.

  Scenario: full stack lifecycle with PAT auth via API
    Given a management service config with PAT auth and docker backend
    And a management service is created from the config
    And a vitals service is created
    And an API server is started
    When the management service is started
    And the controller connects within 60 seconds
    And I query the runner sets API
    Then the runner sets response should contain 1 set
    And the first runner set should have the configured name
    When a workflow is dispatched via management
    And the workflow should complete successfully within 10 minutes
    And I query the job records API
    Then there should be at least 1 job record
    When the management service is stopped
    Then the management service should shut down cleanly
