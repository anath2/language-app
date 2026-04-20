# HTTP Handler Dependency Injection Growth Plan Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace package-level handler globals with explicit constructor-injected dependencies so the HTTP layer remains easy to test and maintain as the project grows.

**Architecture:** Introduce a typed HTTP app dependency graph built once in `server/internal/http/server.go`, then register routes with handler instances instead of package functions backed by global vars. Migrate by domain (translation/chat first), keep compatibility shims briefly, and remove `ConfigureDependencies` only after all route groups and tests are switched.

**Tech Stack:** Go (`net/http`, `chi`), existing `translation`, `queue`, and `intelligence` packages; standard `go test`.

---

## File Map

- `server/internal/http/server.go`  
  Build concrete stores/providers/queue once, then build handler instances and pass them into route registration.
- `server/internal/http/handlers/deps.go`  
  Convert from package-global registry to typed dependency structs and constructors.
- `server/internal/http/handlers/translation.go`  
  Move handler logic to receiver methods (`func (h *TranslationHandler) ...`).
- `server/internal/http/handlers/chat.go`  
  Move chat endpoints to receiver methods sharing explicit dependencies.
- `server/internal/http/handlers/admin.go`  
  Move admin/profile endpoints to receiver methods.
- `server/internal/http/handlers/vocab.go`  
  Move vocab/review endpoints to receiver methods.
- `server/internal/http/routes/translation.go`  
  Register routes against injected handler instances.
- `server/internal/http/routes/vocab.go`
- `server/internal/http/routes/review.go`
- `server/internal/http/routes/admin.go`
- `server/tests/integration/chat_rest_test.go`  
  Stop mutating global handler state from tests; inject test dependencies via router setup.
- `server/internal/http/server_test.go` and `server/internal/http/server_internal_test.go`  
  Update to assert constructor wiring/route registration via injected handlers.

---

## Task 1: Introduce explicit HTTP dependency and handler constructors

**Files:**
- Modify: `server/internal/http/handlers/deps.go`
- Create: `server/internal/http/handlers/constructors.go`
- Test: `server/internal/http/server_internal_test.go`

- [ ] **Step 1: Define a typed dependency container used only at startup**

Add a struct grouping concrete interface dependencies currently stored in globals:

```go
type HTTPDependencies struct {
	Translations  translationStore
	Chats         chatStore
	SRS           srsStore
	Profiles      profileStore
	Queue         *queue.Manager
	TransProvider intelligence.TranslationProvider
	ChatProvider  intelligence.ChatProvider
}
```

- [ ] **Step 2: Add fail-fast validation for startup wiring**

Add:

```go
func (d HTTPDependencies) Validate() error
```

with the same nil checks currently in `validateDependencies`, but called once during startup.

- [ ] **Step 3: Add per-domain constructors**

Create constructor signatures (in `constructors.go`) such as:

```go
func NewTranslationHandler(d HTTPDependencies) (*TranslationHandler, error)
func NewChatHandler(d HTTPDependencies) (*ChatHandler, error)
func NewVocabHandler(d HTTPDependencies) (*VocabHandler, error)
func NewAdminHandler(d HTTPDependencies) (*AdminHandler, error)
```

- [ ] **Step 4: Add a startup-level test**

Add a test that verifies constructor returns an error when one required dependency is nil.

- [ ] **Step 5: Run tests**

Run:

```bash
cd server && go test ./internal/http/ -run 'Test.*Dependencies|Test.*Constructor' -v
```

Expected: tests pass and prove startup validation works.

---

## Task 2: Migrate translation and chat endpoints from package funcs to methods

**Files:**
- Modify: `server/internal/http/handlers/translation.go`
- Modify: `server/internal/http/handlers/chat.go`
- Modify: `server/internal/http/routes/translation.go`
- Test: `server/tests/integration/chat_rest_test.go`

- [ ] **Step 1: Create handler structs with explicit fields**

Define:

```go
type TranslationHandler struct {
	translations  translationStore
	jobQueue      *queue.Manager
	transProvider intelligence.TranslationProvider
}

type ChatHandler struct {
	chats        chatStore
	translations translationStore
	chatProvider intelligence.ChatProvider
}
```

- [ ] **Step 2: Convert route targets to methods**

Convert existing functions, for example:

```go
func (h *TranslationHandler) CreateTranslation(w http.ResponseWriter, r *http.Request)
func (h *TranslationHandler) TranslationStream(w http.ResponseWriter, r *http.Request)
func (h *ChatHandler) CreateChatMessage(w http.ResponseWriter, r *http.Request)
```

- [ ] **Step 3: Update translation route registration to accept handler instances**

Change route registration signature to:

```go
func RegisterTranslationRoutes(r chi.Router, th *handlers.TranslationHandler, ch *handlers.ChatHandler)
```

- [ ] **Step 4: Update integration test setup**

Replace direct `handlers.ConfigureDependencies(...)` calls with router construction that injects mocks through constructors.

- [ ] **Step 5: Run targeted tests**

Run:

```bash
cd server && go test ./tests/integration -run 'TestTranslationChat' -v
```

Expected: chat SSE lifecycle tests still pass with no package-global dependency mutation.

---

## Task 3: Migrate vocab/review/admin handlers to explicit method receivers

**Files:**
- Modify: `server/internal/http/handlers/vocab.go`
- Modify: `server/internal/http/handlers/admin.go`
- Modify: `server/internal/http/routes/vocab.go`
- Modify: `server/internal/http/routes/review.go`
- Modify: `server/internal/http/routes/admin.go`
- Test: `server/internal/http/routes/routes_test.go`

- [ ] **Step 1: Introduce `VocabHandler` and `AdminHandler`**

Lift required stores to explicit fields and convert exported package funcs to receiver methods.

- [ ] **Step 2: Update route signatures**

Change route registration funcs to accept the handler instances they register.

- [ ] **Step 3: Update route tests**

Adjust route tests to build handlers with test doubles and assert route availability/status codes.

- [ ] **Step 4: Run route tests**

Run:

```bash
cd server && go test ./internal/http/routes -v
```

Expected: all route tests pass after signatures change.

---

## Task 4: Move composition root wiring into startup and remove global wiring API

**Files:**
- Modify: `server/internal/http/server.go`
- Modify: `server/internal/http/routes/*.go`
- Delete/Modify: `server/internal/http/handlers/deps.go`
- Test: `server/internal/http/server_test.go`

- [ ] **Step 1: Build all concrete dependencies once in `initDependencies`**

Keep existing DB/store/provider/queue creation, but return initialized handler instances instead of calling `handlers.ConfigureDependencies(...)`.

- [ ] **Step 2: Register routes with injected handler instances**

Update:

```go
registerRoutes(r, cfg, sessionManager, handlersBundle)
```

where `handlersBundle` contains constructed handler pointers.

- [ ] **Step 3: Remove `ConfigureDependencies` and request-time dependency guards**

Delete global mutable vars and remove `validateDependencies()` checks from endpoints now that constructors enforce invariants.

- [ ] **Step 4: Run HTTP package tests**

Run:

```bash
cd server && go test ./internal/http/... -v
```

Expected: initialization and routing tests pass with explicit DI.

---

## Task 5: Final cleanup, formatting, and regression verification

**Files:**
- Modify: any touched files above
- Test: `server/tests/integration/*.go`

- [ ] **Step 1: Remove compatibility shims**

If temporary wrapper funcs were introduced during migration, remove them now so route registration only uses receiver methods.

- [ ] **Step 2: Format all server code**

Run:

```bash
cd server && gofmt -w .
```

- [ ] **Step 3: Run full backend test suite**

Run:

```bash
cd server && go test ./...
```

Expected: full suite passes and there are no references to `handlers.ConfigureDependencies`.

- [ ] **Step 4: Verify no global dependency state remains**

Run:

```bash
cd server && rg 'ConfigureDependencies|validateDependencies|var transProvider|var chatProvider' internal/http
```

Expected: no matches for removed global wiring patterns (or only historical comments/tests if intentionally kept).

---

## Scope Notes

- This plan intentionally keeps package locations mostly stable (`internal/http/handlers`) to minimize risk in the first migration.
- A follow-up plan can split handlers into domain packages (`internal/http/translation`, `internal/http/chat`, etc.) once explicit DI is complete.
- The migration order is chosen so translation/chat (highest churn) is stabilized first before broad route rewiring.
