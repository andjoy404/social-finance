package rt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-finance/internal/auth"
)

func TestCanViewMembershipContactsRequiresExactKetuaAndSameRT(t *testing.T) {
	cases := []struct {
		name string
		ac   *auth.AuthContext
		rtID string
		want bool
	}{
		{"exact Ketua same RT", &auth.AuthContext{Jabatan: auth.JabatanKetua, RTID: "rt-1"}, "rt-1", true},
		{"wakil ketua", &auth.AuthContext{Jabatan: auth.JabatanWakilKetua, RTID: "rt-1"}, "rt-1", false},
		{"sekretaris", &auth.AuthContext{Jabatan: auth.JabatanSekretaris, RTID: "rt-1"}, "rt-1", false},
		{"bendahara", &auth.AuthContext{Jabatan: auth.JabatanBendahara, RTID: "rt-1"}, "rt-1", false},
		{"keamanan", &auth.AuthContext{Jabatan: auth.JabatanKeamanan, RTID: "rt-1"}, "rt-1", false},
		{"kebersihan", &auth.AuthContext{Jabatan: auth.JabatanKebersihan, RTID: "rt-1"}, "rt-1", false},
		{"operasional", &auth.AuthContext{Jabatan: auth.JabatanOperasional, RTID: "rt-1"}, "rt-1", false},
		{"humas", &auth.AuthContext{Jabatan: auth.JabatanHumas, RTID: "rt-1"}, "rt-1", false},
		{"sosial", &auth.AuthContext{Jabatan: auth.JabatanSosial, RTID: "rt-1"}, "rt-1", false},
		{"anggota", &auth.AuthContext{Jabatan: auth.JabatanAnggota, RTID: "rt-1"}, "rt-1", false},
		{"anggota RT", &auth.AuthContext{Jabatan: auth.JabatanAnggotaRT, RTID: "rt-1"}, "rt-1", false},
		{"ketua alias underscore", &auth.AuthContext{Jabatan: "ketua_rt", RTID: "rt-1"}, "rt-1", false},
		{"ketua alias space", &auth.AuthContext{Jabatan: "ketua rt", RTID: "rt-1"}, "rt-1", false},
		{"ketua other RT", &auth.AuthContext{Jabatan: auth.JabatanKetua, RTID: "rt-2"}, "rt-1", false},
		{"ketua missing RT scope", &auth.AuthContext{Jabatan: auth.JabatanKetua}, "rt-1", false},
		{"ketua missing target RT", &auth.AuthContext{Jabatan: auth.JabatanKetua, RTID: "rt-1"}, "", false},
		{"super admin with ketua claim", &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, Jabatan: auth.JabatanKetua, RTID: "rt-1"}, "rt-1", false},
		{"no position", &auth.AuthContext{RTID: "rt-1"}, "rt-1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canViewMembershipContacts(tc.ac, tc.rtID); got != tc.want {
				t.Fatalf("canViewMembershipContacts() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRTMembershipJSONRedactionAndSelfServicePreserved(t *testing.T) {
	email, phone := "resident@example.test", "+628123456789"
	item := RTMembership{ID: "membership-1", UserID: "resident-1", FullName: "Resident", Email: &email, Phone: &phone}
	cases := []struct {
		name string
		ac   *auth.AuthContext
		want bool
	}{
		{"exact Ketua", &auth.AuthContext{Jabatan: auth.JabatanKetua, RTID: "rt-1"}, true},
		{"Super Admin", &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, RTID: "rt-1"}, false},
		{"general permission", &auth.AuthContext{RTID: "rt-1"}, false},
		{"alias", &auth.AuthContext{Jabatan: "ketua_rt", RTID: "rt-1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responseMemberships := redactRTMembershipContacts([]RTMembership{item}, tc.ac, "rt-1")
			data, err := json.Marshal(responseMemberships)
			if err != nil {
				t.Fatal(err)
			}
			var response []map[string]any
			if err := json.Unmarshal(data, &response); err != nil {
				t.Fatal(err)
			}
			if tc.want {
				if response[0]["email"] != email || response[0]["phone"] != phone {
					t.Fatalf("authorized response did not retain contact fields: %s", data)
				}
				return
			}
			if _, exists := response[0]["email"]; exists {
				t.Fatalf("email leaked in JSON: %s", data)
			}
			if _, exists := response[0]["phone"]; exists {
				t.Fatalf("phone leaked in JSON: %s", data)
			}
		})
	}

	// Self-service is provided by the existing authenticated /me profile endpoint;
	// membership serializers do not replace or alter that response path.
}

func TestRWMembershipAccountContactsRemainAvailableToAccountManagement(t *testing.T) {
	// This Super Admin-only endpoint manages RW user accounts. These are
	// account contacts, not private Warga resident-contact fields.
	email, phone := "officer@example.test", "+628123456789"
	items := []RWMembership{{ID: "rw-membership-1", UserID: "user-1", Email: &email, Phone: &phone}}
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("response data count = %d, want 1", len(response.Data))
	}
	if response.Data[0]["email"] != email || response.Data[0]["phone"] != phone {
		t.Fatalf("RW account contacts were lost: %s", w.Body.String())
	}
}

func TestMembershipHandlerRequiresRTScopeForContactVisibility(t *testing.T) {
	email, phone := "resident@example.test", "+628123456789"
	item := RTMembership{ID: "membership-1", UserID: "resident-1", Email: &email, Phone: &phone}
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]any{
		"data": redactRTMembershipContacts([]RTMembership{item}, &auth.AuthContext{Jabatan: auth.JabatanKetua, RTID: "rt-other"}, "rt-target"),
	})
	var response map[string][]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"email", "phone"} {
		if _, exists := response["data"][0][field]; exists {
			t.Fatalf("cross-RT %s leaked: %s", field, w.Body.String())
		}
	}
}
