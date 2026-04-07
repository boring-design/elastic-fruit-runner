//go:build integration

package integration_test

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

func TestFeatures(t *testing.T) {
	configURL := os.Getenv("EFR_TEST_CONFIG_URL")
	if configURL == "" {
		t.Fatal("EFR_TEST_CONFIG_URL not set")
	}

	opts := godog.Options{
		Format:   "pretty",
		Output:   colors.Colored(os.Stdout),
		Paths:    []string{"../../features"},
		TestingT: t,
	}

	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options:             &opts,
	}

	if suite.Run() != 0 {
		t.Fatal("integration tests failed")
	}
}
