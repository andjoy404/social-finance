package auth

import "testing"

func TestJabatanCatalogIncludesAnggota(t *testing.T) {
	if !ValidJabatans[JabatanAnggota] {
		t.Fatal("anggota must be a valid jabatan")
	}
	found := false
	for _, jabatan := range AllJabatans {
		if jabatan == JabatanAnggota {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("anggota must be present in AllJabatans")
	}
}

func TestJabatanCatalogIncludesHumas(t *testing.T) {
	if !ValidJabatans[JabatanHumas] {
		t.Fatal("humas must be a valid jabatan")
	}
	found := false
	for _, jabatan := range AllJabatans {
		if jabatan == JabatanHumas {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("humas must be present in AllJabatans")
	}
}
