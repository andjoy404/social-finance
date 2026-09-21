package household

import (
	"fmt"
	"strings"
	"time"
)

// OccupancyStatus represents the occupancy status of a household.
type OccupancyStatus string

const (
	OccupancyOwner  OccupancyStatus = "OWNER"
	OccupancyTenant OccupancyStatus = "TENANT"
)

// Validate checks that s is a recognized occupancy status value.
func (s OccupancyStatus) Validate() error {
	if s == "" || s == OccupancyOwner || s == OccupancyTenant {
		return nil
	}
	return fmt.Errorf("invalid occupancy status: %q", s)
}

// IsValid checks if an occupancy_status value is in the allowed set.
func (o OccupancyStatus) IsValid() bool {
	return o == OccupancyOwner || o == OccupancyTenant
}

// Canonical relationship value for household head.
const RelationshipHead = "HEAD"

// Household represents a household within an RT with its current physical house and occupancy.
type Household struct {
	ID              string           `json:"id"`
	RTID            string           `json:"rt_id"`
	HouseNumber     *string          `json:"house_number,omitempty"`
	HeadName        string           `json:"head_name"`
	Address         *string          `json:"address,omitempty"`
	OccupancyStatus *OccupancyStatus `json:"occupancy_status,omitempty"`
	Nik             *string          `json:"nik,omitempty"`
	Phone           *string          `json:"phone,omitempty"`
	Email           *string          `json:"email,omitempty"`
	HeadResident    *Resident        `json:"head_resident,omitempty"`
	IsActive        bool             `json:"is_active"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// CreateHouseholdInput holds the fields for creating a new household.
// RTID is populated by the handler from auth context — never from client input.
type CreateHouseholdInput struct {
	RTID            string          `json:"-"`
	RequestedRTID   *string         `json:"rt_id,omitempty"`
	HouseNumber     string          `json:"house_number"`
	HeadName        string          `json:"head_name"`
	FullName        *string         `json:"full_name,omitempty"`
	Nik             *string         `json:"nik,omitempty"`
	Phone           *string         `json:"phone,omitempty"`
	Email           *string         `json:"email,omitempty"`
	Address         *string         `json:"address,omitempty"`
	OccupancyStatus OccupancyStatus `json:"occupancy_status"`
	StartDate       *string         `json:"start_date,omitempty"` // YYYY-MM-DD
}

// UpdateHouseholdInput holds optional fields for updating a household.
type UpdateHouseholdInput struct {
	HouseNumber     *string          `json:"house_number,omitempty"`
	HeadName        *string          `json:"head_name,omitempty"`
	FullName        *string          `json:"full_name,omitempty"`
	Nik             *string          `json:"nik,omitempty"`
	Phone           *string          `json:"phone,omitempty"`
	Email           *string          `json:"email,omitempty"`
	Address         *string          `json:"address,omitempty"`
	OccupancyStatus *OccupancyStatus `json:"occupancy_status,omitempty"`
	IsActive        *bool            `json:"is_active,omitempty"`
}

// MoveHouseholdInput holds the fields required to move a household to a new physical house.
type MoveHouseholdInput struct {
	HouseNumber     string           `json:"house_number"`
	Address         *string          `json:"address,omitempty"`
	StartDate       string           `json:"start_date"` // YYYY-MM-DD (Required)
	OccupancyStatus *OccupancyStatus `json:"occupancy_status,omitempty"`
}

// Resident represents a resident linked to a household.
type Resident struct {
	ID                 string    `json:"id"`
	RTID               string    `json:"rt_id"`
	HouseholdID        *string   `json:"household_id,omitempty"`
	FullName           string    `json:"full_name"`
	Phone              *string   `json:"phone,omitempty"`
	Nik                *string   `json:"nik,omitempty"`
	Email              *string   `json:"email,omitempty"`
	RelationshipToHead *string   `json:"relationship_to_head,omitempty"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateResidentInput holds the fields for creating a new resident.
type CreateResidentInput struct {
	HouseholdID        string  `json:"household_id"`
	FullName           string  `json:"full_name"`
	Phone              *string `json:"phone"`
	Nik                *string `json:"nik"`
	Email              *string `json:"email"`
	RelationshipToHead *string `json:"relationship_to_head"`
	StartDate          *string `json:"start_date,omitempty"` // YYYY-MM-DD
}

// MoveResidentInput holds fields for moving a resident to another household.
type MoveResidentInput struct {
	DestinationHouseholdID string  `json:"destination_household_id"`
	StartDate              string  `json:"start_date"` // YYYY-MM-DD (Required)
	RelationshipToHead     *string `json:"relationship_to_head,omitempty"`
}

// TrimAndValidateHouseNumber trims whitespace and validates the house_number.
func (in *CreateHouseholdInput) TrimAndValidateHouseNumber() error {
	trimmed := strings.TrimSpace(in.HouseNumber)
	if trimmed == "" {
		return fmt.Errorf("house_number is required")
	}
	in.HouseNumber = trimmed
	return nil
}

// UpdateResidentInput holds optional fields for updating a resident.
type UpdateResidentInput struct {
	FullName           *string `json:"full_name"`
	Phone              *string `json:"phone"`
	Nik                *string `json:"nik"`
	Email              *string `json:"email"`
	RelationshipToHead *string `json:"relationship_to_head"`
	IsActive           *bool   `json:"is_active"`
}

// ValidateNik validates that the NIK (national ID) is exactly 16 digits.
func ValidateNik(nik string) error {
	trimmed := strings.TrimSpace(nik)
	if trimmed == "" {
		return fmt.Errorf("nik is required")
	}
	if len(trimmed) != 16 {
		return fmt.Errorf("nik must be exactly 16 digits")
	}
	for _, c := range trimmed {
		if c < '0' || c > '9' {
			return fmt.Errorf("nik must contain only digits")
		}
	}
	return nil
}

// ValidateEmail checks that the email has basic valid structure if non-empty.
func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(trimmed, "@") || !strings.Contains(trimmed, ".") {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// NormalizePhone converts Indonesian phone input to canonical +62 format.
// Accepts: 081234567890, 6281234567890, +6281234567890
// Returns: +6281234567890
func NormalizePhone(phone string) (string, error) {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return "", fmt.Errorf("phone is required")
	}
	canon := trimmed
	if strings.HasPrefix(canon, "+62") {
		// Strip leading +0 if present
		if len(canon) > 3 && canon[3] == '0' {
			canon = "+62" + canon[4:]
		}
	} else if strings.HasPrefix(canon, "08") {
		canon = "+62" + canon[1:]
	} else if strings.HasPrefix(canon, "62") {
		canon = "+" + canon
	} else {
		return "", fmt.Errorf("phone number format is invalid")
	}
	// Strip any internal dashes or spaces that may remain
	var sb strings.Builder
	for _, c := range canon {
		if (c >= '0' && c <= '9') || c == '+' {
			sb.WriteByte(byte(c))
		}
	}
	canon = sb.String()
	return canon, nil
}

// NormalizeEmail trims whitespace and lowercases the email.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
