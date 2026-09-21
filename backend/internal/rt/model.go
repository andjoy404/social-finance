package rt

import "time"

// RT represents a RT (Rukun Tetangga) organization.
type RT struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RW        int       `json:"rw"`
	RT        string    `json:"rt"`
	Address   *string   `json:"address,omitempty"`
	HeadName  *string   `json:"head_name,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRTInput holds the required fields for creating a new RT.
type CreateRTInput struct {
	Name     string
	RW       int
	RT       string
	Address  *string
	HeadName *string
}

// UpdateRTInput holds optional fields for updating an existing RT.
// A nil pointer means that field should not be updated.
type UpdateRTInput struct {
	Name     *string
	RW       *int
	RT       *string
	Address  *string
	HeadName *string
	IsActive *bool
}
