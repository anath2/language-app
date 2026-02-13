package handlers

import (
	"encoding/json"
	"net/http"
)

func ExportProgress(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	jsonContent, err := sharedSRS.ExportProgressJSON()
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"language_app_progress.json\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(jsonContent))
}

func ImportProgress(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid multipart payload"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid file type. Please upload a .json file."})
		return
	}
	defer file.Close()
	buf := make([]byte, 1<<20+1)
	n, _ := file.Read(buf)
	if n > 1<<20 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "File too large. Maximum size is 1024KB."})
		return
	}
	counts, err := sharedSRS.ImportProgressJSON(string(buf[:n]))
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"counts":  counts,
	})
}

func GetProfile(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	profile, ok := sharedProfile.GetUserProfile()
	var profileObj any
	if ok {
		profileObj = map[string]any{
			"name":       profile.Name,
			"email":      profile.Email,
			"language":   profile.Language,
			"created_at": profile.CreatedAt,
			"updated_at": profile.UpdatedAt,
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"profile": profileObj,
		"vocabStats": map[string]int{
			"known":    sharedSRS.CountVocabByStatus("known"),
			"learning": sharedSRS.CountVocabByStatus("learning"),
			"total":    sharedSRS.CountTotalVocab(),
		},
	})
}

func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if err := validateDependencies(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	name := payload["name"]
	email := payload["email"]
	language := payload["language"]
	profile, err := sharedProfile.UpsertUserProfile(name, email, language)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"profile": map[string]any{
			"name":       profile.Name,
			"email":      profile.Email,
			"language":   profile.Language,
			"created_at": profile.CreatedAt,
			"updated_at": profile.UpdatedAt,
		},
	})
}
