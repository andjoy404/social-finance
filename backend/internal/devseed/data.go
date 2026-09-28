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
	RelationshipToHead   *string
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
	RelationshipToHead *string
	// Derived household chain for eligible positions
	HouseID       string
	HouseholdID   string
	HouseNumber   string
	HouseAddress  string
	HeadName      string
	EligiblePosIdx int
	EligiblePosName string
	PositionIdx   int
	IsEligible    bool
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

// positionResidentData contains the derived resident records for position users
// who are eligible (ketua, wakil_ketua, sekretaris, bendahara, sosial).
var positionResidentData []PositionResidentSeed

// specialPositionResidentData contains the derived resident records for
// non-eligible position users (keamanan, kebersihan_pembangunan).
// These are special residents: linked to their user account, assigned to their RT,
// but WITHOUT household/occupancy/residency chain.
var specialPositionResidentData []SpecialPositionResidentSeed

// CanonicalRTs contains the 5 RTs.
var CanonicalRTs = []RTSeed{
	{ID: "8dfa1e36-6727-45be-8fea-24d958efc155", Name: "Wisma Rukun Tunggal", RW: 16, RT: "03", Address: "Jl. Gading Raya Blok D", HeadName: "Hartono Wijaya", IsActive: true},
	{ID: "efb70405-09f5-48a0-ad14-34802cf09e3b", Name: "Bakti Harmoni", RW: 16, RT: "04", Address: "Jl. Gading Raya Blok F", HeadName: "Suryadi Pratama", IsActive: true},
	{ID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", Name: "Makmur Sentosa", RW: 16, RT: "05", Address: "Jl. Gading Raya Blok H", HeadName: "Diana Kusuma", IsActive: true},
	{ID: "a3390720-dd56-4b28-92b1-43f384f584cd", Name: "Sukaramai", RW: 16, RT: "06", Address: "Jl. Gading Raya Blok J", HeadName: "Rahmat Hidayat", IsActive: true},
	{ID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", Name: "Urena", RW: 16, RT: "002", Address: "GNI Urena", HeadName: "Atom", IsActive: true},
}

// CanonicalPhysicalHouses contains the 20 physical houses.
var CanonicalPhysicalHouses = []PhysicalHouseSeed{
	// RT 05
	{ID: "bbbb0003-0001-4c33-8000-000000000001", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HouseNumber: "01", Address: "Jl. Gading Raya Blok H No. 01", IsActive: true},
	{ID: "bbbb0003-0002-4c33-8000-000000000002", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HouseNumber: "02", Address: "Jl. Gading Raya Blok H No. 02", IsActive: true},
	{ID: "bbbb0003-0003-4c33-8000-000000000003", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HouseNumber: "03", Address: "Jl. Gading Raya Blok H No. 03", IsActive: true},
	{ID: "bbbb0003-0004-4c33-8000-000000000004", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HouseNumber: "04", Address: "Jl. Gading Raya Blok H No. 04", IsActive: true},
	// RT 03
	{ID: "bbbb0001-0001-4c33-8000-000000000001", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "01", Address: "Jl. Gading Raya Blok D No. 01", IsActive: true},
	{ID: "bbbb0001-0002-4c33-8000-000000000002", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "02", Address: "Jl. Gading Raya Blok D No. 02", IsActive: true},
	{ID: "bbbb0001-0003-4c33-8000-000000000003", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "03", Address: "Jl. Gading Raya Blok D No. 03", IsActive: true},
	{ID: "bbbb0001-0004-4c33-8000-000000000004", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "04", Address: "Jl. Gading Raya Blok D No. 04", IsActive: true},
	{ID: "bbbb0001-0005-4c33-8000-000000000005", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HouseNumber: "05", Address: "Jl. Gading Raya Blok D No. 05", IsActive: true},
	// RT 06
	{ID: "bbbb0004-0001-4c33-8000-000000000001", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HouseNumber: "01", Address: "Jl. Gading Raya Blok J No. 01", IsActive: true},
	{ID: "bbbb0004-0002-4c33-8000-000000000002", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HouseNumber: "02", Address: "Jl. Gading Raya Blok J No. 02", IsActive: true},
	{ID: "bbbb0004-0003-4c33-8000-000000000003", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HouseNumber: "03", Address: "Jl. Gading Raya Blok J No. 03", IsActive: true},
	{ID: "bbbb0004-0004-4c33-8000-000000000004", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HouseNumber: "04", Address: "Jl. Gading Raya Blok J No. 04", IsActive: true},
	// RT 002 (Urena)
	{ID: "ab799cca-f833-4504-8a4e-ef715d407449", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/12", Address: "Jakarta Timur", IsActive: true},
	{ID: "dd9c1056-29eb-4211-8468-df25323cb4be", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HouseNumber: "U12/14", Address: "GNI Blok Urena", IsActive: true},
	// RT 04
	{ID: "bbbb0002-0001-4c33-8000-000000000001", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HouseNumber: "01", Address: "Jl. Gading Raya Blok F No. 01", IsActive: true},
	{ID: "bbbb0002-0002-4c33-8000-000000000002", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HouseNumber: "02", Address: "Jl. Gading Raya Blok F No. 02", IsActive: true},
	{ID: "bbbb0002-0003-4c33-8000-000000000003", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HouseNumber: "03", Address: "Jl. Gading Raya Blok F No. 03", IsActive: true},
	{ID: "bbbb0002-0004-4c33-8000-000000000004", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HouseNumber: "04", Address: "Jl. Gading Raya Blok F No. 04", IsActive: true},
	{ID: "bbbb0002-0005-4c33-8000-000000000005", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HouseNumber: "05", Address: "Jl. Gading Raya Blok F No. 05", IsActive: true},
}

// CanonicalHouseholds contains the 20 households.
var CanonicalHouseholds = []HouseholdSeed{
	{ID: "5360467d-4b4b-4325-9609-dde8c780b12e", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Andriyan", IsActive: true},
	{ID: "9d1f520c-54d3-46b3-b4bc-69a8edf0842e", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", HeadName: "Mety Fitriani", IsActive: true},
	{ID: "cccc0001-0001-4d44-8000-000000000001", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Hartana Wibowo", IsActive: true},
	{ID: "cccc0001-0002-4d44-8000-000000000002", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Sri Mulyani", IsActive: true},
	{ID: "cccc0001-0003-4d44-8000-000000000003", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Dedi Kurniawan", IsActive: true},
	{ID: "cccc0001-0004-4d44-8000-000000000004", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Nurhaliza Sari", IsActive: true},
	{ID: "cccc0001-0005-4d44-8000-000000000005", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", HeadName: "Bambang Sutrisno", IsActive: false},
	{ID: "cccc0002-0001-4d44-8000-000000000001", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HeadName: "Susanti Dewi", IsActive: true},
	{ID: "cccc0002-0002-4d44-8000-000000000002", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HeadName: "Ahmad Fauzi", IsActive: true},
	{ID: "cccc0002-0003-4d44-8000-000000000003", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HeadName: "Lestari Wulandari", IsActive: true},
	{ID: "cccc0002-0004-4d44-8000-000000000004", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HeadName: "Hendra Gunawan", IsActive: true},
	{ID: "cccc0002-0005-4d44-8000-000000000005", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", HeadName: "Dewi Sartika", IsActive: false},
	{ID: "cccc0003-0001-4d44-8000-000000000001", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HeadName: "Agus Prasetyo", IsActive: true},
	{ID: "cccc0003-0002-4d44-8000-000000000002", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HeadName: "Ratna Sari", IsActive: true},
	{ID: "cccc0003-0003-4d44-8000-000000000003", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HeadName: "Muhammad Iqbal", IsActive: true},
	{ID: "cccc0003-0004-4d44-8000-000000000004", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", HeadName: "Kartini Rahayu", IsActive: false},
	{ID: "cccc0004-0001-4d44-8000-000000000001", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HeadName: "Endi Permana", IsActive: true},
	{ID: "cccc0004-0002-4d44-8000-000000000002", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HeadName: "Rina Anggraini", IsActive: true},
	{ID: "cccc0004-0003-4d44-8000-000000000003", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HeadName: "Fauzan Rizki", IsActive: true},
	{ID: "cccc0004-0004-4d44-8000-000000000004", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", HeadName: "Triyani Sudarwati", IsActive: true},
}

// CanonicalOccupancies contains the 20 household occupancies.
var CanonicalOccupancies = []HouseholdOccupancySeed{
	{ID: "d23353e9-5372-41c6-bf0a-a813f1e683d1", PhysicalHouseID: "dd9c1056-29eb-4211-8468-df25323cb4be", HouseholdID: "5360467d-4b4b-4325-9609-dde8c780b12e", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-20"), EndDate: nil},
	{ID: "dddd0001-0001-4e55-8000-000000000001", PhysicalHouseID: "bbbb0001-0001-4c33-8000-000000000001", HouseholdID: "cccc0001-0001-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2023-06-16"), EndDate: nil},
	{ID: "dddd0001-0002-4e55-8000-000000000002", PhysicalHouseID: "bbbb0001-0002-4c33-8000-000000000002", HouseholdID: "cccc0001-0002-4d44-8000-000000000002", OccupancyStatus: "OWNER", StartDate: strPtr("2024-02-01"), EndDate: nil},
	{ID: "dddd0001-0003-4e55-8000-000000000003", PhysicalHouseID: "bbbb0001-0003-4c33-8000-000000000003", HouseholdID: "cccc0001-0003-4d44-8000-000000000003", OccupancyStatus: "TENANT", StartDate: strPtr("2025-01-02"), EndDate: nil},
	{ID: "dddd0001-0004-4e55-8000-000000000004", PhysicalHouseID: "bbbb0001-0004-4c33-8000-000000000004", HouseholdID: "cccc0001-0004-4d44-8000-000000000004", OccupancyStatus: "OWNER", StartDate: strPtr("2023-09-15"), EndDate: nil},
	{ID: "dddd0001-0005-4e55-8000-000000000005", PhysicalHouseID: "bbbb0001-0005-4c33-8000-000000000005", HouseholdID: "cccc0001-0005-4d44-8000-000000000005", OccupancyStatus: "OWNER", StartDate: strPtr("2019-05-20"), EndDate: nil},
	{ID: "dddd0002-0001-4e55-8000-000000000001", PhysicalHouseID: "bbbb0002-0001-4c33-8000-000000000001", HouseholdID: "cccc0002-0001-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2024-07-10"), EndDate: nil},
	{ID: "dddd0002-0002-4e55-8000-000000000002", PhysicalHouseID: "bbbb0002-0002-4c33-8000-000000000002", HouseholdID: "cccc0002-0002-4d44-8000-000000000002", OccupancyStatus: "OWNER", StartDate: strPtr("2024-03-11"), EndDate: nil},
	{ID: "dddd0002-0003-4e55-8000-000000000003", PhysicalHouseID: "bbbb0002-0003-4c33-8000-000000000003", HouseholdID: "cccc0002-0003-4d44-8000-000000000003", OccupancyStatus: "TENANT", StartDate: strPtr("2024-11-01"), EndDate: nil},
	{ID: "dddd0002-0004-4e55-8000-000000000004", PhysicalHouseID: "bbbb0002-0004-4c33-8000-000000000004", HouseholdID: "cccc0002-0004-4d44-8000-000000000004", OccupancyStatus: "OWNER", StartDate: strPtr("2022-12-21"), EndDate: nil},
	{ID: "dddd0002-0005-4e55-8000-000000000005", PhysicalHouseID: "bbbb0002-0005-4c33-8000-000000000005", HouseholdID: "cccc0002-0005-4d44-8000-000000000005", OccupancyStatus: "TENANT", StartDate: strPtr("2023-03-14"), EndDate: nil},
	{ID: "dddd0003-0001-4e55-8000-000000000001", PhysicalHouseID: "bbbb0003-0001-4c33-8000-000000000001", HouseholdID: "cccc0003-0001-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2024-08-20"), EndDate: nil},
	{ID: "dddd0003-0002-4e55-8000-000000000002", PhysicalHouseID: "bbbb0003-0002-4c33-8000-000000000002", HouseholdID: "cccc0003-0002-4d44-8000-000000000002", OccupancyStatus: "OWNER", StartDate: strPtr("2025-01-05"), EndDate: nil},
	{ID: "dddd0003-0003-4e55-8000-000000000003", PhysicalHouseID: "bbbb0003-0003-4c33-8000-000000000003", HouseholdID: "cccc0003-0003-4d44-8000-000000000003", OccupancyStatus: "TENANT", StartDate: strPtr("2025-06-02"), EndDate: nil},
	{ID: "dddd0003-0004-4e55-8000-000000000004", PhysicalHouseID: "bbbb0003-0004-4c33-8000-000000000004", HouseholdID: "cccc0003-0004-4d44-8000-000000000004", OccupancyStatus: "TENANT", StartDate: strPtr("2023-04-01"), EndDate: nil},
	{ID: "dddd0004-0001-4e55-8000-000000000001", PhysicalHouseID: "bbbb0004-0001-4c33-8000-000000000001", HouseholdID: "cccc0004-0001-4d44-8000-000000000001", OccupancyStatus: "OWNER", StartDate: strPtr("2024-06-15"), EndDate: nil},
	{ID: "dddd0004-0002-4e55-8000-000000000002", PhysicalHouseID: "bbbb0004-0002-4c33-8000-000000000002", HouseholdID: "cccc0004-0002-4d44-8000-000000000002", OccupancyStatus: "OWNER", StartDate: strPtr("2023-03-21"), EndDate: nil},
	{ID: "dddd0004-0003-4e55-8000-000000000003", PhysicalHouseID: "bbbb0004-0003-4c33-8000-000000000003", HouseholdID: "cccc0004-0003-4d44-8000-000000000003", OccupancyStatus: "TENANT", StartDate: strPtr("2024-05-10"), EndDate: nil},
	{ID: "dddd0004-0004-4e55-8000-000000000004", PhysicalHouseID: "bbbb0004-0004-4c33-8000-000000000004", HouseholdID: "cccc0004-0004-4d44-8000-000000000004", OccupancyStatus: "OWNER", StartDate: strPtr("2021-11-16"), EndDate: nil},
	{ID: "f68449c9-5472-43fb-853e-794887ec43f9", PhysicalHouseID: "ab799cca-f833-4504-8a4e-ef715d407449", HouseholdID: "9d1f520c-54d3-46b3-b4bc-69a8edf0842e", OccupancyStatus: "OWNER", StartDate: strPtr("2026-09-22"), EndDate: nil},
}

// CanonicalResidents contains the 20 residents (HEAD only, no family/relative).
var CanonicalResidents = []ResidentSeed{
	{ID: "1e0f80e4-6cbc-4b64-960e-4c4419c819d4", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Andriyan", Phone: "+6281200000001", IsActive: true, NIK: strPtr("3175000000000001"), Email: strPtr("andriyan@example.test")},
	{ID: "6c2a10a1-5afb-4eee-b457-69d18eddbea5", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", FullName: "Mety Fitriani", Phone: "+6281200000002", IsActive: true, NIK: strPtr("3175000000000002"), Email: strPtr("mety.fitriani@example.test")},
	{ID: "eeee0001-0001-4f66-8000-000000000001", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Hartana Wibowo", Phone: "+628130000101", IsActive: true, NIK: strPtr("3173160301730001"), Email: strPtr("hartana.wibowo@example.test")},
	{ID: "eeee0001-0003-4f66-8000-000000000003", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Dedi Kurniawan", Phone: "+628130000103", IsActive: true, NIK: strPtr("3173160301730003"), Email: strPtr("dedi.kurniawan@example.test")},
	{ID: "eeee0001-0004-4f66-8000-000000000004", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Nurhaliza Sari", Phone: "+628130000104", IsActive: true, NIK: strPtr("3173160301730004"), Email: strPtr("nurhaliza.sari@example.test")},
	{ID: "eeee0001-0005-4f66-8000-000000000005", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Bambang Sutrisno", Phone: "+628130000105", IsActive: true, NIK: strPtr("3173160301730005"), Email: strPtr("")},
	{ID: "eeee0001-0006-4f66-8000-000000000006", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155", FullName: "Sri Mulyani", Phone: "+628130000106", IsActive: true, NIK: strPtr("3173160301730006"), Email: strPtr("sri.mulyani@example.test")},
	{ID: "eeee0002-0001-4f66-8000-000000000001", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", FullName: "Susanti Dewi", Phone: "+628140000101", IsActive: true, NIK: strPtr("3173160401730001"), Email: strPtr("susanti.dewi@example.test")},
	{ID: "eeee0002-0002-4f66-8000-000000000002", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", FullName: "Ahmad Fauzi", Phone: "+628140000102", IsActive: true, NIK: strPtr("3173160401730002"), Email: strPtr("ahmad.fauzi@example.test")},
	{ID: "eeee0002-0003-4f66-8000-000000000003", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", FullName: "Lestari Wulandari", Phone: "+628140000103", IsActive: true, NIK: strPtr("3173160401730003"), Email: strPtr("lestari.wulandari@example.test")},
	{ID: "eeee0002-0004-4f66-8000-000000000004", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", FullName: "Hendra Gunawan", Phone: "+628140000104", IsActive: true, NIK: strPtr("3173160401730004"), Email: strPtr("hendra.gunawan@example.test")},
	{ID: "eeee0002-0005-4f66-8000-000000000005", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b", FullName: "Dewi Sartika", Phone: "+628140000105", IsActive: true, NIK: strPtr("3173160401730005"), Email: strPtr("")},
	{ID: "eeee0003-0001-4f66-8000-000000000001", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", FullName: "Agus Prasetyo", Phone: "+628150000101", IsActive: true, NIK: strPtr("3173160501730001"), Email: strPtr("agus.prasetyo@example.test")},
	{ID: "eeee0003-0002-4f66-8000-000000000002", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", FullName: "Ratna Sari", Phone: "+628150000102", IsActive: true, NIK: strPtr("3173160501730002"), Email: strPtr("ratna.sari@example.test")},
	{ID: "eeee0003-0003-4f66-8000-000000000003", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", FullName: "Muhammad Iqbal", Phone: "+628150000103", IsActive: true, NIK: strPtr("3173160501730003"), Email: strPtr("muhammad.iqbal@example.test")},
	{ID: "eeee0003-0004-4f66-8000-000000000004", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d", FullName: "Kartini Rahayu", Phone: "+628150000104", IsActive: true, NIK: strPtr("3173160501730004"), Email: strPtr("")},
	{ID: "eeee0004-0001-4f66-8000-000000000001", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", FullName: "Endi Permana", Phone: "+628160000101", IsActive: true, NIK: strPtr("3173160601730001"), Email: strPtr("endi.permana@example.test")},
	{ID: "eeee0004-0002-4f66-8000-000000000002", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", FullName: "Rina Anggraini", Phone: "+628160000102", IsActive: true, NIK: strPtr("3173160601730002"), Email: strPtr("rina.anggraini@example.test")},
	{ID: "eeee0004-0003-4f66-8000-000000000003", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", FullName: "Fauzan Rizki", Phone: "+628160000103", IsActive: true, NIK: strPtr("3173160601730003"), Email: strPtr("fauzan.rizki@example.test")},
	{ID: "eeee0004-0004-4f66-8000-000000000004", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd", FullName: "Triyani Sudarwati", Phone: "+628160000104", IsActive: true, NIK: strPtr("3173160601730004"), Email: strPtr("tri.sudarwati@example.test")},
}

// CanonicalResidencyPeriods contains the 20 current + 6 historical residency periods.
var CanonicalResidencyPeriods = []ResidencyPeriodSeed{
	{ID: "1c51623d-837d-42f1-853d-a9e7bfa544c3", ResidentID: "eeee0003-0002-4f66-8000-000000000002", HouseholdOccupancyID: "dddd0003-0002-4e55-8000-000000000002", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2025-01-05"), EndDate: nil},
	{ID: "23c362ed-8cba-4bb1-a326-1bcc768b7944", ResidentID: "eeee0003-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0003-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2025-06-02"), EndDate: nil},
	{ID: "29bf92be-33e9-42f7-8eab-c76eba0b9cd0", ResidentID: "eeee0002-0002-4f66-8000-000000000002", HouseholdOccupancyID: "dddd0002-0002-4e55-8000-000000000002", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-03-11"), EndDate: nil},
	{ID: "35b253a4-5185-4b6f-9f5d-da1f28cf2e30", ResidentID: "eeee0003-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0003-0001-4e55-8000-000000000001", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-08-20"), EndDate: nil},
	{ID: "35fe309a-7f6b-41ee-a484-ed000e7a86d2", ResidentID: "eeee0003-0004-4f66-8000-000000000004", HouseholdOccupancyID: "dddd0003-0004-4e55-8000-000000000004", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2023-04-01"), EndDate: nil},
	{ID: "5931e434-f558-4611-a636-8ec63769115f", ResidentID: "eeee0004-0002-4f66-8000-000000000002", HouseholdOccupancyID: "dddd0004-0002-4e55-8000-000000000002", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2023-03-21"), EndDate: nil},
	{ID: "5b006622-0b0a-4e9f-b844-cf86396e9932", ResidentID: "eeee0002-0005-4f66-8000-000000000005", HouseholdOccupancyID: "dddd0002-0005-4e55-8000-000000000005", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2023-03-14"), EndDate: nil},
	{ID: "5cba43a2-2feb-44bf-9ad4-a66767578d3f", ResidentID: "eeee0001-0005-4f66-8000-000000000005", HouseholdOccupancyID: "dddd0001-0005-4e55-8000-000000000005", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2019-05-20"), EndDate: nil},
	{ID: "5f50c294-1c24-4cf2-b674-40d3fdc03670", ResidentID: "eeee0004-0004-4f66-8000-000000000004", HouseholdOccupancyID: "dddd0004-0004-4e55-8000-000000000004", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2021-11-16"), EndDate: nil},
	{ID: "794bf894-aa65-41d1-abb5-2234d7be8c1f", ResidentID: "eeee0004-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0004-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-05-10"), EndDate: nil},
	{ID: "889a68db-b4b7-451f-bffd-e0ca1f22695e", ResidentID: "1e0f80e4-6cbc-4b64-960e-4c4419c819d4", HouseholdOccupancyID: "d23353e9-5372-41c6-bf0a-a813f1e683d1", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2026-09-25"), EndDate: nil},
	{ID: "b228bb72-6d4b-45d2-a057-f590f87f667d", ResidentID: "eeee0001-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0001-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2025-01-02"), EndDate: nil},
	{ID: "c666ed0a-e923-4871-92ac-1241bd7b10a5", ResidentID: "eeee0001-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0001-0001-4e55-8000-000000000001", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2023-06-16"), EndDate: nil},
	{ID: "cffa99bc-2937-46ca-b8f3-68b24cfc5f28", ResidentID: "eeee0002-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0002-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-11-01"), EndDate: nil},
	{ID: "d06b7002-c583-4b89-9250-ba5c4c9a49d1", ResidentID: "eeee0001-0004-4f66-8000-000000000004", HouseholdOccupancyID: "dddd0001-0004-4e55-8000-000000000004", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2023-09-15"), EndDate: nil},
	{ID: "d81e50b4-c1f7-4328-b4a2-92bd71356797", ResidentID: "eeee0002-0004-4f66-8000-000000000004", HouseholdOccupancyID: "dddd0002-0004-4e55-8000-000000000004", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2022-12-21"), EndDate: nil},
	{ID: "da6ef5e3-321b-4cdf-9153-7ed6cbcae5d8", ResidentID: "eeee0002-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0002-0001-4e55-8000-000000000001", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-07-10"), EndDate: nil},
	{ID: "e9f48fbd-f870-4236-9438-4e8b1dada615", ResidentID: "eeee0001-0006-4f66-8000-000000000006", HouseholdOccupancyID: "dddd0001-0002-4e55-8000-000000000002", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-02-01"), EndDate: nil},
	{ID: "ee8fa6d7-fde1-4158-b7a1-13a15b5db9a7", ResidentID: "eeee0004-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0004-0001-4e55-8000-000000000001", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2024-06-15"), EndDate: nil},
	{ID: "f5f4239e-fce6-4186-9faa-a8566631024c", ResidentID: "6c2a10a1-5afb-4eee-b457-69d18eddbea5", HouseholdOccupancyID: "f68449c9-5472-43fb-853e-794887ec43f9", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2026-09-22"), EndDate: nil},
	// Historical residency periods
	{ID: "ffff0001-0001-feff-5b77-000000000001", ResidentID: "eeee0001-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0001-0001-4e55-8000-000000000001", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2015-03-10"), EndDate: strPtr("2020-06-15")},
	{ID: "ffff0001-0003-feff-5b77-000000000003", ResidentID: "eeee0001-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0001-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2018-07-01"), EndDate: strPtr("2025-01-01")},
	{ID: "ffff0002-0001-feff-5b77-000000000001", ResidentID: "eeee0002-0001-4f66-8000-000000000001", HouseholdOccupancyID: "dddd0002-0001-4e55-8000-000000000001", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2019-01-15"), EndDate: strPtr("2024-07-09")},
	{ID: "ffff0002-0003-feff-5b77-000000000003", ResidentID: "eeee0002-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0002-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2020-05-20"), EndDate: strPtr("2024-10-31")},
	{ID: "ffff0003-0003-feff-5b77-000000000003", ResidentID: "eeee0003-0003-4f66-8000-000000000003", HouseholdOccupancyID: "dddd0003-0003-4e55-8000-000000000003", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2021-08-10"), EndDate: strPtr("2025-06-01")},
	{ID: "ffff0004-0002-feff-5b77-000000000002", ResidentID: "eeee0004-0002-4f66-8000-000000000002", HouseholdOccupancyID: "dddd0004-0002-4e55-8000-000000000002", RelationshipToHead: strPtr("HEAD"), StartDate: strPtr("2017-06-01"), EndDate: strPtr("2023-03-20")},
}

// CanonicalUsers contains the mock users for Pengurus, Warga, and residents.
// All users share the password "TestUser123!".
// NEVER include Super Admin here.
var CanonicalUsers = []UserSeed{
	// Pengurus for RT 03 - 06
	{ID: "11110001-0000-4000-8000-000000000001", Email: "pengurus.rt03@example.com", Phone: "+6281300000003", FullName: "Pengurus RT 03", Role: "pengurus", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "11110002-0000-4000-8000-000000000001", Email: "pengurus.rt04@example.com", Phone: "+6281400000004", FullName: "Pengurus RT 04", Role: "pengurus", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b"},
	{ID: "11110003-0000-4000-8000-000000000001", Email: "pengurus.rt05@example.com", Phone: "+6281500000005", FullName: "Pengurus RT 05", Role: "pengurus", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d"},
	{ID: "11110004-0000-4000-8000-000000000001", Email: "pengurus.rt06@example.com", Phone: "+6281600000006", FullName: "Pengurus RT 06", Role: "pengurus", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd"},

	// Dedicated Warga accounts for RT 03 - 06
	{ID: "22220001-0000-4000-8000-000000000001", Email: "warga.rt03@example.com", Phone: "+6281300000030", FullName: "Warga RT 03", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220002-0000-4000-8000-000000000001", Email: "warga.rt04@example.com", Phone: "+6281400000040", FullName: "Warga RT 04", Role: "warga", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b"},
	{ID: "22220003-0000-4000-8000-000000000001", Email: "warga.rt05@example.com", Phone: "+6281500000050", FullName: "Warga RT 05", Role: "warga", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d"},
	{ID: "22220004-0000-4000-8000-000000000001", Email: "warga.rt06@example.com", Phone: "+6281600000060", FullName: "Warga RT 06", Role: "warga", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd"},

	// Resident Warga accounts (RT 03)
	{ID: "22220001-0001-4000-8000-000000000001", Email: "hartana.wibowo@example.test", Phone: "+628130000101", FullName: "Hartana Wibowo", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220001-0003-4000-8000-000000000001", Email: "dedi.kurniawan@example.test", Phone: "+628130000103", FullName: "Dedi Kurniawan", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220001-0004-4000-8000-000000000001", Email: "nurhaliza.sari@example.test", Phone: "+628130000104", FullName: "Nurhaliza Sari", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},
	{ID: "22220001-0006-4000-8000-000000000001", Email: "sri.mulyani@example.test", Phone: "+628130000106", FullName: "Sri Mulyani", Role: "warga", RTID: "8dfa1e36-6727-45be-8fea-24d958efc155"},

	// Resident Warga accounts (RT 04)
	{ID: "22220002-0001-4000-8000-000000000001", Email: "susanti.dewi@example.test", Phone: "+628140000101", FullName: "Susanti Dewi", Role: "warga", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b"},
	{ID: "22220002-0002-4000-8000-000000000001", Email: "ahmad.fauzi@example.test", Phone: "+628140000102", FullName: "Ahmad Fauzi", Role: "warga", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b"},
	{ID: "22220002-0003-4000-8000-000000000001", Email: "lestari.wulandari@example.test", Phone: "+628140000103", FullName: "Lestari Wulandari", Role: "warga", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b"},
	{ID: "22220002-0004-4000-8000-000000000001", Email: "hendra.gunawan@example.test", Phone: "+628140000104", FullName: "Hendra Gunawan", Role: "warga", RTID: "efb70405-09f5-48a0-ad14-34802cf09e3b"},

	// Resident Warga accounts (RT 05)
	{ID: "22220003-0001-4000-8000-000000000001", Email: "agus.prasetyo@example.test", Phone: "+628150000101", FullName: "Agus Prasetyo", Role: "warga", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d"},
	{ID: "22220003-0002-4000-8000-000000000001", Email: "ratna.sari@example.test", Phone: "+628150000102", FullName: "Ratna Sari", Role: "warga", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d"},
	{ID: "22220003-0003-4000-8000-000000000001", Email: "muhammad.iqbal@example.test", Phone: "+628150000103", FullName: "Muhammad Iqbal", Role: "warga", RTID: "5106197a-c4e4-4bbc-8ef7-921d7208717d"},

	// Resident Warga accounts (RT 06)
	{ID: "22220004-0001-4000-8000-000000000001", Email: "endi.permana@example.test", Phone: "+628160000101", FullName: "Endi Permana", Role: "warga", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd"},
	{ID: "22220004-0002-4000-8000-000000000001", Email: "rina.anggraini@example.test", Phone: "+628160000102", FullName: "Rina Anggraini", Role: "warga", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd"},
	{ID: "22220004-0003-4000-8000-000000000001", Email: "fauzan.rizki@example.test", Phone: "+628160000103", FullName: "Fauzan Rizki", Role: "warga", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd"},
	{ID: "22220004-0004-4000-8000-000000000001", Email: "tri.sudarwati@example.test", Phone: "+628160000104", FullName: "Triyani Sudarwati", Role: "warga", RTID: "a3390720-dd56-4b28-92b1-43f384f584cd"},

	// RT 002 (Urena) accounts
	{ID: "22220000-0001-4000-8000-000000000001", Email: "andriyan@example.test", Phone: "+6281200000001", FullName: "Andriyan", Role: "warga", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
	{ID: "22220000-0002-4000-8000-000000000001", Email: "mety.fitriani@example.test", Phone: "+6281200000002", FullName: "Mety Fitriani", Role: "warga", RTID: "a95c1cb5-2f67-4ea5-9197-bfec537b6d7b"},
}

// positionRTs holds the RT IDs used for position seeding.
// Must match the RTs in CanonicalRTs in the same order.
var positionRTs = []string{
	"8dfa1e36-6727-45be-8fea-24d958efc155", // RT 03
	"efb70405-09f5-48a0-ad14-34802cf09e3b", // RT 04
	"5106197a-c4e4-4bbc-8ef7-921d7208717d", // RT 05
	"a3390720-dd56-4b28-92b1-43f384f584cd", // RT 06
	"a95c1cb5-2f67-4ea5-9197-bfec537b6d7b", // RT 002
}

// positionHouseNumbers maps each RT index to 7 house numbers (one per eligible position: ketua, wakil_ketua, sekretaris, bendahara, sosial).
// Numbers are chosen to avoid collision with CanonicalPhysicalHouses:
//   RT 03 has "01"-"05" → use "06"-"10"
//   RT 04 has "01"-"05" → use "06"-"10"
//   RT 05 has "01"-"04" → use "05"-"09"
//   RT 06 has "01"-"04" → use "05"-"09"
//   RT 002 has "U12/12", "U12/14" → use "U12/16"-"U12/24"
var positionHouseNumbers = map[int][]string{
	0: {"06", "07", "08", "09", "10"}, // RT 03
	1: {"06", "07", "08", "09", "10"}, // RT 04
	2: {"05", "06", "07", "08", "09"}, // RT 05
	3: {"05", "06", "07", "08", "09"}, // RT 06
	4: {"U12/16", "U12/18", "U12/20", "U12/22", "U12/24"}, // RT 002
}

// positionHouseAddrs maps each RT index to 5 addresses.
var positionHouseAddrs = map[int][]string{
	0: {"Jl. Gading Raya Blok D No. 06", "Jl. Gading Raya Blok D No. 07", "Jl. Gading Raya Blok D No. 08", "Jl. Gading Raya Blok D No. 09", "Jl. Gading Raya Blok D No. 10"},
	1: {"Jl. Gading Raya Blok F No. 06", "Jl. Gading Raya Blok F No. 07", "Jl. Gading Raya Blok F No. 08", "Jl. Gading Raya Blok F No. 09", "Jl. Gading Raya Blok F No. 10"},
	2: {"Jl. Gading Raya Blok H No. 05", "Jl. Gading Raya Blok H No. 06", "Jl. Gading Raya Blok H No. 07", "Jl. Gading Raya Blok H No. 08", "Jl. Gading Raya Blok H No. 09"},
	3: {"Jl. Gading Raya Blok J No. 05", "Jl. Gading Raya Blok J No. 06", "Jl. Gading Raya Blok J No. 07", "Jl. Gading Raya Blok J No. 08", "Jl. Gading Raya Blok J No. 09"},
	4: {"Jakarta Timur", "Jakarta Timur", "Jakarta Timur", "Jakarta Timur", "Jakarta Timur"},
}

// positionHeadNames maps each RT index to 5 head names (one per eligible position).
var positionHeadNames = map[int][]string{
	0: {"Ketua RT 03", "Wakil Ketua RT 03", "Sekretaris RT 03", "Bendahara RT 03", "Sosial RT 03"},
	1: {"Ketua RT 04", "Wakil Ketua RT 04", "Sekretaris RT 04", "Bendahara RT 04", "Sosial RT 04"},
	2: {"Ketua RT 05", "Wakil Ketua RT 05", "Sekretaris RT 05", "Bendahara RT 05", "Sosial RT 05"},
	3: {"Ketua RT 06", "Wakil Ketua RT 06", "Sekretaris RT 06", "Bendahara RT 06", "Sosial RT 06"},
	4: {"Ketua RT 002", "Wakil Ketua RT 002", "Sekretaris RT 002", "Bendahara RT 002", "Sosial RT 002"},
}

// eligiblePositionIdx maps jabatan to its index among the eligible positions (ketua=0, wakil_ketua=1, sekretaris=2, bendahara=3, sosial=4).
var eligiblePositionIdx = map[string]int{
	"ketua":     0,
	"wakil_ketua": 1,
	"sekretaris": 2,
	"bendahara": 3,
	"sosial":    4,
}

// CanonicalPositionUsers contains 40 position-based seed accounts:
// 5 seeded RTs x 8 positions.
// All use system_role=nil and tenant role "pengurus" via the membership role.
// Each account has a deterministic ID derived from RT index + position index.
var CanonicalPositionUsers []PositionSeed

func init() {
	positions := []string{"ketua", "wakil_ketua", "sekretaris", "bendahara", "keamanan", "sosial", "kebersihan_pembangunan"}
	posNames := map[string]string{
		"ketua":                  "Ketua",
		"wakil_ketua":            "Wakil Ketua",
		"sekretaris":             "Sekretaris",
		"bendahara":              "Bendahara",
		"keamanan":               "Keamanan",
		"sosial":                 "Sosial",
		"kebersihan_pembangunan": "Kebersihan Pembangunan",
	}
	rtLabels := []string{"03", "04", "05", "06", "002-urena"}

	// Positions eligible for resident records
	eligiblePositions := map[string]bool{
		"ketua":                  true,
		"wakil_ketua":            true,
		"sekretaris":             true,
		"bendahara":              true,
		"sosial":                 true,
		"keamanan":               false,
		"kebersihan_pembangunan": false,
	}

	seedIdx := 0
	for rtIdx, rtID := range positionRTs {
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
			// Base NIK prefix for this RT: RT 03=31731603, RT 04=31731604, RT 05=31731605, RT 06=31731606, RT 002=31750000
			rtBase := []string{"31731603", "31731604", "31731605", "31731606", "31750000"}
			base := rtBase[rtIdx]
			// Combine 8-char prefix with 8-digit suffix to form 16-digit NIK
			suffix := nikNum % 100000000
			nik := base + fmt.Sprintf("%08d", suffix)

			phone := fmt.Sprintf("+628%d%d%06d", 13+rtIdx, posIdx, 100+seedIdx)
			if !eligiblePositions[pos] {
				phone = ""
			}

			seed := PositionSeed{
				ID:       id,
				Email:    fmt.Sprintf("position.%s.rt%s@example.com", pos, rtLabels[rtIdx]),
				Phone:    phone,
				FullName: fmt.Sprintf("Position %s RT %s", posNames[pos], rtLabels[rtIdx]),
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

		// Find the RT index
		rtIdx := -1
		for i, rid := range positionRTs {
			if rid == ps.RTID {
				rtIdx = i
				break
			}
		}
		if rtIdx < 0 {
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
			RelationshipToHead: strPtr("HEAD"),
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

		// Find the RT index
		rtIdx := -1
		for i, rid := range positionRTs {
			if rid == ps.RTID {
				rtIdx = i
				break
			}
		}
		if rtIdx < 0 {
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
