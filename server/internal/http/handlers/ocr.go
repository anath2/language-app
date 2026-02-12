package handlers

import "net/http"

func ExtractText(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid multipart payload"})
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Image file is required"})
		return
	}
	_ = file.Close()

	// Intelligence layer deferred: return stable contract-compatible placeholder.
	WriteJSON(w, http.StatusOK, map[string]string{"text": ""})
}
