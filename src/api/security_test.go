package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/solguardlabs/compassdtl/src/api"
	"github.com/solguardlabs/compassdtl/src/scenario"
)

func TestHTTPHandlerEmitsDefensiveHeaders(t *testing.T) {
	service, err := api.NewService(scenario.DefaultBootstrap())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	api.NewHTTPHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	for _, header := range []string{"Cache-Control", "Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("missing header %s", header)
		}
	}
}

func TestHTTPHandlerRejectsTrailingJSON(t *testing.T) {
	service, _ := api.NewService(scenario.DefaultBootstrap())
	request := httptest.NewRequest(http.MethodPost, "/v1/execute", strings.NewReader(`{"count":1}{"count":2}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	api.NewHTTPHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestProductionServerHasBoundedTimeouts(t *testing.T) {
	server := api.NewProductionServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts are not bounded: %+v", server)
	}
	if server.MaxHeaderBytes != 32<<10 {
		t.Fatalf("max header bytes = %d", server.MaxHeaderBytes)
	}
}
