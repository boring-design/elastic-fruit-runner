Feature: Binary path lookup

  The binary lookup resolves executable paths by checking PATH first,
  then falling back to well-known system directories, and caching
  results for subsequent calls.

  Scenario: finding a binary in PATH
    Given the well-known dirs are cleared
    When I look up the binary "ls"
    Then the result should be an absolute path

  Scenario: falling back to well-known directories
    Given a temporary directory with a fake binary "test-fake-bin"
    And the well-known dirs are set to that directory
    When I look up the binary "test-fake-bin"
    Then the result should be the path to "test-fake-bin" in that directory

  Scenario: returning bare name when not found
    Given the well-known dirs are cleared
    When I look up the binary "nonexistent-binary-xyz-12345"
    Then the result should be "nonexistent-binary-xyz-12345"

  Scenario: caching lookup results
    Given the well-known dirs are cleared
    When I look up the binary "ls" twice
    Then both results should be identical
