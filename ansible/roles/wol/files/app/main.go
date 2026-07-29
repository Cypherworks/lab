// wol — a minimal Wake-on-LAN web page. It lists the configured devices, shows a
// live online/offline ping, and sends a magic packet on a button press. No auth
// (Caddy + Authentik forward-auth gate it) and no database — devices come from a
// config file the deploy renders from ip-plan, so it redeploys identically with no
// bootstrap.
package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"
)

// Device is one wake target, as rendered into the config file from ip-plan.
type Device struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
}

type deviceView struct {
	Device
	Online bool
}

var (
	devices      []Device
	deviceByName = map[string]Device{}
)

func loadDevices(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &devices); err != nil {
		return err
	}
	for _, d := range devices {
		deviceByName[d.Name] = d
	}
	return nil
}

// magicPacket builds the 102-byte Wake-on-LAN payload: six 0xFF bytes then the MAC
// repeated sixteen times.
func magicPacket(mac string) ([]byte, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	pkt := make([]byte, 0, 6+16*len(hw))
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xFF)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, hw...)
	}
	return pkt, nil
}

// wake broadcasts the magic packet on the local segment (the container sits on the
// target's VLAN), covering both common WoL ports.
func wake(mac string) error {
	pkt, err := magicPacket(mac)
	if err != nil {
		return err
	}
	dialer := net.Dialer{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if cerr := c.Control(func(fd uintptr) {
			serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
		}); cerr != nil {
			return cerr
		}
		return serr
	}}
	for _, port := range []string{"9", "7"} {
		conn, err := dialer.Dial("udp", net.JoinHostPort("255.255.255.255", port))
		if err != nil {
			return err
		}
		_, werr := conn.Write(pkt)
		conn.Close()
		if werr != nil {
			return werr
		}
	}
	return nil
}

// icmpChecksum is the 16-bit ones-complement checksum ICMP requires over its message.
func icmpChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// online reports whether the host answers an ICMP echo within a second, over a raw
// ICMP socket (NET_RAW). Stdlib only — no third-party network code. On Linux the read
// carries the IP header ahead of the ICMP message, so it's skipped by header length.
func online(ip string) bool {
	dst, err := net.ResolveIPAddr("ip4", ip)
	if err != nil {
		return false
	}
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	req := []byte{8, 0, 0, 0, byte(id >> 8), byte(id), 0, 1} // type 8 echo, code 0, id, seq 1
	cs := icmpChecksum(req)
	req[2], req[3] = byte(cs>>8), byte(cs)
	if _, err := conn.WriteTo(req, dst); err != nil {
		return false
	}

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return false
	}
	buf := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			return false
		}
		if peer.String() != dst.String() {
			continue
		}
		pkt := buf[:n]
		if len(pkt) < 20 {
			continue
		}
		ihl := int(pkt[0]&0x0f) * 4
		if len(pkt) >= ihl+1 && pkt[ihl] == 0 { // ICMP type 0 = echo reply
			return true
		}
	}
}

// statuses pings every device concurrently.
func statuses() []deviceView {
	views := make([]deviceView, len(devices))
	var wg sync.WaitGroup
	for i, d := range devices {
		wg.Add(1)
		go func(i int, d Device) {
			defer wg.Done()
			views[i] = deviceView{Device: d, Online: online(d.IP)}
		}(i, d)
	}
	wg.Wait()
	return views
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Lab Wake-on-LAN</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 40rem; margin: 3rem auto; padding: 0 1rem; }
  h1 { font-size: 1.4rem; }
  ul { list-style: none; padding: 0; }
  li { display: flex; align-items: center; gap: 1rem; padding: .75rem 0; border-bottom: 1px solid #ddd; }
  .name { flex: 1; font-weight: 600; }
  .dot { width: .7rem; height: .7rem; border-radius: 50%; }
  .up { background: #2a9d3f; } .down { background: #bbb; }
  button { padding: .5rem 1rem; font-size: 1rem; cursor: pointer; }
  button:disabled { opacity: .4; cursor: default; }
</style>
</head>
<body>
<h1>Lab Wake-on-LAN</h1>
<ul>
{{range .}}
  <li>
    <span class="dot {{if .Online}}up{{else}}down{{end}}" title="{{if .Online}}online{{else}}offline{{end}}"></span>
    <span class="name">{{.Name}}</span>
    <form method="post" action="/wake/{{.Name}}">
      <button type="submit" {{if .Online}}disabled{{end}}>Wake</button>
    </form>
  </li>
{{end}}
</ul>
</body>
</html>`))

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := page.Execute(w, statuses()); err != nil {
		log.Printf("render: %v", err)
	}
}

// handleHealthz answers the container health check without pinging any device.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func handleWake(w http.ResponseWriter, r *http.Request) {
	d, ok := deviceByName[r.PathValue("name")]
	if !ok {
		http.Error(w, "unknown device", http.StatusNotFound)
		return
	}
	if err := wake(d.MAC); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func main() {
	addr := os.Getenv("WOL_LISTEN")
	if addr == "" {
		addr = ":8090"
	}
	path := os.Getenv("WOL_DEVICES_FILE")
	if path == "" {
		path = "/config/devices.json"
	}
	if err := loadDevices(path); err != nil {
		log.Fatalf("load devices from %s: %v", path, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("POST /wake/{name}", handleWake)
	log.Printf("wol listening on %s with %d device(s)", addr, len(devices))
	log.Fatal(http.ListenAndServe(addr, mux))
}
