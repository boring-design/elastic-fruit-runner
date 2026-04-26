Feature: Job store lifecycle

  The job store persists runner job records in SQLite, tracking
  start and completion events, and provides ordered snapshots
  for the management dashboard.

  Background:
    Given a fresh in-memory job store

  Scenario: recording a job start
    When I record job "job-1" started on runner "runner-1" in set "set-a"
    Then the snapshot should contain 1 job
    And job "job-1" should have runner name "runner-1"
    And job "job-1" should have runner set name "set-a"
    And job "job-1" should have result "running"
    And job "job-1" should not have a completion time

  Scenario: recording a job completion
    Given job "job-1" was started on runner "runner-1" in set "set-a"
    When I record job "job-1" completed with result "succeeded"
    Then job "job-1" should have result "succeeded"
    And job "job-1" should have a completion time

  Scenario: completing an unknown job (orphan)
    When I record job "orphan-job" completed with result "failed"
    Then job "orphan-job" should have result "failed"
    And job "orphan-job" should have runner name ""

  Scenario: snapshot returns jobs in most-recent-first order
    Given the following jobs were started:
      | job_id | runner    | set   |
      | job-1  | runner-1  | set-a |
      | job-2  | runner-2  | set-a |
      | job-3  | runner-3  | set-a |
    Then the snapshot should have jobs in order: job-3, job-2, job-1

  Scenario: snapshot is limited to 200 entries
    Given 250 jobs were started in set "set-a"
    Then the snapshot should contain 200 jobs

  Scenario: rejecting a job completion with unexpected result
    Given job "job-1" was started on runner "runner-1" in set "set-a"
    When I record job "job-1" completed with result "cancelled"
    Then job "job-1" should have result "running"
    And job "job-1" should not have a completion time

  Scenario: concurrent access is safe
    When 50 jobs are started and completed concurrently in set "set-a"
    Then the snapshot should contain 50 jobs
