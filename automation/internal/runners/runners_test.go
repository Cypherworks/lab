package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const token = "github_pat_example"

// fakeGitHub holds an org's runners and enforces GitHub's REST headers.
type fakeGitHub struct {
	t       *testing.T
	mu      sync.Mutex
	runners map[int64]map[string]any
	nextID  int64
	groups  []map[string]any
	status  int
	minted  map[string]any
	pages   []int
	config  string
}

func newFake(t *testing.T) (*fakeGitHub, *Client) {
	t.Helper()
	g := &fakeGitHub{t: t, runners: map[int64]map[string]any{}, nextID: 4193,
		config: "ZW5jb2RlZA=="}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	return g, &Client{BaseURL: srv.URL, HTTP: srv.Client(), Token: token, Org: "example"}
}

func (g *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+token {
		http.Error(w, "bad credentials", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" ||
		r.Header.Get("Accept") != "application/vnd.github+json" {
		http.Error(w, "missing REST headers", http.StatusBadRequest)
		return
	}
	if g.status != 0 {
		w.WriteHeader(g.status)
		fmt.Fprint(w, `{"message":"jit config secret-looking body"}`)
		return
	}
	const base = "/orgs/example/actions"
	path := strings.TrimPrefix(r.URL.Path, base)
	switch {
	case r.Method == http.MethodGet && path == "/runner-groups":
		writeJSON(w, http.StatusOK, map[string]any{
			"total_count": len(g.groups), "runner_groups": g.groups,
		})
	case r.Method == http.MethodPost && path == "/runners/generate-jitconfig":
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g.minted = body
		id := g.nextID
		g.nextID++
		labels := []map[string]any{}
		for _, l := range body["labels"].([]any) {
			labels = append(labels, map[string]any{"name": l})
		}
		runner := map[string]any{
			"id": id, "name": body["name"], "status": "offline",
			"busy": false, "ephemeral": nil, "labels": labels,
		}
		g.runners[id] = runner
		writeJSON(w, http.StatusCreated, map[string]any{
			"runner": runner, "encoded_jit_config": g.config,
		})
	case r.Method == http.MethodGet && path == "/runners":
		g.list(w, r)
	case strings.HasPrefix(path, "/runners/"):
		id, err := strconv.ParseInt(strings.TrimPrefix(path, "/runners/"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		runner, ok := g.runners[id]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, runner)
		case http.MethodDelete:
			delete(g.runners, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

// list serves runners in id order, per_page at a time, as GitHub pages them.
func (g *fakeGitHub) list(w http.ResponseWriter, r *http.Request) {
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if perPage < 1 || page < 1 {
		http.Error(w, "want per_page and page", http.StatusBadRequest)
		return
	}
	g.pages = append(g.pages, page)
	ids := []int64{}
	for id := range g.runners {
		ids = append(ids, id)
	}
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	out := []map[string]any{}
	for i := (page - 1) * perPage; i < len(ids) && i < page*perPage; i++ {
		out = append(out, g.runners[ids[i]])
	}
	writeJSON(w, http.StatusOK, map[string]any{"total_count": len(ids), "runners": out})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestMintJITSendsNameGroupAndLabelsAndReturnsTheConfig(t *testing.T) {
	g, c := newFake(t)
	r, config, err := c.MintJIT(context.Background(), "docker-runner-1-20261122T150006", 1,
		[]string{"self-hosted", "docker"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name":            "docker-runner-1-20261122T150006",
		"runner_group_id": float64(1),
		"labels":          []any{"self-hosted", "docker"},
	}
	if !reflect.DeepEqual(g.minted, want) {
		t.Fatalf("request body = %v, want %v", g.minted, want)
	}
	if r.ID != 4193 || r.Name != "docker-runner-1-20261122T150006" || r.Status != "offline" {
		t.Fatalf("runner = %+v", r)
	}
	if config != "ZW5jb2RlZA==" {
		t.Fatalf("config = %q", config)
	}
}

func TestMintJITFailureNamesTheStatusButNotTheBody(t *testing.T) {
	g, c := newFake(t)
	g.status = http.StatusUnprocessableEntity
	_, config, err := c.MintJIT(context.Background(), "docker-runner-1-x", 1, []string{"docker"})
	if err == nil {
		t.Fatal("mint succeeded on a 422")
	}
	if config != "" {
		t.Fatalf("config = %q on failure", config)
	}
	if !strings.Contains(err.Error(), "422") || strings.Contains(err.Error(), "secret-looking") {
		t.Fatalf("error = %q, want the status and no response body", err)
	}
}

func TestMintJITWithoutAConfigIsAnError(t *testing.T) {
	g, c := newFake(t)
	g.config = ""
	if _, _, err := c.MintJIT(context.Background(), "x", 1, []string{"docker"}); err == nil {
		t.Fatal("mint succeeded without an encoded_jit_config")
	}
}

func TestNoErrorCarriesTheToken(t *testing.T) {
	_, c := newFake(t)
	c.Token = "github_pat_wrong"
	_, _, err := c.MintJIT(context.Background(), "x", 1, []string{"docker"})
	if err == nil || strings.Contains(err.Error(), "github_pat") {
		t.Fatalf("error = %v, want a refusal without the token", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want the 401", err)
	}
}

func TestGetReportsStatusAndBusy(t *testing.T) {
	g, c := newFake(t)
	g.runners[7] = map[string]any{"id": 7, "name": "docker-runner-2-x", "status": "online", "busy": true}
	r, err := c.Get(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if r != (Runner{ID: 7, Name: "docker-runner-2-x", Status: "online", Busy: true}) {
		t.Fatalf("runner = %+v", r)
	}
}

func TestGetOfAGoneRunnerIsErrNotFound(t *testing.T) {
	_, c := newFake(t)
	_, err := c.Get(context.Background(), 99)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetOtherFailuresAreNotErrNotFound(t *testing.T) {
	g, c := newFake(t)
	g.status = http.StatusBadGateway
	_, err := c.Get(context.Background(), 7)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want a non-NotFound error", err)
	}
}

func TestDeleteRemovesTheRunnerAndToleratesOneAlreadyGone(t *testing.T) {
	g, c := newFake(t)
	g.runners[7] = map[string]any{"id": 7, "name": "docker-runner-1-x", "status": "offline"}
	if err := c.Delete(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if _, ok := g.runners[7]; ok {
		t.Fatal("runner still registered")
	}
	if err := c.Delete(context.Background(), 7); err != nil {
		t.Fatalf("second delete: %v, want nil for a runner already gone", err)
	}
}

func TestDeleteFailureIsAnError(t *testing.T) {
	g, c := newFake(t)
	g.status = http.StatusInternalServerError
	if err := c.Delete(context.Background(), 7); err == nil {
		t.Fatal("delete succeeded on a 500")
	}
}

func TestDefaultGroupIDIsTheGroupMarkedDefault(t *testing.T) {
	g, c := newFake(t)
	g.groups = []map[string]any{
		{"id": 5, "name": "crackingday", "default": false},
		{"id": 1, "name": "Default", "default": true},
	}
	id, err := c.DefaultGroupID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("id = %d, want 1", id)
	}
}

func TestDefaultGroupIDWithoutADefaultGroupIsAnError(t *testing.T) {
	g, c := newFake(t)
	g.groups = []map[string]any{{"id": 5, "name": "other", "default": false}}
	if _, err := c.DefaultGroupID(context.Background()); err == nil {
		t.Fatal("no error without a default group")
	}
}

func TestListReadsEveryPage(t *testing.T) {
	g, c := newFake(t)
	for id := int64(1); id <= 150; id++ {
		g.runners[id] = map[string]any{"id": id, "name": fmt.Sprintf("r-%d", id), "status": "offline"}
	}
	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 150 || got[0].ID != 1 || got[149].ID != 150 {
		t.Fatalf("got %d runners, want ids 1 to 150", len(got))
	}
	if !reflect.DeepEqual(g.pages, []int{1, 2}) {
		t.Fatalf("pages fetched = %v, want [1 2]", g.pages)
	}
}

func TestListFailureIsAnError(t *testing.T) {
	g, c := newFake(t)
	g.status = http.StatusServiceUnavailable
	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("list succeeded on a 503")
	}
}
