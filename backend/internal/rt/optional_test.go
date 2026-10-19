package rt

import (
	"encoding/json"
	"testing"
)

func TestOptionalStringPresence(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		set, valid bool
		value      string
	}{
		{"omitted", `{}`, false, false, ""},
		{"null", `{"rw_id":null}`, true, false, ""},
		{"value", `{"rw_id":"rw-1"}`, true, true, "rw-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req struct {
				RWID OptionalString `json:"rw_id"`
			}
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatal(err)
			}
			if req.RWID.Set != tc.set || req.RWID.Valid != tc.valid || req.RWID.Value != tc.value {
				t.Fatalf("got %+v", req.RWID)
			}
		})
	}
}
