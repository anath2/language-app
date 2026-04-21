package handlers

import "net/http"

func DiscoveryStatus(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"status": "scaffold",
		"phase":  "backend-prep",
	})
}
