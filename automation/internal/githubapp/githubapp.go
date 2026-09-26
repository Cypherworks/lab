// Package githubapp authenticates as a GitHub App: it signs the RS256 app JWT
// and exchanges it for installation tokens that are always down-scoped to
// named repositories and permissions. It refuses to mint a full-scope token.
package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"
)

// apiVersion is the GitHub REST API version the client speaks.
const apiVersion = "2022-11-28"

// maxBody bounds every GitHub response read.
const maxBody = 1 << 20

// AppJWT signs the app JWT: iat 60s back for drift, exp inside 10 minutes.
func AppJWT(key *rsa.PrivateKey, issuer string, now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": issuer,
	})
	enc := base64.RawURLEncoding
	input := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign app JWT: %w", err)
	}
	return input + "." + enc.EncodeToString(sig), nil
}

// ParsePrivateKey reads the App's PEM key, PKCS#1 (GitHub's) or PKCS#8.
func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block in the App private key")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("App private key is not RSA")
		}
		return rsaKey, nil
	}
	return nil, fmt.Errorf("unexpected PEM block %q", block.Type)
}

// Client talks to the GitHub REST API as the App.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Key     *rsa.PrivateKey
	Issuer  string
	Now     func() time.Time
}

// TokenRequest names exactly what an installation token may reach.
type TokenRequest struct {
	RepositoryIDs []int64
	Permissions   map[string]string
}

// Token is an installation access token and its expiry.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// InstallationToken mints a token for org's installation, scoped to req.
func (c *Client) InstallationToken(ctx context.Context, org string, req TokenRequest) (Token, error) {
	if len(req.RepositoryIDs) == 0 || len(req.Permissions) == 0 {
		return Token{}, errors.New("refusing an installation token without repositories and permissions")
	}
	var inst struct {
		ID int64 `json:"id"`
	}
	path := "/orgs/" + org + "/installation"
	if err := c.do(ctx, http.MethodGet, path, nil, http.StatusOK, &inst); err != nil {
		return Token{}, err
	}
	body := map[string]any{
		"repository_ids": req.RepositoryIDs,
		"permissions":    req.Permissions,
	}
	var resp struct {
		Token       string            `json:"token"`
		ExpiresAt   time.Time         `json:"expires_at"`
		Permissions map[string]string `json:"permissions"`
	}
	path = fmt.Sprintf("/app/installations/%d/access_tokens", inst.ID)
	if err := c.do(ctx, http.MethodPost, path, body, http.StatusCreated, &resp); err != nil {
		return Token{}, err
	}
	if !grantMatches(resp.Permissions, req.Permissions) {
		return Token{}, fmt.Errorf("GitHub granted %v, requested %v",
			resp.Permissions, req.Permissions)
	}
	return Token{Value: resp.Token, ExpiresAt: resp.ExpiresAt}, nil
}

// grantMatches: the grant is the request, plus GitHub's forced metadata:read.
func grantMatches(granted, requested map[string]string) bool {
	extra := maps.Clone(granted)
	if extra["metadata"] == "read" && requested["metadata"] == "" {
		delete(extra, "metadata")
	}
	return maps.Equal(extra, requested)
}

// do sends an app-JWT request and decodes a response with the wanted status.
func (c *Client) do(ctx context.Context, method, path string, in any, want int, out any) error {
	jwt, err := AppJWT(c.Key, c.Issuer, c.Now())
	if err != nil {
		return err
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out)
	if err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}
