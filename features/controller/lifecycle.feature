Feature: Controller full lifecycle

  The controller connects to GitHub Actions, registers a runner scale set,
  picks up workflow jobs, launches runners via the Docker backend, and
  cleans up after completion.

  This feature requires real GitHub API access and Docker.

  Scenario: end-to-end workflow execution
    Given a GitHub scaleset client is configured
    And a Docker backend is initialized
    And a fresh in-memory job store
    And a controller is created with a random scale set name
    When the controller is started
    And the controller connects within 60 seconds
    And a workflow is dispatched
    Then the workflow should complete successfully within 10 minutes
    And at least one job should be recorded
    And the controller should shut down cleanly
