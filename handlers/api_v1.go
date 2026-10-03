package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"

	"github.com/mtlynch/picoshare/handlers/parse"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
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

	// apiFilePatchRequest holds the metadata changes for an entry. A nil field
	// leaves that part of the metadata unchanged.
	apiFilePatchRequest struct {
		ID                 picoshare.EntryID
		Filename           *picoshare.Filename
		Expires            *picoshare.ExpirationTime
		Note               *picoshare.FileNote
		DownloadPassphrase *picoshare.DownloadPassphrase
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

func (s Server) apiFilePatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := s.parseAPIFilePatchRequest(r)
		if err != nil {
			log.Printf("invalid file edit request: %v", err)
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		metadata, err := s.store.GetEntryMetadata(req.ID)
		if _, ok := errors.AsType[store.EntryNotFoundError](err); ok {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		} else if err != nil {
			log.Printf("failed to retrieve entry %v: %v", req.ID, err)
			http.Error(w, "Failed to retrieve file", http.StatusInternalServerError)
			return
		}

		if req.Filename != nil {
			metadata.Filename = *req.Filename
		}
		if req.Expires != nil {
			metadata.Expires = *req.Expires
		}
		if req.Note != nil {
			metadata.Note = *req.Note
		}
		if req.DownloadPassphrase != nil {
			metadata.DownloadPassphrase = *req.DownloadPassphrase
		}

		if err := s.store.UpdateEntryMetadata(req.ID, metadata); err != nil {
			log.Printf("failed to save metadata for entry %v: %v", req.ID, err)
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
	}
}

func (s Server) parseAPIFilePatchRequest(r *http.Request) (apiFilePatchRequest, error) {
	id, err := picoshare.EntryIDFromString(mux.Vars(r)["id"])
	if err != nil {
		return apiFilePatchRequest{}, err
	}

	var payload struct {
		Filename           *string `json:"filename"`
		Expiration         *string `json:"expiration"`
		Note               *string `json:"note"`
		DownloadPassphrase *string `json:"downloadPassphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return apiFilePatchRequest{}, err
	}

	req := apiFilePatchRequest{ID: id}

	if payload.Filename != nil {
		filename, err := parse.Filename(*payload.Filename)
		if err != nil {
			return apiFilePatchRequest{}, err
		}
		req.Filename = &filename
	}

	// An empty expiration makes the file never expire.
	if payload.Expiration != nil {
		expiration := picoshare.NeverExpire
		if *payload.Expiration != "" {
			expiration, err = parse.Expiration(*payload.Expiration, s.now())
			if err != nil {
				return apiFilePatchRequest{}, err
			}
		}
		req.Expires = &expiration
	}

	// An empty note removes the file's note.
	if payload.Note != nil {
		note, err := parse.FileNote(*payload.Note)
		if err != nil {
			return apiFilePatchRequest{}, err
		}
		req.Note = &note
	}

	// An empty passphrase removes the file's download passphrase.
	if payload.DownloadPassphrase != nil {
		downloadPassphrase := picoshare.DownloadPassphrase{}
		if *payload.DownloadPassphrase != "" {
			downloadPassphrase, err = picoshare.NewDownloadPassphrase(*payload.DownloadPassphrase)
			if err != nil {
				return apiFilePatchRequest{}, err
			}
		}
		req.DownloadPassphrase = &downloadPassphrase
	}

	return req, nil
}

func (s Server) apiFileDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.EntryIDFromString(mux.Vars(r)["id"])
		if err != nil {
			log.Printf("invalid entry ID: %v", err)
			http.Error(w, fmt.Sprintf("Invalid file ID: %v", err), http.StatusBadRequest)
			return
		}

		// DeleteEntry succeeds even when the entry doesn't exist, so check first
		// to report missing files to the client.
		if _, err := s.store.GetEntryMetadata(id); err != nil {
			if _, ok := errors.AsType[store.EntryNotFoundError](err); ok {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			log.Printf("failed to retrieve entry %v: %v", id, err)
			http.Error(w, "Failed to retrieve file", http.StatusInternalServerError)
			return
		}

		if err := s.store.DeleteEntry(id); err != nil {
			log.Printf("failed to delete entry %v: %v", id, err)
			http.Error(w, "Failed to delete file", http.StatusInternalServerError)
			return
		}
	}
}

func entryURL(baseURL string, id picoshare.EntryID) string {
	return fmt.Sprintf("%s/-%s", baseURL, id.String())
}
