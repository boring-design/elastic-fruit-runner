Feature: Controller lifecycle with PAT auth

  The controller connects to GitHub Actions using a Personal Access Token,
  registers a runner scale set, picks up workflow jobs, launches runners
  via the Docker backend, and cleans up after completion.

  Scenario: end-to-end workflow execution with PAT auth
    Given a GitHub scaleset client is configured using PAT auth
    And a Docker backend is initialized
    And a fresh in-memory job store
    And a controller is created with a random scale set name
    When the controller is started
    And the controller connects within 60 seconds
    And a workflow is dispatched
    Then the workflow should complete successfully within 10 minutes
    And at least one job should be recorded
    And the controller should shut down cleanly
