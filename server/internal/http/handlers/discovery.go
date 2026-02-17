package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/anath2/language-app/internal/discovery"
	"github.com/go-chi/chi/v5"
)

type savePreferenceRequest struct {
	Topic  string  `json:"topic"`
	Weight float64 `json:"weight"`
}

func ListDiscoveryPreferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := sharedDiscovery.ListPreferences()
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if prefs == nil {
		prefs = []discovery.Preference{}
	}
	WriteJSON(w, http.StatusOK, prefs)
}

func SaveDiscoveryPreference(w http.ResponseWriter, r *http.Request) {
	var req savePreferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON payload"})
		return
	}
	if req.Topic == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"detail": "topic is required"})
		return
	}
	if req.Weight <= 0 {
		req.Weight = 1.0
	}
	pref, err := sharedDiscovery.SavePreference(req.Topic, req.Weight)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, pref)
}

func DeleteDiscoveryPreference(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !sharedDiscovery.DeletePreference(id) {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "preference not found"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func ListDiscoveryArticles(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	articles, total, err := sharedDiscovery.ListArticles(status, limit, offset)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"articles": articles, "total": total})
}

func GetDiscoveryArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	article, ok := sharedDiscovery.GetArticle(id)
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "article not found"})
		return
	}
	WriteJSON(w, http.StatusOK, article)
}

func DismissDiscoveryArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !sharedDiscovery.DismissArticle(id) {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "article not found or already dismissed"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func ImportDiscoveryArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	article, ok := sharedDiscovery.GetArticle(id)
	if !ok {
		WriteJSON(w, http.StatusNotFound, map[string]string{"detail": "article not found"})
		return
	}

	page, err := discovery.FetchPage(r.Context(), article.URL)
	if err != nil {
		WriteJSON(w, http.StatusBadGateway, map[string]string{"detail": "failed to fetch article: " + err.Error()})
		return
	}

	trans, err := sharedTranslations.Create(page.Body, "discovery")
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	sharedQueue.StartProcessing(trans.ID)
	sharedDiscovery.ImportArticle(id, trans.ID)

	WriteJSON(w, http.StatusOK, map[string]any{
		"translation_id": trans.ID,
		"article_id":     id,
	})
}

func TriggerDiscoveryRun(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	go func() {
		_ = sharedDiscoveryPipeline.Run(ctx, "manual")
	}()
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}
