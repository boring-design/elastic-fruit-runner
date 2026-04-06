package bdd_test

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

func TestFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("BDD integration tests skipped in short mode")
	}

	opts := godog.Options{
		Format:   "pretty",
		Output:   colors.Colored(os.Stdout),
		Paths:    []string{"../../features"},
		TestingT: t,
		Tags:     "~@external",
	}

	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options:             &opts,
	}

	if suite.Run() != 0 {
		t.Fatal("BDD tests failed")
	}
}

func TestExternalFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("BDD integration tests skipped in short mode")
	}

	configURL := os.Getenv("EFR_TEST_CONFIG_URL")
	if configURL == "" {
		t.Skip("EFR_TEST_CONFIG_URL not set, skipping external integration tests")
	}

	opts := godog.Options{
		Format:   "pretty",
		Output:   colors.Colored(os.Stdout),
		Paths:    []string{"../../features"},
		TestingT: t,
		Tags:     "@external",
	}

	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options:             &opts,
	}

	if suite.Run() != 0 {
		t.Fatal("BDD external tests failed")
	}
}
