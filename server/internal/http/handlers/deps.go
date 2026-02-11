package handlers

import (
	"errors"

	"github.com/anath2/language-app/internal/intelligence"
	"github.com/anath2/language-app/internal/queue"
	"github.com/anath2/language-app/internal/translation"
)

var sharedStore *translation.Store
var sharedQueue *queue.Manager
var sharedProvider intelligence.Provider

func ConfigureDependencies(store *translation.Store, manager *queue.Manager, provider intelligence.Provider) {
	sharedStore = store
	sharedQueue = manager
	sharedProvider = provider
}

func validateDependencies() error {
	if sharedStore == nil || sharedQueue == nil || sharedProvider == nil {
		return errors.New("application dependencies are not configured")
	}
	return nil
}
