package product

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/takemerightttt/cart-service/internal/shared/httpjson"
	"github.com/takemerightttt/cart-service/internal/shared/pagination"
)

type ErrorWriter func(w http.ResponseWriter, r *http.Request, err error)

type Handler struct {
	svc      *Service
	writeErr ErrorWriter
}

func NewHandler(svc *Service, writeErr ErrorWriter) *Handler {
	return &Handler{svc: svc, writeErr: writeErr}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/products", h.create)
	mux.HandleFunc("GET /v1/products", h.list)
	mux.HandleFunc("GET /v1/products/{id}", h.get)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpjson.Decode(w, r, &in); err != nil {
		h.writeErr(w, r, err)
		return
	}
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/products/"+p.ID.String())
	httpjson.Write(w, http.StatusCreated, p)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.writeErr(w, r, ErrNotFound)
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpjson.Write(w, http.StatusOK, p)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := pagination.ParseLimit(q.Get("limit"), DefaultPageSize, MaxPageSize)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	page, err := h.svc.List(r.Context(), q.Get("cursor"), limit)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpjson.Write(w, http.StatusOK, page)
}
