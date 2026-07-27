package permissions

import "testing"

func TestDescribeIncludesGrantedAndDeniedNetworkAccess(t *testing.T) {
	result := Describe(Set{Network: Network{Local: true}})
	if len(result) < 2 {
		t.Fatalf("statements = %d", len(result))
	}
	if !result[0].Granted || result[1].Granted {
		t.Fatalf("unexpected network statements: %#v", result[:2])
	}
}
