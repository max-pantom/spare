package service

import (
	"strings"
	"testing"
)

func TestBuildDefinitions(t *testing.T) {
	tests := []struct {
		goos     string
		contains []string
	}{
		{"darwin", []string{"run.spare.spared", "LaunchAgents", "/Applications/Spare/spared", "<key>Umask</key>", "<integer>63</integer>"}},
		{"linux", []string{"spared.service", "WantedBy=default.target", `ExecStart="/Applications/Spare/spared"`, "NoNewPrivileges=true", "PrivateTmp=true", "MemoryDenyWriteExecute=true", "CapabilityBoundingSet="}},
		{"windows", []string{"Spare", `"/Applications/Spare/spared"`}},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			definition, err := BuildDefinition(test.goos, "/Applications/Spare/spared", "/state/spare", "/Users/test")
			if err != nil {
				t.Fatal(err)
			}
			combined := definition.Path + "\n" + definition.Name + "\n" + definition.Content
			for _, expected := range test.contains {
				if !strings.Contains(combined, expected) {
					t.Errorf("definition missing %q:\n%s", expected, combined)
				}
			}
			if test.goos != "windows" && definition.Path == "" {
				t.Error("definition path is empty")
			}
		})
	}
}

func TestDarwinDefinitionEscapesPaths(t *testing.T) {
	definition, err := BuildDefinition("darwin", `/Applications/Spare & More/spared`, `/Users/test/A&B`, "/Users/test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(definition.Content, "Spare & More") || !strings.Contains(definition.Content, "Spare &amp; More") {
		t.Fatalf("path was not XML-escaped:\n%s", definition.Content)
	}
}
