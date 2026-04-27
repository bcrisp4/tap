package api

import (
	"net/http"

	"github.com/bcrisp4/tap/internal/storage"
)

type categoryHandlers struct {
	store *storage.Store
}

func (h *categoryHandlers) list(w http.ResponseWriter, r *http.Request) {
	cats, err := h.store.ListCategories(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	WriteList(w, cats, len(cats), 0, len(cats))
}

type categoryReq struct {
	Name string `json:"name"`
}

// decodeCategoryReq decodes the body and enforces that name is set.
// Empty name shares the bad_json code so the test surface is uniform
// (a missing-field check is a body-validation error like a parse fail).
func decodeCategoryReq(w http.ResponseWriter, r *http.Request) (categoryReq, bool) {
	var req categoryReq
	if !decodeJSON(w, r, &req) {
		return req, false
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "bad_json", "name is required")
		return req, false
	}
	return req, true
}

func (h *categoryHandlers) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeCategoryReq(w, r)
	if !ok {
		return
	}
	id, err := h.store.CreateCategory(r.Context(), userID, req.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, &storage.Category{ID: id, UserID: userID, Name: req.Name})
}

func (h *categoryHandlers) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := requirePathID(w, r, "category")
	if !ok {
		return
	}
	req, ok := decodeCategoryReq(w, r)
	if !ok {
		return
	}
	if err := h.store.RenameCategory(r.Context(), userID, id, req.Name); err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, &storage.Category{ID: id, UserID: userID, Name: req.Name})
}

func (h *categoryHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := requirePathID(w, r, "category")
	if !ok {
		return
	}
	if err := h.store.DeleteCategory(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
