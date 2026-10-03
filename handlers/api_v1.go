package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/mtlynch/picoshare/picoshare"
)

type (
	apiFile struct {
		ID            string     `json:"id"`
		Filename      string     `json:"filename"`
		URL           string     `json:"url"`
		Size          uint64     `json:"size"`
		ContentType   string     `json:"contentType"`
		Uploaded      time.Time  `json:"uploaded"`
		Expires       *time.Time `json:"expires"`
		Note          *string    `json:"note"`
		DownloadCount uint64     `json:"downloadCount"`
	}

	apiFilePostResponse struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
)

func (s Server) apiFilesGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		em, err := s.store.GetEntriesMetadata()
		if err != nil {
			log.Printf("failed to retrieve entries metadata: %v", err)
			http.Error(w, "Failed to retrieve files", http.StatusInternalServerError)
			return
		}
		sort.Slice(em, func(i, j int) bool {
			return em[i].Uploaded.After(em[j].Uploaded)
		})

		baseURL := baseURLFromRequest(r)
		files := make([]apiFile, 0, len(em))
		for _, m := range em {
			var expires *time.Time
			if m.Expires != picoshare.NeverExpire {
				t := m.Expires.Time()
				expires = &t
			}
			files = append(files, apiFile{
				ID:            m.ID.String(),
				Filename:      m.Filename.String(),
				URL:           entryURL(baseURL, m.ID),
				Size:          m.Size.UInt64(),
				ContentType:   string(m.ContentType),
				Uploaded:      m.Uploaded,
				Expires:       expires,
				Note:          m.Note.Value,
				DownloadCount: m.DownloadCount,
			})
		}

		respondJSON(w, files)
	}
}

func (s Server) apiFilesPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Files uploaded without an explicit expiration use the default file
		// lifetime from the settings.
		var expiration picoshare.ExpirationTime
		if r.URL.Query().Get("expiration") == "" {
			settings, err := s.store.ReadSettings()
			if err != nil {
				log.Printf("failed to read settings: %v", err)
				http.Error(w, "Failed to read settings", http.StatusInternalServerError)
				return
			}
			expiration = settings.DefaultFileLifetime.ExpirationFromTime(s.now())
		} else {
			var err error
			expiration, err = s.parseExpirationFromRequest(r)
			if err != nil {
				log.Printf("invalid expiration URL parameter: %v", err)
				http.Error(w, fmt.Sprintf("Invalid expiration URL parameter: %v", err), http.StatusBadRequest)
				return
			}
		}

		id, err := s.insertFileFromRequest(r, expiration, picoshare.GuestLinkID(""))
		if err != nil {
			if _, ok := errors.AsType[dbError](err); ok {
				log.Printf("failed to insert uploaded file into data store: %v", err)
				http.Error(w, "failed to insert file into database", http.StatusInternalServerError)
			} else {
				log.Printf("invalid upload: %v", err)
				http.Error(w, fmt.Sprintf("invalid request: %s", err), http.StatusBadRequest)
			}
			return
		}

		respondJSON(w, apiFilePostResponse{
			ID:  id.String(),
			URL: entryURL(baseURLFromRequest(r), id),
		})
	}
}

func entryURL(baseURL string, id picoshare.EntryID) string {
	return fmt.Sprintf("%s/-%s", baseURL, id.String())
}
