package iterations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

type fakeStore struct{ items []Iteration }

func (f *fakeStore) Create(_ context.Context, _ uuid.UUID, projectID uuid.UUID, parentID *uuid.UUID, typ Type, title, description *string) (Iteration, error) {
	if err := ValidateType(typ); err != nil { return Iteration{}, err }
	item := Iteration{ID: uuid.New(), ProjectID: projectID, ParentIterationID: parentID, Type: typ, Title: title, Description: description}
	f.items = append(f.items, item)
	return item, nil
}
func (f *fakeStore) Get(_ context.Context, _ uuid.UUID, id uuid.UUID) (Iteration, error) {
	for _, item := range f.items { if item.ID == id { return item, nil } }
	return Iteration{}, ErrIterationNotFound
}
func (f *fakeStore) List(_ context.Context, _ uuid.UUID, projectID uuid.UUID) ([]Iteration, error) {
	items := make([]Iteration, 0)
	for _, item := range f.items { if item.ProjectID == projectID { items = append(items, item) } }
	return items, nil
}
func (f *fakeStore) Restore(_ context.Context, _ uuid.UUID, id uuid.UUID) (Iteration, error) {
	for _, item := range f.items {
		if item.ID == id {
			parent := item.ID
			restored := item
			restored.ID = uuid.New()
			restored.ParentIterationID = &parent
			f.items = append(f.items, restored)
			return restored, nil
		}
	}
	return Iteration{}, ErrIterationNotFound
}

func TestValidateTypesAndLengths(t *testing.T) {
	if err := ValidateType(Type("unknown")); err == nil { t.Fatal("expected invalid type") }
	title := " "
	if err := ValidateTitle(&title); err == nil { t.Fatal("expected invalid title") }
}

func TestRestoreCreatesNewNode(t *testing.T) {
	projectID, userID := uuid.New(), uuid.New()
	source := Iteration{ID: uuid.New(), ProjectID: projectID, Type: TypePrompt}
	store := &fakeStore{items: []Iteration{source}}
	handler, err := NewHandler(store)
	if err != nil { t.Fatal(err) }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/iterations/"+source.ID.String()+"/restore", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: userID}))
	rec := httptest.NewRecorder()
	handler.restore(rec, req)

	if rec.Code != http.StatusCreated { t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated) }
	if len(store.items) != 2 { t.Fatalf("items = %d, want 2", len(store.items)) }
	if store.items[0].ID != source.ID { t.Fatal("source iteration was mutated") }
	if store.items[1].ID == source.ID { t.Fatal("restore reused source id") }
	if store.items[1].ParentIterationID == nil || *store.items[1].ParentIterationID != source.ID { t.Fatal("restore parent = %v, want source", store.items[1].ParentIterationID) }
}

func TestCreateRejectsUnknownFields(t *testing.T) {
	store := &fakeStore{}
	handler, _ := NewHandler(store)
	projectID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/iterations", strings.NewReader(`{"type":"idea","unknown":true}`))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: uuid.New()}))
	rec := httptest.NewRecorder()
	handler.create(rec, req)
	if rec.Code != http.StatusBadRequest { t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest) }
}
