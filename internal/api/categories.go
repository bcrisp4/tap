package api

import (
	"encoding/json"
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

func (h *categoryHandlers) create(w http.ResponseWriter, r *http.Request) {
	var req categoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "bad_json", "name is required")
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
	id, ok := pathInt(r, "id")
	if !ok {
		WriteError(w, http.StatusBadRequest, "bad_id", "category id must be integer")
		return
	}
	var req categoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "bad_json", "name is required")
		return
	}
	if err := h.store.RenameCategory(r.Context(), userID, id, req.Name); err != nil {
		writeErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, &storage.Category{ID: id, UserID: userID, Name: req.Name})
}

func (h *categoryHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		WriteError(w, http.StatusBadRequest, "bad_id", "category id must be integer")
		return
	}
	if err := h.store.DeleteCategory(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
