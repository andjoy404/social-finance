package devseed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Helper function to create string pointers.
func strPtr(s string) *string {
	return &s
}

type RTSeed struct {
	ID       string
	Name     string
	RW       int
	RT       string
	Address  string
	HeadName string
	IsActive bool
}

type PhysicalHouseSeed struct {
	ID          string
	RTID        string
	HouseNumber string
	Address     string
	IsActive    bool
}

type HouseholdSeed struct {
	ID       string
	RTID     string
	HeadName string
	IsActive bool
}

type HouseholdOccupancySeed struct {
	ID              string
	PhysicalHouseID string
	HouseholdID     string
	OccupancyStatus string
	StartDate       *string
	EndDate         *string
}

type ResidentSeed struct {
	ID       string
	RTID     string
	FullName string
	Phone    string
	IsActive bool
	NIK      *string
	Email    *string
}

type ResidencyPeriodSeed struct {
	ID                   string
	ResidentID           string
	HouseholdOccupancyID string
	StartDate            *string
	EndDate              *string
}

type UserSeed struct {
	ID       string
	Email    string
	Phone    string
	FullName string
	Role     string
	RTID     string
}

// PositionSeed defines a position-based seed user: a new user with
// system_role=pengurus (not stored as system_role in the struct since all
// positions use "pengurus") assigned a jabatan and a single RT membership.
type PositionSeed struct {
	ID       string
	Email    string
	Phone    string
	FullName string
	Jabatan  string
	RTID     string
	NIK      string
}

// PositionResidentSeed defines data needed to create a resident record
// linked to a position user account and its own household chain.
type PositionResidentSeed struct {
	UserID             string
	FullName           string
	Phone              string
	NIK                string
	Email              *string
	RTID               string
	OccupancyID        string
	// Derived household chain for eligible positions
	HouseID         string
	HouseholdID     string
	HouseNumber     string
	HouseAddress    string
	HeadName        string
	EligiblePosIdx  int
	EligiblePosName string
	PositionIdx     int
	IsEligible      bool
}

// SpecialPositionResidentSeed defines data needed to create a resident record
// for a non-eligible position user (security/cleaning).
type SpecialPositionResidentSeed struct {
	UserID   string
	FullName string
	RTID     string
	NIK      string
	Email    *string
}

// FinancialCategorySeed defines data for seeding financial categories.
type FinancialCategorySeed struct {
	ID       string
	RTID     string
	Name     string
	Type     string // "income" or "expense"
	IsActive bool
}

// DueSeed defines data for seeding dues.
type DueSeed struct {
	ID         string
	RTID       string
	Name       string
	Amount     string // string decimal, e.g. "50000.00"
	PeriodType string // "monthly", "yearly", "one_time"
	IsActive   bool
}

// BillSeed defines data for seeding bills.
type BillSeed struct {
	ID                   string
	RTID                 string
	HouseholdOccupancyID string
	DueID                string
	Amount               string // string decimal
	Period               string // e.g. "2026-10"
	DueDate              string // YYYY-MM-DD
	Status               string // "unpaid", "partial", "paid"
}

// PaymentSeed defines data for seeding payments.
type PaymentSeed struct {
	ID              string
	RTID            string
	BillID          string
	Amount          string // string decimal
	Method          string // "CASH" or "TRANSFER"
	Origin          string // "STAFF_RECORDED" or "SELF_SUBMITTED"
	Status          string // "PENDING", "APPROVED", "REJECTED"
	VerifiedBy      string // bendahara user ID
	VerifiedAt      string // RFC3339 timestamp
	RejectionReason *string
	Notes           *string
}

// positionResidentData contains the derived resident records for position users
// who are eligible (ketua, wakil_ketua, sekretaris, bendahara, sosial).
var positionResidentData []PositionResidentSeed

// specialPositionResidentData contains the derived resident records for
// non-eligible position users (keamanan, kebersihan_pembangunan).
// These are special residents: linked to their user account, assigned to their RT,
// but WITHOUT household/occupancy/residency chain.
var specialPositionResidentData []SpecialPositionResidentSeed

// CanonicalFinancialCategories contains 2 categories per RT (income + expense).
var CanonicalFinancialCategories []FinancialCategorySeed

// CanonicalDues contains 3 dues per RT (Kebersihan, Keamanan, Kas RT).
var CanonicalDues []DueSeed

// CanonicalBills contains bill samples linked to existing occupancies + dues.
var CanonicalBills []BillSeed

// CanonicalPayments contains payment samples linked to bills.
var CanonicalPayments []PaymentSeed

// CanonicalRTs contains the operational RTs (RT 003 and RT 002).
var CanonicalRTs = []RTSeed{
	{ID: "8dfa1e36-6727-45be-8fea-24d958efc155", Name: "Wisma Rukun Tunggal", RW: 16, RT: "003", Address: "Jl. Gading Raya Blok D", HeadName: "Hartono Wijaya", IsActive: true},
	{ID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", Name: "Urena", RW: 16, RT: "002", Address: "GNI Urena", HeadName: "Atom", IsActive: true},
}

// CanonicalPhysicalHouses contains the physical houses for RT 003 and RT 002.
var CanonicalPhysicalHouses = []PhysicalHouseSeed{
	// RT 03
	{ID: "bbbb0001-0001-4c33-8000-000000000001", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "01", Address: "Jl. Gading Raya Blok D No. 01", IsActive: true},
	{ID: "bbbb0001-0002-4c33-8000-000000000002", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "02", Address: "Jl. Gading Raya Blok D No. 02", IsActive: true},
	{ID: "bbbb0001-0003-4c33-8000-000000000003", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "03", Address: "Jl. Gading Raya Blok D No. 03", IsActive: true},
	{ID: "bbbb0001-0004-4c33-8000-000000000004", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "04", Address: "Jl. Gading Raya Blok D No. 04", IsActive: true},
	{ID: "bbbb0001-0005-4c33-8000-000000000005", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "05", Address: "Jl. Gading Raya Blok D No. 05", IsActive: true},
	// RT 002 (Urena)
	{ID: "aabb1122-0001-4c33-8000-000000000001", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/10", Address: "GNI Blok Urena", IsActive: true},
	{ID: "ab799cca-f833-4504-8a4e-ef715d407449", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/12", Address: "Jakarta Timur", IsActive: true},
	{ID: "dd9c1056-29eb-4211-8468-df25323cb4be", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/14", Address: "GNI Blok Urena", IsActive: true},
	// Aqueen, Azis, Namah: masing-masing rumah tangga independen
	{ID: "aabb1122-0003-4c33-8000-000000000001", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/26", Address: "GNI Blok Urena", IsActive: true},
	{ID: "aabb1122-0004-4c33-8000-000000000002", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/28", Address: "GNI Blok Urena", IsActive: true},
	{ID: "aabb1122-0005-4c33-8000-000000000003", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/30", Address: "GNI Blok Urena", IsActive: true},
}

// CanonicalHouseholds contains the households for RT 002 and RT 003.
var CanonicalHouseholds = []HouseholdSeed{
	{ID: "aabb1122-0001-4d44-8000-000000000001", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Aqueen Alsava", IsActive: true},
	{ID: "5360467d-4b4b-4325-9609-dde8c780b12e", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Andriyan", IsActive: true},
	{ID: "9d1f520c-54d3-46b3-b4bc-69a8edf0842e", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Mety Fitriani", IsActive: true},
	// Aqueen, Azis, Namah: masing-masing rumah tangga independen
	{ID: "aabb1122-0003-4d44-8000-000000000001", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Aqueen Alsava Khaylila", IsActive: true},
	{ID: "aabb1122-0004-4d44-8000-000000000002", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Muhamad Azis", IsActive: true},
	{ID: "aabb1122-0005-4d44-8000-000000000003", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Namah", IsActive: true},
	{ID: "cccc0001-0001-4d44-8000-000000000001", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Hartana Wibowo", IsActive: true},
	{ID: "cccc0001-0002-4d44-8000-000000000002", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Sri Mulyani", IsActive: true},
	{ID: "cccc0001-0003-4d44-8000-000000000003", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Dedi Kurniawan", IsActive: true},
	{ID: "cccc0001-0004-4d44-8000-000000000004", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Nurhaliza Sari", IsActive: true},
	{ID: "cccc0001-0005-4d44-8000-000000000005", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Bambang Sutrisno", IsActive: false},
}

// CanonicalOccupancies contains the household occupancies for RT 002 and RT 003.
var CanonicalOccupancies = []HouseholdOccupancySeed{
	{ID: "aaee1100-0001-4e55-8000-000000000001", PhysicalHouseID: "aabb1122-0001-4c33-8000-000000000001", HouseholdID: "aabb1122-0001-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "d23353e9-5372-41c6-bf0a-a813f1e683d1", PhysicalHouseID: "dd9c1056-29eb-4211-8468-df25323cb4be", HouseholdID: "5360467d-4b4b-4325-9609-dde8c780b12e", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-20"), EndDate: nil},
	// Aqueen: rumah tangga independen (U12/26)
	{ID: "aaee1100-0003-4e55-8000-000000000001", PhysicalHouseID: "aabb1122-0003-4c33-8000-000000000001", HouseholdID: "aabb1122-0003-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-25"), EndDate: nil},
	// Azis: rumah tangga independen (U12/28)
	{ID: "aaee1100-0004-4e55-8000-000000000002", PhysicalHouseID: "aabb1122-0004-4c33-8000-000000000002", HouseholdID: "aabb1122-0004-4d44-8000-000000000002", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-25"), EndDate: nil},
	// Namah: rumah tangga independen (U12/30)
	{ID: "aaee1100-0005-4e55-8000-000000000003", PhysicalHouseID: "aabb1122-0005-4c33-8000-000000000003", HouseholdID: "aabb1122-0005-4d44-8000-000000000003", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "dddd0001-0001-4e55-8000-000000000001", PhysicalHouseID: "bbbb0001-0001-4c33-8000-000000000001", HouseholdID: "cccc0001-0001-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2023-06-16"), EndDate: nil},
	{ID: "dddd0001-0002-4e55-8000-000000000002", PhysicalHouseID: "bbbb0001-0002-4c33-8000-000000000002", HouseholdID: "cccc0001-0002-4d44-8000-000000000002", OccupancyStatus: "OWNER", StartDate: strPtr("2024-02-01"), EndDate: nil},
	{ID: "dddd0001-0003-4e55-8000-000000000003", PhysicalHouseID: "bbbb0001-0003-4c33-8000-000000000003", HouseholdID: "cccc0001-0003-4d44-8000-000000000003", OccupancyStatus: "TENANT", StartDate: strPtr("2025-01-02"), EndDate: nil},
	{ID: "dddd0001-0004-4e55-8000-000000000004", PhysicalHouseID: "bbbb0001-0004-4c33-8000-000000000004", HouseholdID: "cccc0001-0004-4d44-8000-000000000004", OccupancyStatus: "OWNER", StartDate: strPtr("2023-09-15"), EndDate: nil},
	{ID: "dddd0001-0005-4e55-8000-000000000005", PhysicalHouseID: "bbbb0001-0005-4c33-8000-000000000005", HouseholdID: "cccc0001-0005-4d44-8000-000000000005", OccupancyStatus: "OWNER", StartDate: strPtr("2019-05-20"), EndDate: nil},
	{ID: "f68449c9-5472-43fb-853e-794887ec43f9", PhysicalHouseID: "ab799cca-f833-4504-8a4e-ef715d407449", HouseholdID: "9d1f520c-54d3-46b3-b4bc-69a8edf0842e", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-22"), EndDate: nil},
}

// CanonicalResidents contains the residents for RT 002 and RT 003.
var CanonicalResidents = []ResidentSeed{
	{ID: "1e0f80e4-6cbc-4b64-960e-4c4419c819d4", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Andriyan", Phone: "+6281200000001", IsActive: true, NIK: strPtr("3175000000000001"), Email: strPtr("andriyan@local.com")},
	{ID: "6c2a10a1-5afb-4eee-b457-69d18eddbea5", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Mety Fitriani", Phone: "+6281200000002", IsActive: true, NIK: strPtr("3175000000000002"), Email: strPtr("mety.fitriani@local.com")},
	{ID: "aaaa0002-0001-4f66-8000-000000000001", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Aqueen Alsava Khaylila", Phone: "+6281200000003", IsActive: true, NIK: strPtr("3175000000000003"), Email: strPtr("aqueen.alsava@local.com")},
	{ID: "aaaa0002-0002-4f66-8000-000000000002", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Muhamad Azis", Phone: "+6281200000004", IsActive: true, NIK: strPtr("3175000000000004"), Email: strPtr("muhamad.azis@local.com")},
	{ID: "aaaa0002-0003-4f66-8000-000000000003", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Namah", Phone: "+6281200000005", IsActive: true, NIK: strPtr("3175000000000005"), Email: nil},
	{ID: "eeee0001-0001-4f66-8000-000000000001", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Hartana Wibowo", Phone: "+628130000101", IsActive: true, NIK: strPtr("3173160301730001"), Email: strPtr("hartana.wibowo@local.com")},
	{ID: "eeee0001-0003-4f66-8000-000000000003", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Dedi Kurniawan", Phone: "+628130000103", IsActive: true, NIK: strPtr("3173160301730003"), Email: strPtr("dedi.kurniawan@local.com")},
	{ID: "eeee0001-0004-4f66-8000-000000000004", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Nurhaliza Sari", Phone: "+628130000104", IsActive: true, NIK: strPtr("3173160301730004"), Email: strPtr("nurhaliza.sari@local.com")},
	{ID: "eeee0001-0005-4f66-8000-000000000005", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Bambang Sutrisno", Phone: "+628130000105", IsActive: true, NIK: strPtr("3173160301730005"), Email: strPtr("")},
	{ID: "eeee0001-0006-4f66-8000-000000000006", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Sri Mulyani", Phone: "+628130000106", IsActive: true, NIK: strPtr("3173160301730006"), Email: strPtr("sri.mulyani@local.com")},
}

// CanonicalResidencyPeriods contains the current + historical residency periods for RT 002 and RT 003.
var CanonicalResidencyPeriods = []ResidencyPeriodSeed{
	{ID: "aacc0002-0001-4f66-8000-000000000001", ResidentID: "aaaa0002-0001-4f66-8000-000000000001", HouseholdOccupancyID: "aaee1100-0003-4e55-8000-000000000001", StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "aacc0002-0002-4f66-8000-000000000002", ResidentID: "aaaa0002-0002-4f66-8000-000000000002", HouseholdOccupancyID: "aaee1100-0004-4e55-8000-000000000002", StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "aacc0002-0003-4f66-8000-000000000003", ResidentID: "aaaa0002-0003-4f66-8000-000000000003", HouseholdOccupancyID: "aaee1100-0005-4e55-8000-000000000003", StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "5cba43a2-2feb-44bf-9ad4-a66767578d3f", ResidentID: "eeee0001-0005-4f66-8000-000000000005", HouseholdOccupancyID: "dddd0001-0005-4e55-8000-000000000005", StartDate: strPtr("2019-05-20"), EndDate: nil},
	{ID: "889a68db-b4b7-451f-bffd-e0ca1f22695e", ResidentID: "1e0f80e4-6cbc-4b64-960e-4c4419c819d4", HouseholdOccupancyID: "d23353e9-5372-41c6-bf0a-a813f1e683d1", StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "b228bb72-6d4b-45d2-a057-f590f87f667d", ResidentID: "eeee0001-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0001-0003-4e55-8000-000000000003", StartDate: strPtr("2025-01-02"), EndDate: nil},
	{ID: "c666ed0a-e923-4871-92ac-1241bd7b10a5", ResidentID: "eeee0001-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0001-0001-4e55-8000-000000000001", StartDate: strPtr("2023-06-16"), EndDate: nil},
	{ID: "d06b7002-c583-4b89-9250-ba5c4c9a49d1", ResidentID: "eeee0001-0004-4f66-8000-000000000004", HouseholdOccupancyID: "dddd0001-0004-4e55-8000-000000000004", StartDate: strPtr("2023-09-15"), EndDate: nil},
	{ID: "e9f48fbd-f870-4236-9438-4e8b1dada615", ResidentID: "eeee0001-0006-4f66-8000-000000000006", HouseholdOccupancyID: "dddd0001-0002-4e55-8000-000000000002", StartDate: strPtr("2024-02-01"), EndDate: nil},
	{ID: "f5f4239e-fce6-4186-9faa-a8566631024c", ResidentID: "6c2a10a1-5afb-4eee-b457-69d18eddbea5", HouseholdOccupancyID: "f68449c9-5472-43fb-853e-794887ec43f9", StartDate: strPtr("2026-09-22"), EndDate: nil},
	// Historical residency periods (RT 03)
	{ID: "ffff0001-0001-feff-5b77-000000000001", ResidentID: "eeee0001-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0001-0001-4e55-8000-000000000001", StartDate: strPtr("2015-03-10"), EndDate: strPtr("2020-06-15")},
	{ID: "ffff0001-0003-feff-5b77-000000000003", ResidentID: "eeee0001-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0001-0003-4e55-8000-000000000003", StartDate: strPtr("2018-07-01"), EndDate: strPtr("2025-01-01")},
}

// CanonicalUsers contains the mock users for Pengurus, Warga, and residents.
// All users share the password "123qweasdzxc" (hashed via auth.Hash()).
// NEVER include Super Admin here.
var CanonicalUsers = []UserSeed{
	// Dedicated Warga account for RT 03
	{ID: "22220001-0000-4000-8000-000000000001", Email: "warga.rt03@local.com", Phone: "+6281300000030", FullName: "Warga RT 03", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},

	// Resident Warga accounts (RT 03)
	{ID: "22220001-0001-4000-8000-000000000001", Email: "hartana.wibowo@local.com", Phone: "+628130000101", FullName: "Hartana Wibowo", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220001-0003-4000-8000-000000000001", Email: "dedi.kurniawan@local.com", Phone: "+628130000103", FullName: "Dedi Kurniawan", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220001-0004-4000-8000-000000000001", Email: "nurhaliza.sari@local.com", Phone: "+628130000104", FullName: "Nurhaliza Sari", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220001-0006-4000-8000-000000000001", Email: "sri.mulyani@local.com", Phone: "+628130000106", FullName: "Sri Mulyani", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},

	// RT 002 (Urena) accounts
	{ID: "22220000-0001-4000-8000-000000000001", Email: "andriyan@local.com", Phone: "+6281200000001", FullName: "Andriyan", Role: "warga", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
	{ID: "22220000-0002-4000-8000-000000000001", Email: "mety.fitriani@local.com", Phone: "+6281200000002", FullName: "Mety Fitriani", Role: "warga", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
	{ID: "22220000-0003-4000-8000-000000000001", Email: "aqueen.alsava@local.com", Phone: "+6281200000003", FullName: "Aqueen Alsava Khaylila", Role: "warga", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
}

// positionRTs holds the RT IDs used for position seeding (RT 003 and RT 002).
var positionRTs = []string{
	"8dfa1e36-6727-45be-8fea-24d958efc155", // RT 03
	"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", // RT 002
}

var positionRTIndex = map[string]int{
	"8dfa1e36-6727-45be-8fea-24d958efc155": 0, // RT 03
	"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b": 4, // RT 002
}

// positionHouseNumbers maps each RT index to house numbers.
var positionHouseNumbers = map[int][]string{
	0: {"06", "07", "08", "09", "10"},                     // RT 03
	4: {"U12/16", "U12/18", "U12/20", "U12/22", "U12/24"}, // RT 002
}

// positionHouseAddrs maps each RT index to 5 addresses.
var positionHouseAddrs = map[int][]string{
	0: {"Jl. Gading Raya Blok D No. 06", "Jl. Gading Raya Blok D No. 07", "Jl. Gading Raya Blok D No. 08", "Jl. Gading Raya Blok D No. 09", "Jl. Gading Raya Blok D No. 10"},
	4: {"Jakarta Timur", "Jakarta Timur", "Jakarta Timur", "Jakarta Timur", "Jakarta Timur"},
}

// positionHeadNames maps each RT index to 5 head names (one per eligible position).
var positionHeadNames = map[int][]string{
	0: {"Ketua RT 03", "Wakil Ketua RT 03", "Sekretaris RT 03", "Bendahara RT 03", "Sosial RT 03"},
	4: {"Ketua RT 002", "Wakil Ketua RT 002", "Sekretaris RT 002", "Bendahara RT 002", "Sosial RT 002"},
}

// eligiblePositionIdx maps jabatan to its index among the eligible positions (ketua=0, wakil_ketua=1, sekretaris=2, bendahara=3, sosial=4).
var eligiblePositionIdx = map[string]int{
	"ketua":       0,
	"wakil_ketua": 1,
	"sekretaris":  2,
	"bendahara":   3,
	"sosial":      4,
}

// CanonicalPositionUsers contains position-based seed accounts:
// 2 seeded RTs x 7 positions = 14 accounts.
// All use system_role=nil and tenant role "pengurus" via the membership role.
// Each account has a deterministic ID derived from RT index + position index.
var CanonicalPositionUsers []PositionSeed

func init() {
	positions := []string{"ketua", "wakil_ketua", "sekretaris", "bendahara", "keamanan", "sosial", "kebersihan_pembangunan", "anggota"}
	posNames := map[string]string{
		"ketua":       "Ketua",
		"wakil_ketua": "Wakil Ketua",
		"sekretaris":  "Sekretaris",
		"bendahara":   "Bendahara",
		"keamanan":    "Keamanan",
		"sosial":      "Sosial",
		"anggota":     "Anggota",
	}
	// RT labels match the simplified email pattern used in database
	rtLabels := map[int]string{
		0: "003", // RT 003
		4: "002", // RT 002
	}
	rtBase := map[int]string{
		0: "31731603",
		4: "31750000",
	}

	// Positions eligible for resident records
	eligiblePositions := map[string]bool{
		"ketua":                  true,
		"wakil_ketua":            true,
		"sekretaris":             true,
		"bendahara":              true,
		"sosial":                 true,
		"keamanan":               false,
		"kebersihan_pembangunan": false,
		"anggota":                false,
	}

	seedIdx := 0
	for _, rtID := range positionRTs {
		rtIdx := positionRTIndex[rtID]
		for posIdx, pos := range positions {
			id := fmt.Sprintf("%08x-0000-4000-8000-000000000001", (4000+rtIdx)<<16|(8000+posIdx))

			// Generate deterministic 16-digit NIK based on RT + position + user identity
			nikSource := fmt.Sprintf("%s-%s-%s", rtID, pos, id)
			nikHash := sha256.Sum256([]byte(nikSource))
			// Use first 8 bytes, convert to hex, take first 16 chars (all hex = digits 0-9 and a-f)
			// We need only digits, so use modulo to ensure numeric
			hexPart := hex.EncodeToString(nikHash[:8])
			// Convert hex to uint64: 8 bytes = up to 16 hex chars, some exceed int64 max
			var nikNum uint64
			for _, c := range hexPart {
				nikNum *= 16
				switch {
				case c >= '0' && c <= '9':
					nikNum += uint64(c - '0')
				case c >= 'a' && c <= 'f':
					nikNum += uint64(c - 'a' + 10)
				}
			}
			base := rtBase[rtIdx]
			// Combine 8-char prefix with 8-digit suffix to form 16-digit NIK
			suffix := nikNum % 100000000
			nik := base + fmt.Sprintf("%08d", suffix)

			phone := fmt.Sprintf("+628%d%d%06d", 13+rtIdx, posIdx, 100+seedIdx)
			if !eligiblePositions[pos] {
				phone = ""
			}

			// Normalize jabatan to simplified email form
			var email string
			if pos == "kebersihan_pembangunan" {
				// Special format: operasional.rt{RT}@local.com
				email = fmt.Sprintf("operasional.rt%s@local.com", rtLabels[rtIdx])
			} else if pos == "wakil_ketua" {
				// Hyphen instead of underscore: wakil-ketua-{RT}@local.com
				email = fmt.Sprintf("wakil-ketua-%s@local.com", rtLabels[rtIdx])
			} else {
				// Standard format: {jabatan}-{RT}@local.com
				email = fmt.Sprintf("%s-%s@local.com", pos, rtLabels[rtIdx])
			}
			// Build FullName: Position {Name} RT {code}
			fullName := fmt.Sprintf("Position %s RT %s", posNames[pos], rtLabels[rtIdx])

			seed := PositionSeed{
				ID:       id,
				Email:    email,
				Phone:    phone,
				FullName: fullName,
				Jabatan:  pos,
				RTID:     rtID,
				NIK:      nik,
			}
			CanonicalPositionUsers = append(CanonicalPositionUsers, seed)
			seedIdx++
		}
	}

	// Build position resident mapping for seed.go.
	// Each eligible position gets its own physical house, household, and occupancy.
	eligiblePositionsList := []string{"ketua", "wakil_ketua", "sekretaris", "bendahara", "sosial"}

	for _, ps := range CanonicalPositionUsers {
		idx, eligible := eligiblePositionIdx[ps.Jabatan]
		if !eligible {
			continue
		}

		rtIdx, ok := positionRTIndex[ps.RTID]
		if !ok {
			continue
		}

		// Derive deterministic IDs for the new entities (physical house, household, occupancy)
		posGlobalIdx := rtIdx*7 + idx
		houseID := fmt.Sprintf("%08x-0000-4c33-8000-000000000001", (5000+rtIdx)<<16|(8000+posGlobalIdx))
		householdID := fmt.Sprintf("%08x-0000-4d44-8000-000000000001", (6000+rtIdx)<<16|(8000+posGlobalIdx))
		occupancyID := fmt.Sprintf("%08x-0000-4e55-8000-000000000001", (7000+rtIdx)<<16|(8000+posGlobalIdx))

		positionResidentData = append(positionResidentData, PositionResidentSeed{
			UserID:             ps.ID,
			FullName:           ps.FullName,
			Phone:              ps.Phone,
			NIK:                ps.NIK,
			Email:              strPtr(ps.Email),
			RTID:               ps.RTID,
			OccupancyID:        occupancyID,
			HouseID:            houseID,
			HouseholdID:        householdID,
			HouseNumber:        positionHouseNumbers[rtIdx][idx],
			HouseAddress:       positionHouseAddrs[rtIdx][idx],
			HeadName:           positionHeadNames[rtIdx][idx],
			EligiblePosIdx:     idx,
			EligiblePosName:    eligiblePositionsList[idx],
			PositionIdx:        posGlobalIdx,
			IsEligible:         true,
		})
	}

	// Build special position resident data for non-eligible positions
	// (keamanan, kebersihan_pembangunan). These get a resident record but NO
	// household chain.
	for _, ps := range CanonicalPositionUsers {
		if eligiblePositions[ps.Jabatan] {
			continue
		}

		if _, ok := positionRTIndex[ps.RTID]; !ok {
			continue
		}

		specialPositionResidentData = append(specialPositionResidentData, SpecialPositionResidentSeed{
			UserID:   ps.ID,
			FullName: ps.FullName,
			RTID:     ps.RTID,
			NIK:      ps.NIK,
			Email:    strPtr(ps.Email),
		})
	}
}

// financialPositions maps RT index to bendahara user ID for payment approval.
// Bendahara is position index 3 within each RT's position users.
var financialPositionRTs []string
var financialBendaharaUIDs map[string]string // rtID -> bendahara user ID

// Financial category IDs: 0=income, 1=expense
// Due IDs: 0=Kebersihan, 1=Keamanan, 2=Kas RT
// Occupancy slots: house numbers 01 and 03 (first and third per RT)

func init() {
	// Financial seed needs RT IDs and bendahara user IDs
	financialPositionRTs = make([]string, len(positionRTs))
	copy(financialPositionRTs, positionRTs)

	// Filter financial seed to only RT 002 and RT 003
	// Position seed still runs for all 5 RTs (org data), but financial data
	// (categories, dues, bills, payments) is restricted to 2 RTs.
	{
		filtered := financialPositionRTs[:0]
		for _, rtID := range financialPositionRTs {
			if rtID == "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b" || // RT 002
				rtID == "8dfa1e36-6727-45be-8fea-24d958efc155" { // RT 003
				filtered = append(filtered, rtID)
			}
		}
		financialPositionRTs = filtered
	}

	financialBendaharaUIDs = make(map[string]string)

	// Find bendahara user IDs from CanonicalPositionUsers
	for _, ps := range CanonicalPositionUsers {
		if ps.Jabatan == "bendahara" {
			financialBendaharaUIDs[ps.RTID] = ps.ID
		}
	}

	// --- CanonicalFinancialCategories ---
	// 2 categories per RT: income + expense
	for rtIdx, rtID := range financialPositionRTs {
		// Income category
		incomeID := fmt.Sprintf("%08x-0000-3000-8000-000000000001", (8000+rtIdx)<<16|(8000+0))
		CanonicalFinancialCategories = append(CanonicalFinancialCategories, FinancialCategorySeed{
			ID:       incomeID,
			RTID:     rtID,
			Name:     "Iuran Warga",
			Type:     "income",
			IsActive: true,
		})
		// Expense category
		expenseID := fmt.Sprintf("%08x-0000-3000-8000-000000000002", (8000+rtIdx)<<16|(8000+1))
		CanonicalFinancialCategories = append(CanonicalFinancialCategories, FinancialCategorySeed{
			ID:       expenseID,
			RTID:     rtID,
			Name:     "Operasional RT",
			Type:     "expense",
			IsActive: true,
		})
	}

	// --- CanonicalDues ---
	// 3 dues per RT: Kebersihan (50k), Keamanan (100k), Kas RT (25k)
	for rtIdx, rtID := range financialPositionRTs {
		dueName := []string{"Iuran Kebersihan", "Iuran Keamanan", "Iuran Kas RT"}
		dueAmount := []string{"50000.00", "100000.00", "25000.00"}
		duePeriodType := []string{"monthly", "monthly", "monthly"}

		for i := 0; i < 3; i++ {
			dueID := fmt.Sprintf("%08x-0000-3100-8000-00000000000%d", (8000+rtIdx)<<16|(8000+i), i+1)
			CanonicalDues = append(CanonicalDues, DueSeed{
				ID:         dueID,
				RTID:       rtID,
				Name:       dueName[i],
				Amount:     dueAmount[i],
				PeriodType: duePeriodType[i],
				IsActive:   true,
			})
		}
	}

	// CanonicalBills and CanonicalPayments are intentionally empty.
	// On fresh install / seed, transactional data must be 0:
	// bills = 0, payments = 0, payment_allocations = 0, transactions = 0.
}
