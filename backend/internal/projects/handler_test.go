package projects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

type fakeStore struct {
	items []Project
}

func (f *fakeStore) Create(_ context.Context, userID uuid.UUID, name string, description *string) (Project, error) {
	name, err := ValidateName(name)
	if err != nil {
		return Project{}, err
	}
	project := Project{ID: uuid.New(), UserID: userID, Name: name, Description: description, Status: StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.items = append(f.items, project)
	return project, nil
}

func (f *fakeStore) Get(_ context.Context, userID, projectID uuid.UUID) (Project, error) {
	for _, item := range f.items {
		if item.ID == projectID && item.UserID == userID && item.Status != StatusDeleted {
			return item, nil
		}
	}
	return Project{}, ErrProjectNotFound
}

func (f *fakeStore) List(_ context.Context, userID uuid.UUID) ([]Project, error) {
	items := make([]Project, 0)
	for _, item := range f.items {
		if item.UserID == userID && item.Status != StatusDeleted {
			items = append(items, item)
		}
	}
	return items, nil
}

func (f *fakeStore) Update(_ context.Context, userID, projectID uuid.UUID, name string, description *string, status Status) (Project, error) {
	for i, item := range f.items {
		if item.ID != projectID || item.UserID != userID || item.Status == StatusDeleted {
			continue
		}
		if _, err := ValidateName(name); err != nil {
			return Project{}, err
		}
		if err := ValidateStatus(status); err != nil {
			return Project{}, err
		}
		f.items[i].Name, f.items[i].Description, f.items[i].Status = name, description, status
		f.items[i].UpdatedAt = time.Now()
		return f.items[i], nil
	}
	return Project{}, ErrProjectNotFound
}

func (f *fakeStore) Archive(_ context.Context, userID, projectID uuid.UUID) (Project, error) {
	for i, item := range f.items {
		if item.ID == projectID && item.UserID == userID && item.Status != StatusDeleted {
			f.items[i].Status = StatusArchived
			f.items[i].UpdatedAt = time.Now()
			return f.items[i], nil
		}
	}
	return Project{}, ErrProjectNotFound
}

func withPrincipal(req *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(req.Context(), reflectContextKey{}, auth.Principal{UserID: userID})
	return req.WithContext(ctx)
}

type reflectContextKey struct{}

func TestValidateProjectInput(t *testing.T) {
	if _, err := ValidateName("   "); !errors.Is(err, ErrInvalidProject) {
		t.Fatal("expected blank name rejection")
	}
	if _, err := ValidateName(strings.Repeat("a", 201)); !errors.Is(err, ErrInvalidProject) {
		t.Fatal("expected long name rejection")
	}
	if err := ValidateStatus(Status("broken")); !errors.Is(err, ErrInvalidProject) {
		t.Fatal("expected invalid status rejection")
	}
}

func TestProjectHandlerRequiresAuth(t *testing.T) {
	store := &fakeStore{}
	handler, err := NewHandler(store)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestProjectCRUDAndOwnership(t *testing.T) {
	store := &fakeStore{}
	handler, err := NewHandler(store)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Register(mux)

	userID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":" Studio ","description":"demo"}`))
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, auth.Principal{UserID: userID}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.items) != 1 || store.items[0].Name != "Studio" {
		t.Fatalf("unexpected store state: %+v", store.items)
	}

	projectID := store.items[0].ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String(), nil)
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, auth.Principal{UserID: userID}))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}

	otherID := uuid.New()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String(), nil)
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, auth.Principal{UserID: otherID}))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ownership status=%d", rec.Code)
	}
}

func TestProjectInputRejectsUnknownFields(t *testing.T) {
	store := &fakeStore{}
	handler, err := NewHandler(store)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.Register(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"ok","unknown":true}`))
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, auth.Principal{UserID: uuid.New()}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

type authContextKey struct{}
