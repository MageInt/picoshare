package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

var contextKeyAPIKeyPermissions = new(contextKey{name: "api-key-permissions"})

type apiKeyPostResponse struct {
	APIKey  string    `json:"apiKey"`
	Created time.Time `json:"created"`
}

// apiKeyPost generates a new API key and replaces any existing one. The new
// key keeps the permissions of the key it replaces.
func (s Server) apiKeyPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		previous, err := s.store.ReadAPIKey()
		if _, ok := errors.AsType[store.APIKeyNotFoundError](err); !ok && err != nil {
			log.Printf("failed to read API key: %v", err)
			http.Error(w, "Failed to read API key", http.StatusInternalServerError)
			return
		}

		key := picoshare.NewAPIKey()
		// Match the second-level UTC precision of the stored timestamp.
		record := picoshare.APIKeyRecord{
			Hash:        key.Hash(),
			Created:     s.now().UTC().Truncate(time.Second),
			Permissions: previous.Permissions,
		}
		if err := s.store.UpdateAPIKey(record); err != nil {
			log.Printf("failed to save API key: %v", err)
			http.Error(w, "Failed to save API key", http.StatusInternalServerError)
			return
		}

		respondJSON(w, apiKeyPostResponse{
			APIKey:  key.String(),
			Created: record.Created,
		})
	}
}

// apiKeyPermissionsPut changes which operations the current API key allows.
func (s Server) apiKeyPermissionsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		permissions, err := parseAPIKeyPermissionsRequest(r)
		if err != nil {
			log.Printf("invalid API key permissions request: %v", err)
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		record, err := s.store.ReadAPIKey()
		if _, ok := errors.AsType[store.APIKeyNotFoundError](err); ok {
			http.Error(w, "Generate an API key before setting its permissions", http.StatusNotFound)
			return
		} else if err != nil {
			log.Printf("failed to read API key: %v", err)
			http.Error(w, "Failed to read API key", http.StatusInternalServerError)
			return
		}

		record.Permissions = permissions
		if err := s.store.UpdateAPIKey(record); err != nil {
			log.Printf("failed to save API key permissions: %v", err)
			http.Error(w, "Failed to save API key permissions", http.StatusInternalServerError)
			return
		}
	}
}

func parseAPIKeyPermissionsRequest(r *http.Request) (picoshare.APIKeyPermissions, error) {
	var payload struct {
		AllowEdit   *bool `json:"allowEdit"`
		AllowDelete *bool `json:"allowDelete"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return picoshare.APIKeyPermissions{}, err
	}
	if payload.AllowEdit == nil || payload.AllowDelete == nil {
		return picoshare.APIKeyPermissions{}, errors.New("allowEdit and allowDelete are required")
	}
	return picoshare.APIKeyPermissions{
		AllowEdit:   *payload.AllowEdit,
		AllowDelete: *payload.AllowDelete,
	}, nil
}

// requireAPIKey rejects requests that lack a bearer token matching the stored
// API key. It records the key's permissions in the request context.
func (s Server) requireAPIKey(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := parseAPIKeyFromRequest(r)
		if err != nil {
			respondAPIKeyRequired(w)
			return
		}

		record, err := s.store.ReadAPIKey()
		if _, ok := errors.AsType[store.APIKeyNotFoundError](err); ok {
			respondAPIKeyRequired(w)
			return
		} else if err != nil {
			log.Printf("failed to read API key: %v", err)
			http.Error(w, "Failed to verify API key", http.StatusInternalServerError)
			return
		}

		if !record.Hash.Equal(key.Hash()) {
			respondAPIKeyRequired(w)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyAPIKeyPermissions, record.Permissions)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAPIKeyPermission rejects requests whose API key lacks the permission
// that isAllowed checks. It must run after requireAPIKey.
func requireAPIKeyPermission(isAllowed func(picoshare.APIKeyPermissions) bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		permissions, ok := r.Context().Value(contextKeyAPIKeyPermissions).(picoshare.APIKeyPermissions)
		if !ok || !isAllowed(permissions) {
			http.Error(w, "The API key lacks permission for this operation", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func canEditFiles(p picoshare.APIKeyPermissions) bool {
	return p.AllowEdit
}

func canDeleteFiles(p picoshare.APIKeyPermissions) bool {
	return p.AllowDelete
}

func parseAPIKeyFromRequest(r *http.Request) (picoshare.APIKey, error) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return picoshare.APIKey{}, errors.New("missing bearer token")
	}
	return picoshare.APIKeyFromString(strings.TrimSpace(token))
}

func respondAPIKeyRequired(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, "Valid API key required", http.StatusUnauthorized)
}
