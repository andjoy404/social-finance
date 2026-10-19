package rt

import (
	"bytes"
	"encoding/json"
)

// OptionalString distinguishes an omitted JSON field from explicit null.
// It is intended for canonical PATCH request parsing; legacy handlers remain
// unchanged until the community migration is enabled.
type OptionalString struct {
	Set   bool
	Valid bool
	Value string
}

func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Valid = false
		o.Value = ""
		return nil
	}
	o.Valid = true
	return json.Unmarshal(data, &o.Value)
}

func (o OptionalString) IsOmitted() bool { return !o.Set }
func (o OptionalString) IsNull() bool    { return o.Set && !o.Valid }
func (o OptionalString) HasValue() bool  { return o.Set && o.Valid }
