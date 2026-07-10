package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type Handler struct {
	Service *Service
}

func NewHTTPHandler(service *Service) http.Handler {
	return Handler{Service: service}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodGet && path == "/healthz":
		writeJSON(w, http.StatusOK, h.Service.Health())
	case r.Method == http.MethodGet && path == "/v1/snapshot":
		writeJSON(w, http.StatusOK, h.Service.Snapshot())
	case r.Method == http.MethodPost && path == "/v1/intents":
		h.handleSubmit(w, r)
	case r.Method == http.MethodPost && path == "/v1/quotes":
		h.handleQuote(w, r)
	case r.Method == http.MethodPost && path == "/v1/execute":
		h.handleExecute(w, r)
	case r.Method == http.MethodPost && path == "/v1/reconcile/exposure":
		h.handleExposure(w, r)
	case r.Method == http.MethodPost && path == "/v1/routes":
		h.handleRouteUpdate(w, r)
	case r.Method == http.MethodPost && path == "/v1/epoch":
		h.handleEpoch(w, r)
	default:
		writeError(w, http.StatusNotFound, domain.NotFound("endpoint not found"))
	}
}

func (h Handler) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var request domain.SubmitIntentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	response, err := h.Service.SubmitIntent(request)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (h Handler) handleQuote(w http.ResponseWriter, r *http.Request) {
	var request domain.SubmitIntentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	response, err := h.Service.QuoteIntent(request.Intent)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var request domain.ExecuteRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	response, err := h.Service.Execute(request)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) handleExposure(w http.ResponseWriter, r *http.Request) {
	var request domain.ExposureAdjustmentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.Service.AdjustExposure(request); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.Service.Snapshot())
}

func (h Handler) handleRouteUpdate(w http.ResponseWriter, r *http.Request) {
	var request domain.RouteUpdateRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	route, err := h.Service.UpdateRoute(request)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (h Handler) handleEpoch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Delta uint64 `json:"delta"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	epoch := h.Service.AdvanceEpoch(request.Delta)
	writeJSON(w, http.StatusOK, map[string]uint64{"epoch": epoch})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, domain.Invalid("invalid json body"))
		return false
	}
	return true
}

func writeDomainError(w http.ResponseWriter, err error) {
	if converted, ok := domain.AsDomainError(err); ok {
		switch converted.Code {
		case domain.CodeInvalidRequest:
			writeError(w, http.StatusBadRequest, converted)
		case domain.CodeNotFound:
			writeError(w, http.StatusNotFound, converted)
		case domain.CodeInsufficient, domain.CodeLimitExceeded, domain.CodeRouteUnavailable, domain.CodeConflict:
			writeError(w, http.StatusConflict, converted)
		default:
			writeError(w, http.StatusInternalServerError, converted)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, domain.Internal(err.Error()))
}

func writeError(w http.ResponseWriter, status int, err domain.Error) {
	writeJSON(w, status, domain.ErrorResponse{Error: err})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}
