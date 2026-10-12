package cart

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/takemerightttt/cart-service/internal/auth"
	"github.com/takemerightttt/cart-service/internal/shared/httpjson"
)

type ErrorWriter func(w http.ResponseWriter, r *http.Request, err error)

type Middleware func(http.Handler) http.Handler

type Handler struct {
	svc        *Service
	writeErr   ErrorWriter
	idempotent Middleware
}

func NewHandler(svc *Service, writeErr ErrorWriter, idempotent Middleware) *Handler {
	return &Handler{svc: svc, writeErr: writeErr, idempotent: idempotent}
}

func (h *Handler) Register(mux *http.ServeMux, authn Middleware) {
	mux.Handle("POST /v1/carts", authn(http.HandlerFunc(h.open)))
	mux.Handle("GET /v1/carts/{cartID}", authn(http.HandlerFunc(h.get)))
	mux.Handle("POST /v1/carts/{cartID}/items", authn(h.idempotent(http.HandlerFunc(h.addItem))))
	mux.Handle("PATCH /v1/carts/{cartID}/items/{productID}", authn(http.HandlerFunc(h.updateItem)))
	mux.Handle("DELETE /v1/carts/{cartID}/items/{productID}", authn(http.HandlerFunc(h.removeItem)))
	mux.Handle("DELETE /v1/carts/{cartID}/items", authn(http.HandlerFunc(h.clear)))
}

func (h *Handler) open(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	c, created, err := h.svc.Open(r.Context(), userID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	w.Header().Set("Location", "/v1/carts/"+c.ID.String())
	httpjson.Write(w, status, c)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, cartID, ok := h.ids(w, r)
	if !ok {
		return
	}
	h.respond(w, r)(h.svc.Get(r.Context(), userID, cartID))
}

func (h *Handler) addItem(w http.ResponseWriter, r *http.Request) {
	userID, cartID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var in AddItemInput
	if err := httpjson.Decode(w, r, &in); err != nil {
		h.writeErr(w, r, err)
		return
	}
	h.respond(w, r)(h.svc.AddItem(r.Context(), userID, cartID, in))
}

func (h *Handler) updateItem(w http.ResponseWriter, r *http.Request) {
	userID, cartID, ok := h.ids(w, r)
	if !ok {
		return
	}
	productID, err := uuid.Parse(r.PathValue("productID"))
	if err != nil {
		h.writeErr(w, r, ErrItemNotFound)
		return
	}
	var in UpdateItemInput
	if err := httpjson.Decode(w, r, &in); err != nil {
		h.writeErr(w, r, err)
		return
	}
	h.respond(w, r)(h.svc.UpdateItem(r.Context(), userID, cartID, productID, in))
}

func (h *Handler) removeItem(w http.ResponseWriter, r *http.Request) {
	userID, cartID, ok := h.ids(w, r)
	if !ok {
		return
	}
	productID, err := uuid.Parse(r.PathValue("productID"))
	if err != nil {
		h.writeErr(w, r, ErrItemNotFound)
		return
	}
	h.respond(w, r)(h.svc.RemoveItem(r.Context(), userID, cartID, productID))
}

func (h *Handler) clear(w http.ResponseWriter, r *http.Request) {
	userID, cartID, ok := h.ids(w, r)
	if !ok {
		return
	}
	h.respond(w, r)(h.svc.Clear(r.Context(), userID, cartID))
}

func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, _ := auth.UserID(r.Context())
	cartID, err := uuid.Parse(r.PathValue("cartID"))
	if err != nil {
		h.writeErr(w, r, ErrNotFound)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, cartID, true
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request) func(Cart, error) {
	return func(c Cart, err error) {
		if err != nil {
			h.writeErr(w, r, err)
			return
		}
		httpjson.Write(w, http.StatusOK, c)
	}
}
