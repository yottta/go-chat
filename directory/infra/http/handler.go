package http

import (
	"encoding/json"
	"fmt"
	"github.com/yottta/chat/directory/app"
	"github.com/yottta/chat/directory/domain"
	"io"
	"log"
	"net/http"
)

type Handler struct {
	app      *app.App
	handlers map[handlerDescriptor]http.HandlerFunc
}

type handlerDescriptor struct {
	url    string
	method string
}

func NewHandler(app *app.App) http.Handler {
	handler := Handler{
		app:      app,
		handlers: map[handlerDescriptor]http.HandlerFunc{},
	}
	handler.registerClientsListHandler()
	handler.registerPingHandler()

	return &handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hF, ok := h.handlers[handlerDescriptor{
		url:    r.URL.Path,
		method: r.Method,
	}]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("server does not support the given request"))
		return
	}
	hF(w, r)
}

func (h *Handler) registerClientsListHandler() {
	hd := handlerDescriptor{
		url:    "/clients",
		method: http.MethodGet,
	}
	h.handlers[hd] = func(w http.ResponseWriter, r *http.Request) {
		clients, err := h.app.Clients.GetClients(r.Context())
		if err != nil {
			log.Printf("error during getting the list of clients: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
			return
		}
		resp := struct {
			Clients []domain.Client `json:"clients"`
		}{
			Clients: clients,
		}

		m, err := json.Marshal(resp)
		if err != nil {
			log.Printf("error during marshalling the clients list response: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(m)
	}
}

func (h *Handler) registerPingHandler() {
	hd := handlerDescriptor{
		url:    "/ping",
		method: http.MethodPut,
	}
	h.handlers[hd] = func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("no body"))
			return
		}
		defer func() {
			_ = r.Body.Close()
		}()

		all, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("error during reading request body: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("malformed body"))
			return
		}

		var c domain.Client
		if err := json.Unmarshal(all, &c); err != nil {
			log.Printf("error during unmarshalling request body: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("malformed body"))
			return
		}
		if err := c.Validate(); err != nil {
			log.Printf("error during validating the client ping body: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"message": "%s"}`, err.Error())))
			return
		}
		if err := h.app.Clients.RegisterClient(r.Context(), c); err != nil {
			log.Printf("error during processing client registration request: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
			return
		}
	}
}
