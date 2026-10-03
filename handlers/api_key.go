package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

type apiKeyPostResponse struct {
	APIKey  string    `json:"apiKey"`
	Created time.Time `json:"created"`
}

// apiKeyPost generates a new API key and replaces any existing one.
func (s Server) apiKeyPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := picoshare.NewAPIKey()
		// Match the second-level UTC precision of the stored timestamp.
		record := picoshare.APIKeyRecord{
			Hash:    key.Hash(),
			Created: s.now().UTC().Truncate(time.Second),
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

// requireAPIKey rejects requests that lack a bearer token matching the stored
// API key.
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

		h.ServeHTTP(w, r)
	})
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
