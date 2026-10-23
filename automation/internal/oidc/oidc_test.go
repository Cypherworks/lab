package oidc

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testIssuer   = "https://token.actions.githubusercontent.com"
	testAudience = "cw-broker"
)

var now = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

type signer struct {
	kid string
	key *rsa.PrivateKey
}

func newSigner(t *testing.T, kid string) signer {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return signer{kid: kid, key: k}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func jwk(s signer) map[string]string {
	return map[string]string{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": s.kid,
		"n": b64(s.key.N.Bytes()),
		"e": b64(big.NewInt(int64(s.key.E)).Bytes()),
	}
}

// jwksServer serves the given signers' public keys and counts fetches.
func jwksServer(t *testing.T, keys *atomic.Value, fetches *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		var set []map[string]string
		for _, s := range keys.Load().([]signer) {
			set = append(set, jwk(s))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": set})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func token(t *testing.T, s signer, header map[string]any, claims map[string]any) string {
	t.Helper()
	if header == nil {
		header = map[string]any{"alg": "RS256", "typ": "JWT", "kid": s.kid}
	}
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims)
	input := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + b64(sig)
}

func goodClaims() map[string]any {
	return map[string]any{
		"iss":                 testIssuer,
		"aud":                 testAudience,
		"sub":                 "repo:Cypherworks/lab-deploy:ref:refs/heads/main",
		"exp":                 now.Add(5 * time.Minute).Unix(),
		"nbf":                 now.Add(-1 * time.Minute).Unix(),
		"iat":                 now.Add(-1 * time.Minute).Unix(),
		"repository":          "Cypherworks/lab-deploy",
		"repository_id":       "1277716830",
		"repository_owner_id": "206194062",
		"ref":                 "refs/heads/main",
		"sha":                 "0123456789abcdef0123456789abcdef01234567",
		"event_name":          "workflow_dispatch",
		"actor":               "lloydoliver",
		"actor_id":            "2228673",
		"workflow_ref":        "Cypherworks/lab-deploy/.github/workflows/apply.yml@refs/heads/main",
		"job_workflow_ref":    "Cypherworks/lab-deploy/.github/workflows/_tf.yml@refs/heads/main",
		"run_id":              "36262468605",
		"run_attempt":         "1",
		"check_run_id":        "108460782842",
	}
}

type fixture struct {
	signer   signer
	keys     *atomic.Value
	fetches  *atomic.Int32
	verifier *Verifier
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s := newSigner(t, "kid-1")
	keys := &atomic.Value{}
	keys.Store([]signer{s})
	fetches := &atomic.Int32{}
	srv := jwksServer(t, keys, fetches)
	v := &Verifier{
		Issuer:   testIssuer,
		Audience: testAudience,
		JWKSURL:  srv.URL,
		Client:   srv.Client(),
		Now:      func() time.Time { return now },
	}
	return &fixture{signer: s, keys: keys, fetches: fetches, verifier: v}
}

func TestVerifyAcceptsValidToken(t *testing.T) {
	f := newFixture(t)
	c, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, goodClaims()))
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	want := Claims{
		Subject:           "repo:Cypherworks/lab-deploy:ref:refs/heads/main",
		Repository:        "Cypherworks/lab-deploy",
		RepositoryID:      "1277716830",
		RepositoryOwnerID: "206194062",
		Ref:               "refs/heads/main",
		SHA:               "0123456789abcdef0123456789abcdef01234567",
		EventName:         "workflow_dispatch",
		Actor:             "lloydoliver",
		ActorID:           "2228673",
		WorkflowRef:       "Cypherworks/lab-deploy/.github/workflows/apply.yml@refs/heads/main",
		JobWorkflowRef:    "Cypherworks/lab-deploy/.github/workflows/_tf.yml@refs/heads/main",
		RunID:             "36262468605",
		RunAttempt:        "1",
		CheckRunID:        "108460782842",
	}
	if *c != want {
		t.Fatalf("claims mismatch:\n got %+v\nwant %+v", *c, want)
	}
}

func TestVerifyAcceptsAudienceArrayContainingOurs(t *testing.T) {
	f := newFixture(t)
	claims := goodClaims()
	claims["aud"] = []string{"something-else", testAudience}
	if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, claims)); err != nil {
		t.Fatalf("aud array containing ours rejected: %v", err)
	}
}

func TestVerifyRejects(t *testing.T) {
	cases := map[string]func(c map[string]any){
		"wrong issuer":         func(c map[string]any) { c["iss"] = "https://evil.example" },
		"wrong audience":       func(c map[string]any) { c["aud"] = "sts.amazonaws.com" },
		"audience array":       func(c map[string]any) { c["aud"] = []string{"sts.amazonaws.com"} },
		"missing audience":     func(c map[string]any) { delete(c, "aud") },
		"numeric audience":     func(c map[string]any) { c["aud"] = 42 },
		"numeric repo id":      func(c map[string]any) { c["repository_id"] = 1277716830 },
		"expired":              func(c map[string]any) { c["exp"] = now.Add(-2 * time.Minute).Unix() },
		"missing exp":          func(c map[string]any) { delete(c, "exp") },
		"not yet valid":        func(c map[string]any) { c["nbf"] = now.Add(2 * time.Minute).Unix() },
		"issued in the future": func(c map[string]any) { c["iat"] = now.Add(2 * time.Minute).Unix() },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			claims := goodClaims()
			mutate(claims)
			if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, claims)); err == nil {
				t.Fatal("token accepted")
			}
		})
	}
}

func TestVerifyToleratesSmallClockSkew(t *testing.T) {
	f := newFixture(t)
	claims := goodClaims()
	claims["exp"] = now.Add(-10 * time.Second).Unix()
	claims["nbf"] = now.Add(10 * time.Second).Unix()
	claims["iat"] = now.Add(10 * time.Second).Unix()
	if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, claims)); err != nil {
		t.Fatalf("token within leeway rejected: %v", err)
	}
}

func TestVerifyRejectsForgedSignature(t *testing.T) {
	f := newFixture(t)
	forger := newSigner(t, f.signer.kid) // same kid, different key
	if _, err := f.verifier.Verify(context.Background(), token(t, forger, nil, goodClaims())); err == nil {
		t.Fatal("token signed by a key not in the JWKS accepted")
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	f := newFixture(t)
	parts := strings.Split(token(t, f.signer, nil, goodClaims()), ".")
	claims := goodClaims()
	claims["actor"] = "someone-else"
	c, _ := json.Marshal(claims)
	parts[1] = b64(c)
	if _, err := f.verifier.Verify(context.Background(), strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered payload accepted")
	}
}

func TestVerifyRejectsOtherAlgorithms(t *testing.T) {
	f := newFixture(t)
	c, _ := json.Marshal(goodClaims())

	// alg=none with an empty signature.
	h, _ := json.Marshal(map[string]any{"alg": "none", "kid": f.signer.kid})
	if _, err := f.verifier.Verify(context.Background(), b64(h)+"."+b64(c)+"."); err == nil {
		t.Fatal("alg=none accepted")
	}

	// HS256 keyed with the public modulus: the classic key-confusion forgery.
	h, _ = json.Marshal(map[string]any{"alg": "HS256", "kid": f.signer.kid})
	input := b64(h) + "." + b64(c)
	mac := hmac.New(sha256.New, f.signer.key.N.Bytes())
	mac.Write([]byte(input))
	if _, err := f.verifier.Verify(context.Background(), input+"."+b64(mac.Sum(nil))); err == nil {
		t.Fatal("HS256 accepted")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	f := newFixture(t)
	for _, tok := range []string{"", "a.b", "a.b.c.d", "!!.!!.!!", token(t, f.signer, nil, goodClaims()) + "x"} {
		if _, err := f.verifier.Verify(context.Background(), tok); err == nil {
			t.Fatalf("malformed token %q accepted", tok)
		}
	}
}

func TestVerifyRefetchesJWKSForRotatedKey(t *testing.T) {
	f := newFixture(t)
	if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, goodClaims())); err != nil {
		t.Fatal(err)
	}
	if got := f.fetches.Load(); got != 1 {
		t.Fatalf("fetches after first verify = %d, want 1", got)
	}
	// Cached: a second verify with the same kid doesn't refetch.
	if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, goodClaims())); err != nil {
		t.Fatal(err)
	}
	if got := f.fetches.Load(); got != 1 {
		t.Fatalf("fetches after cached verify = %d, want 1", got)
	}
	// GitHub rotates: a new kid appears in the JWKS.
	rotated := newSigner(t, "kid-2")
	f.keys.Store([]signer{f.signer, rotated})
	if _, err := f.verifier.Verify(context.Background(), token(t, rotated, nil, goodClaims())); err != nil {
		t.Fatalf("token from rotated key rejected: %v", err)
	}
	if got := f.fetches.Load(); got != 2 {
		t.Fatalf("fetches after rotation = %d, want 2", got)
	}
}

func TestVerifyRejectsUnknownKid(t *testing.T) {
	f := newFixture(t)
	stranger := newSigner(t, "kid-unknown")
	if _, err := f.verifier.Verify(context.Background(), token(t, stranger, nil, goodClaims())); err == nil {
		t.Fatal("token with a kid not in the JWKS accepted")
	}
}

// serveJWKS points the verifier at a server returning body with status.
func serveJWKS(t *testing.T, f *fixture, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	f.verifier.JWKSURL = srv.URL
	f.verifier.Client = srv.Client()
}

func TestVerifyFailsClosedOnBadJWKS(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"endpoint down":   {http.StatusInternalServerError, "down"},
		"not JSON":        {http.StatusOK, "<html>"},
		"bad modulus":     {http.StatusOK, `{"keys":[{"kty":"RSA","kid":"kid-1","n":"!!","e":"AQAB"}]}`},
		"empty exponent":  {http.StatusOK, `{"keys":[{"kty":"RSA","kid":"kid-1","n":"AQAB","e":""}]}`},
		"only an EC key":  {http.StatusOK, `{"keys":[{"kty":"EC","kid":"kid-1"}]}`},
		"unreachable url": {0, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			if tc.status == 0 {
				f.verifier.JWKSURL = "http://127.0.0.1:1/jwks"
			} else {
				serveJWKS(t, f, tc.status, tc.body)
			}
			if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, goodClaims())); err == nil {
				t.Fatal("token accepted without a usable JWKS")
			}
		})
	}
}

func TestVerifySkipsNonRSAKeysInJWKS(t *testing.T) {
	f := newFixture(t)
	k := jwk(f.signer)
	body, _ := json.Marshal(map[string]any{"keys": []any{
		map[string]string{"kty": "EC", "kid": "ec-1", "crv": "P-256"},
		k,
	}})
	serveJWKS(t, f, http.StatusOK, string(body))
	if _, err := f.verifier.Verify(context.Background(), token(t, f.signer, nil, goodClaims())); err != nil {
		t.Fatalf("valid token rejected when the JWKS also holds an EC key: %v", err)
	}
}

func TestVerifyRejectsUndecodableParts(t *testing.T) {
	f := newFixture(t)
	parts := strings.Split(token(t, f.signer, nil, goodClaims()), ".")
	cases := map[string]string{
		"header not JSON":   b64([]byte("x")) + "." + parts[1] + "." + parts[2],
		"signature not b64": parts[0] + "." + parts[1] + ".!!",
	}
	for name, tok := range cases {
		if _, err := f.verifier.Verify(context.Background(), tok); err == nil {
			t.Fatalf("%s: token accepted", name)
		}
	}
}

func TestVerifyRejectsSignedGarbagePayload(t *testing.T) {
	f := newFixture(t)
	h, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": f.signer.kid})
	for _, payload := range []string{"!!", b64([]byte("not json"))} {
		input := b64(h) + "." + payload
		sum := sha256.Sum256([]byte(input))
		sig, err := rsa.SignPKCS1v15(rand.Reader, f.signer.key, crypto.SHA256, sum[:])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.verifier.Verify(context.Background(), input+"."+b64(sig)); err == nil {
			t.Fatalf("signed payload %q accepted", payload)
		}
	}
}
