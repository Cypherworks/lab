package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMagicPacket(t *testing.T) {
	pkt, err := magicPacket("10:ff:e0:84:29:d8")
	if err != nil {
		t.Fatalf("magicPacket: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("packet is %d bytes, want 102", len(pkt))
	}
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Fatalf("byte %d = %#x, want 0xFF (sync stream)", i, pkt[i])
		}
	}
	mac := []byte{0x10, 0xff, 0xe0, 0x84, 0x29, 0xd8}
	for rep := 0; rep < 16; rep++ {
		got := pkt[6+rep*6 : 6+rep*6+6]
		for j := range mac {
			if got[j] != mac[j] {
				t.Fatalf("repetition %d byte %d = %#x, want %#x", rep, j, got[j], mac[j])
			}
		}
	}
}

func TestMagicPacketBadMAC(t *testing.T) {
	if _, err := magicPacket("not-a-mac"); err == nil {
		t.Fatal("expected an error for a malformed MAC")
	}
}

func withDevices(t *testing.T, ds []Device) {
	t.Helper()
	devices = ds
	deviceByName = map[string]Device{}
	for _, d := range ds {
		deviceByName[d.Name] = d
	}
}

// handleIndex renders every configured device even when the status ping can't run
// (no raw socket in the test sandbox), so the page never depends on reachability.
func TestIndexListsDevices(t *testing.T) {
	withDevices(t, []Device{{Name: "node-ryzen", MAC: "10:ff:e0:84:29:d8", IP: "10.200.20.41"}})
	rr := httptest.NewRecorder()
	handleIndex(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "node-ryzen") {
		t.Fatal("page does not list node-ryzen")
	}
}

func TestWakeUnknownDeviceIs404(t *testing.T) {
	withDevices(t, []Device{{Name: "node-ryzen", MAC: "10:ff:e0:84:29:d8", IP: "10.200.20.41"}})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/wake/nope", nil)
	req.SetPathValue("name", "nope")
	handleWake(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST /wake/nope = %d, want 404", rr.Code)
	}
}
