package handler

import (
	"fmt"
	"net/http"
)

// HealthHandler returns 200 OK for liveness checks.
type HealthHandler struct{}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
