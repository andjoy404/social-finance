package rt

import "time"

// RW represents an RW (Rukun Warga) organization.
type RW struct {
	ID        string    `json:"id"`
	RWNumber  int       `json:"rw_number"`
	Name      string    `json:"name"`
	HeadName  *string   `json:"head_name,omitempty"`
	Address   *string   `json:"address,omitempty"`
	RTCount   int       `json:"rt_count"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRWInput holds the required fields for creating a new RW.
type CreateRWInput struct {
	RWNumber int
	Name     string
	HeadName *string
	Address  *string
}

// UpdateRWInput holds optional fields for updating an existing RW.
type UpdateRWInput struct {
	RWNumber *int
	Name     *string
	HeadName *string
	Address  *string
	IsActive *bool
}

// RT represents a RT (Rukun Tetangga) organization.
type RT struct {
	ID        string    `json:"id"`
	RWID      *string   `json:"rw_id,omitempty"`
	RWName    *string   `json:"rw_name,omitempty"`
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
	RWID     *string
	RW       int
	RT       string
	Address  *string
	HeadName *string
}

// UpdateRTInput holds optional fields for updating an existing RT.
// A nil pointer means that field should not be updated.
type UpdateRTInput struct {
	Name     *string
	RWID     *string
	RW       *int
	RT       *string
	Address  *string
	HeadName *string
	IsActive *bool
}
