// Package oidc verifies GitHub Actions OIDC ID tokens: RS256 JWTs signed by a
// key in the issuer's JWKS. It checks the signature, issuer, audience and
// validity window, and returns the claims the automation broker decides on.
// Authorising those claims is the caller's job.
package oidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// Leeway absorbs clock skew between GitHub and Lambda on exp, nbf and iat.
const Leeway = 30 * time.Second

// maxJWKSBytes bounds the JWKS response read.
const maxJWKSBytes = 1 << 20

// Claims are the GitHub Actions token claims the broker uses.
type Claims struct {
	Subject           string `json:"sub"`
	Repository        string `json:"repository"`
	RepositoryID      string `json:"repository_id"`
	RepositoryOwnerID string `json:"repository_owner_id"`
	Ref               string `json:"ref"`
	SHA               string `json:"sha"`
	EventName         string `json:"event_name"`
	Actor             string `json:"actor"`
	ActorID           string `json:"actor_id"`
	WorkflowRef       string `json:"workflow_ref"`
	JobWorkflowRef    string `json:"job_workflow_ref"`
	RunID             string `json:"run_id"`
	RunAttempt        string `json:"run_attempt"`
	CheckRunID        string `json:"check_run_id"`
}

// registered holds the RFC 7519 claims checked here.
type registered struct {
	Issuer    string   `json:"iss"`
	Audience  audience `json:"aud"`
	Expiry    *int64   `json:"exp"`
	NotBefore *int64   `json:"nbf"`
	IssuedAt  *int64   `json:"iat"`
}

// audience accepts aud as a string or a string array (RFC 7519 4.1.3).
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return errors.New("aud is neither a string nor an array of strings")
	}
	*a = many
	return nil
}

// Verifier checks tokens; an unknown kid refetches the JWKS (key rotation).
type Verifier struct {
	Issuer   string
	Audience string
	JWKSURL  string
	Client   *http.Client
	Now      func() time.Time

	mu   sync.Mutex
	keys map[string]*rsa.PublicKey
}

// Verify returns the claims of a valid RS256 token from Issuer for Audience.
func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("token is not a three-part JWT")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("alg %q is not RS256", header.Alg)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return nil, errors.New("signature does not verify")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var reg registered
	if err := json.Unmarshal(payload, &reg); err != nil {
		return nil, fmt.Errorf("parse registered claims: %w", err)
	}
	if err := v.checkRegistered(reg); err != nil {
		return nil, err
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	return &claims, nil
}

func (v *Verifier) checkRegistered(reg registered) error {
	if reg.Issuer != v.Issuer {
		return fmt.Errorf("iss %q is not %q", reg.Issuer, v.Issuer)
	}
	if !slices.Contains(reg.Audience, v.Audience) {
		return fmt.Errorf("aud %q does not include %q",
			[]string(reg.Audience), v.Audience)
	}
	now := v.Now()
	if reg.Expiry == nil ||
		!now.Before(time.Unix(*reg.Expiry, 0).Add(Leeway)) {
		return errors.New("token is expired or has no exp")
	}
	if reg.NotBefore != nil &&
		now.Add(Leeway).Before(time.Unix(*reg.NotBefore, 0)) {
		return errors.New("token is not yet valid (nbf)")
	}
	if reg.IssuedAt != nil &&
		now.Add(Leeway).Before(time.Unix(*reg.IssuedAt, 0)) {
		return errors.New("token was issued in the future (iat)")
	}
	return nil
}

// key returns kid's public key, refetching the JWKS once on a cache miss.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	keys, err := v.fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}
	v.keys = keys
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("kid %q is not in the JWKS", kid)
}

func (v *Verifier) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build JWKS request: %w", err)
	}
	resp, err := v.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch JWKS: HTTP %d", resp.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	body := io.LimitReader(resp.Body, maxJWKSBytes)
	if err := json.NewDecoder(body).Decode(&set); err != nil {
		return nil, fmt.Errorf("parse JWKS: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) == 0 || len(e) > 4 {
			return nil, fmt.Errorf("JWKS key %q is malformed", k.Kid)
		}
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}
	}
	return keys, nil
}
