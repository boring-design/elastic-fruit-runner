Feature: Tart VM lifecycle

  The tart manager wraps the tart CLI for VM operations: pull, clone,
  start, IP discovery, SSH exec, stop, and delete. These tests require
  a real tart installation with nested virtualization enabled.

  Scenario: pull, clone, start, exec, and cleanup a VM
    Given a tart manager
    When I pull the VM image
    Then the VM image should exist locally
    When I clone a VM with a random name
    And I start the cloned VM
    And I wait for the VM IP address
    Then the VM IP should be a valid address
    When I exec "echo hello" in the VM
    Then the exec should succeed
    When I stop and delete the VM
    Then the VM should no longer exist

  Scenario: list and cleanup orphaned VMs
    Given a tart manager
    When I clone a VM with a random name
    And I start the cloned VM
    Then listing local VMs should include the cloned VM
    When I cleanup all VMs with the test prefix
    Then listing local VMs should not include the cloned VM
