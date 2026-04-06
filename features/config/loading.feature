Feature: Configuration loading

  The configuration system loads settings from YAML files and environment
  variables, applies sensible defaults, and lets the environment override
  file-based values.

  Background:
    Given a clean temporary directory

  Scenario: default values when no config file is provided
    When I load the configuration without arguments
    Then the idle timeout should be "15m"
    And the log level should be "info"

  Scenario: parsing a multi-org and multi-repo config file
    Given a config file with content:
      """
      orgs:
        - org: boring-design
          auth:
            github_app:
              client_id: Iv23li_test
              installation_id: 116416405
              private_key_path: /path/to/key.pem
          runner_group: MyGroup
          runner_sets:
            - name: efr-linux-arm64
              backend: docker
              image: test-image:latest
              labels: [self-hosted, Linux, ARM64]
              max_runners: 4
              platform: linux/arm64

      repos:
        - repo: boring-design/special-repo
          auth:
            pat_token: ghp_testtoken123
          runner_sets:
            - name: repo-runner
              backend: docker
              image: repo-image:latest
              labels: [self-hosted, Linux]
              max_runners: 2

      idle_timeout: 30m
      """
    When I load the configuration with that file
    Then the idle timeout should be "30m"
    And there should be 1 org configured
    And org 0 should have name "boring-design"
    And org 0 should have runner group "MyGroup"
    And org 0 should use GitHub App auth with client ID "Iv23li_test"
    And org 0 should have 1 runner set
    And org 0 runner set 0 should have name "efr-linux-arm64"
    And org 0 runner set 0 should have max runners 4
    And org 0 runner set 0 should have platform "linux/arm64"
    And there should be 1 repo configured
    And repo 0 should have name "boring-design/special-repo"
    And repo 0 should use PAT auth with token "ghp_testtoken123"
    And repo 0 should have 1 runner set

  Scenario: custom idle timeout duration
    Given a config file with content:
      """
      idle_timeout: 45m
      """
    When I load the configuration with that file
    Then the idle timeout should be "45m"

  Scenario: log level from config file
    Given a config file with content:
      """
      log_level: debug
      """
    When I load the configuration with that file
    Then the log level should be "debug"
    And the parsed log level should be slog.LevelDebug

  Scenario: environment variable overrides log level
    Given the environment variable "LOG_LEVEL" is set to "warn"
    When I load the configuration without arguments
    Then the log level should be "warn"
