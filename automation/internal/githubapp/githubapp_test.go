package githubapp

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// verifyAppJWT checks a JWT the way GitHub documents it and returns claims.
func verifyAppJWT(t *testing.T, pub *rsa.PublicKey, tok string) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts", len(parts))
	}
	var header map[string]string
	h, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if err := json.Unmarshal(h, &header); err != nil {
		t.Fatal(err)
	}
	if header["alg"] != "RS256" || header["typ"] != "JWT" {
		t.Fatalf("header = %v, want alg RS256 typ JWT", header)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		t.Fatalf("JWT signature does not verify: %v", err)
	}
	var claims map[string]any
	p, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(p, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestAppJWTMeetsGitHubRequirements(t *testing.T) {
	key := newKey(t)
	tok, err := AppJWT(key, "Iv23liExample", now)
	if err != nil {
		t.Fatal(err)
	}
	c := verifyAppJWT(t, &key.PublicKey, tok)
	if c["iss"] != "Iv23liExample" {
		t.Fatalf("iss = %v", c["iss"])
	}
	iat := time.Unix(int64(c["iat"].(float64)), 0)
	exp := time.Unix(int64(c["exp"].(float64)), 0)
	if !iat.Equal(now.Add(-60 * time.Second)) {
		t.Fatalf("iat = %v, want 60s before now", iat)
	}
	if !exp.After(now) || exp.Sub(now) > 10*time.Minute {
		t.Fatalf("exp = %v, want after now and at most 10 minutes ahead", exp)
	}
}

func TestParsePrivateKeyAcceptsPKCS1AndPKCS8(t *testing.T) {
	key := newKey(t)
	pkcs1 := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for name, p := range map[string][]byte{"pkcs1": pkcs1, "pkcs8": pkcs8} {
		got, err := ParsePrivateKey(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !got.Equal(key) {
			t.Fatalf("%s: parsed a different key", name)
		}
	}
}

func TestParsePrivateKeyRejectsNonRSAAndGarbage(t *testing.T) {
	cases := map[string][]byte{
		"not PEM":    []byte("hello"),
		"bad DER":    pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte{1, 2}}),
		"bad PKCS8":  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2}}),
		"other type": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1}}),
	}
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(edKey)
	if err != nil {
		t.Fatal(err)
	}
	cases["ed25519 PKCS8"] = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for name, p := range cases {
		if _, err := ParsePrivateKey(p); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

// fakeGitHub enforces GitHub's App auth contract and records the token body.
type fakeGitHub struct {
	t              *testing.T
	pub            *rsa.PublicKey
	grantedPerms   map[string]string
	installStatus  int
	tokenStatus    int
	gotTokenBody   map[string]any
	gotInstallPath string
}

func (g *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "no bearer", http.StatusUnauthorized)
		return
	}
	verifyAppJWT(g.t, g.pub, strings.TrimPrefix(auth, "Bearer "))
	if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
		http.Error(w, "api version", http.StatusBadRequest)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/orgs/Cypherworks/installation":
		g.gotInstallPath = r.URL.Path
		if g.installStatus != 0 {
			http.Error(w, "nope", g.installStatus)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 4242})
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/4242/access_tokens":
		if g.tokenStatus != 0 {
			http.Error(w, "nope", g.tokenStatus)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&g.gotTokenBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":       "ghs_test",
			"expires_at":  "2026-09-26T13:00:00Z",
			"permissions": g.grantedPerms,
		})
	default:
		http.NotFound(w, r)
	}
}

func newClient(t *testing.T, g *fakeGitHub) (*Client, *httptest.Server) {
	t.Helper()
	key := newKey(t)
	g.t, g.pub = t, &key.PublicKey
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	return &Client{
		BaseURL: srv.URL,
		HTTP:    srv.Client(),
		Key:     key,
		Issuer:  "Iv23liExample",
		Now:     func() time.Time { return now },
	}, srv
}

func request() TokenRequest {
	return TokenRequest{
		RepositoryIDs: []int64{1277716830},
		Permissions:   map[string]string{"contents": "read", "pull_requests": "read"},
	}
}

func TestInstallationTokenIsDownScoped(t *testing.T) {
	g := &fakeGitHub{grantedPerms: map[string]string{"contents": "read", "pull_requests": "read"}}
	c, _ := newClient(t, g)
	tok, err := c.InstallationToken(context.Background(), "Cypherworks", request())
	if err != nil {
		t.Fatal(err)
	}
	if tok.Value != "ghs_test" || !tok.ExpiresAt.Equal(time.Date(2026, 9, 26, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("token = %+v", tok)
	}
	want := map[string]any{
		"repository_ids": []any{float64(1277716830)},
		"permissions":    map[string]any{"contents": "read", "pull_requests": "read"},
	}
	if !reflect.DeepEqual(g.gotTokenBody, want) {
		t.Fatalf("token request body = %v, want %v", g.gotTokenBody, want)
	}
}

func TestInstallationTokenRefusesUnscopedRequests(t *testing.T) {
	g := &fakeGitHub{}
	c, _ := newClient(t, g)
	noRepos := request()
	noRepos.RepositoryIDs = nil
	noPerms := request()
	noPerms.Permissions = nil
	for name, req := range map[string]TokenRequest{"no repositories": noRepos, "no permissions": noPerms} {
		if _, err := c.InstallationToken(context.Background(), "Cypherworks", req); err == nil {
			t.Fatalf("%s: token minted", name)
		}
	}
	if g.gotInstallPath != "" {
		t.Fatal("GitHub was called for an unscoped request")
	}
}

func TestInstallationTokenAcceptsForcedMetadataRead(t *testing.T) {
	g := &fakeGitHub{grantedPerms: map[string]string{
		"contents": "read", "pull_requests": "read", "metadata": "read",
	}}
	c, _ := newClient(t, g)
	if _, err := c.InstallationToken(context.Background(), "Cypherworks", request()); err != nil {
		t.Fatalf("rejected GitHub's baseline metadata:read: %v", err)
	}
}

func TestInstallationTokenRejectsGrantOtherThanRequested(t *testing.T) {
	cases := map[string]map[string]string{
		"write for read":  {"contents": "write", "pull_requests": "read"},
		"extra":           {"contents": "read", "pull_requests": "read", "actions": "write"},
		"metadata write":  {"contents": "read", "pull_requests": "read", "metadata": "write"},
		"less than asked": {"contents": "read"},
	}
	for name, granted := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := newClient(t, &fakeGitHub{grantedPerms: granted})
			if _, err := c.InstallationToken(context.Background(), "Cypherworks", request()); err == nil {
				t.Fatalf("accepted grant %v for request %v", granted, request().Permissions)
			}
		})
	}
}

func TestInstallationTokenFailsOnGitHubErrors(t *testing.T) {
	for name, g := range map[string]*fakeGitHub{
		"no installation": {installStatus: http.StatusNotFound},
		"token refused":   {tokenStatus: http.StatusUnprocessableEntity},
	} {
		c, _ := newClient(t, g)
		if _, err := c.InstallationToken(context.Background(), "Cypherworks", request()); err == nil {
			t.Fatalf("%s: token returned", name)
		}
	}
}

func TestInstallationTokenFailsOnMalformedResponse(t *testing.T) {
	key := newKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>"))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Key: key, Issuer: "x", Now: func() time.Time { return now }}
	if _, err := c.InstallationToken(context.Background(), "Cypherworks", request()); err == nil {
		t.Fatal("token returned from a non-JSON response")
	}
}

func TestInstallationTokenFailsOnUnreachableGitHub(t *testing.T) {
	c, srv := newClient(t, &fakeGitHub{})
	srv.Close()
	if _, err := c.InstallationToken(context.Background(), "Cypherworks", request()); err == nil {
		t.Fatal("token returned with GitHub unreachable")
	}
}
