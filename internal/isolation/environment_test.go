package isolation

import (
	"strings"
	"testing"
)

func TestCleanEnvironmentKeepsRuntimeValuesAndDropsSecrets(t *testing.T) {
	t.Setenv("PATH", "/trusted/bin")
	t.Setenv("SPARE_TEST_SECRET", "must-not-leak")
	t.Setenv("HTTP_PROXY", "http://untrusted.invalid")
	environment := strings.Join(cleanEnvironment(), "\n")
	if !strings.Contains(environment, "PATH=/trusted/bin") {
		t.Fatalf("PATH missing from clean environment: %q", environment)
	}
	if strings.Contains(environment, "SPARE_TEST_SECRET") || strings.Contains(strings.ToUpper(environment), "HTTP_PROXY") {
		t.Fatalf("sensitive environment leaked to worker: %q", environment)
	}
}
