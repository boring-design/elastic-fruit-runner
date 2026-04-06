Feature: System vitals collection

  The vitals collector gathers real-time CPU, memory, and disk
  usage metrics from the host system.

  Scenario: collecting valid system metrics
    When I collect system vitals twice with a short delay
    Then CPU usage should be between 0 and 100 percent
    And memory usage should be between 0 and 100 percent (exclusive of zero)
    And disk usage should be between 0 and 100 percent (exclusive of zero)
