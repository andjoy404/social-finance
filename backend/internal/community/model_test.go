package community

import "testing"

func TestValidCode(t *testing.T) {
	valid := []string{"test-community", "a", "community-01"}
	for _, code := range valid {
		if !ValidCode(code) {
			t.Errorf("expected valid community code %q", code)
		}
	}
	invalid := []string{"Test-Community", "test_community", "-test", "test-", "", "x" + string(make([]byte, 63))}
	for _, code := range invalid {
		if ValidCode(code) {
			t.Errorf("expected invalid community code %q", code)
		}
	}
}
