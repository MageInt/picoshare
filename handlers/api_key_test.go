package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtlynch/picoshare/handlers"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

func mustCreateAPIKey(t *testing.T, raw string) picoshare.APIKey {
	t.Helper()
	key, err := picoshare.APIKeyFromString(raw)
	if err != nil {
		t.Fatalf("failed to create API key: %v", err)
	}
	return key
}

func TestAPIFilesGetAuthentication(t *testing.T) {
	for _, tt := range []struct {
		explanation         string
		apiKeyInStore       string
		authorizationHeader string
		status              int
	}{
		{
			explanation:         "a request with the stored API key should succeed",
			apiKeyInStore:       "ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			authorizationHeader: "Bearer ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			status:              http.StatusOK,
		},
		{
			explanation:         "a request without an Authorization header should be rejected",
			apiKeyInStore:       "ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			authorizationHeader: "",
			status:              http.StatusUnauthorized,
		},
		{
			explanation:         "a request with a different API key should be rejected",
			apiKeyInStore:       "ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			authorizationHeader: "Bearer ps_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			status:              http.StatusUnauthorized,
		},
		{
			explanation:         "a request with a malformed API key should be rejected",
			apiKeyInStore:       "ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			authorizationHeader: "Bearer banana",
			status:              http.StatusUnauthorized,
		},
		{
			explanation:         "a request with a non-bearer scheme should be rejected",
			apiKeyInStore:       "ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			authorizationHeader: "Basic ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			status:              http.StatusUnauthorized,
		},
		{
			explanation:         "a request should be rejected when no API key exists",
			apiKeyInStore:       "",
			authorizationHeader: "Bearer ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			status:              http.StatusUnauthorized,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			if tt.apiKeyInStore != "" {
				if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
					Hash:    mustCreateAPIKey(t, tt.apiKeyInStore).Hash(),
					Created: mustParseTime("2025-01-01T00:00:00Z"),
				}); err != nil {
					t.Fatalf("failed to store API key: %v", err)
				}
			}
			s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
			if tt.authorizationHeader != "" {
				req.Header.Set("Authorization", tt.authorizationHeader)
			}
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if got, want := rec.Code, tt.status; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
		})
	}
}

func TestAPIFilesGetListsFiles(t *testing.T) {
	dataStore := test_sqlite.New(t)
	apiKey := "ps_0123456789abcdefghijABCDEFGHIJ0123456789"
	if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
		Hash:    mustCreateAPIKey(t, apiKey).Hash(),
		Created: mustParseTime("2025-01-01T00:00:00Z"),
	}); err != nil {
		t.Fatalf("failed to store API key: %v", err)
	}
	if err := dataStore.InsertEntry(strings.NewReader("older file"), picoshare.UploadMetadata{
		ID:          picoshare.MustCreateEntryID("AAAAAAAAAA"),
		Filename:    "older.txt",
		ContentType: "text/plain",
		Uploaded:    mustParseTime("2025-01-01T00:00:00Z"),
		Expires:     picoshare.NeverExpire,
		Size:        mustParseFileSize(len("older file")),
	}); err != nil {
		t.Fatalf("failed to insert entry: %v", err)
	}
	if err := dataStore.InsertEntry(strings.NewReader("newer file"), picoshare.UploadMetadata{
		ID:                 picoshare.MustCreateEntryID("BBBBBBBBBB"),
		Filename:           "newer.txt",
		ContentType:        "text/plain",
		Uploaded:           mustParseTime("2025-02-01T00:00:00Z"),
		Expires:            mustParseExpirationTime("2030-01-01T00:00:00Z"),
		Size:               mustParseFileSize(len("newer file")),
		DownloadPassphrase: mustCreateDownloadPassphrase(t, "secret passphrase"),
	}); err != nil {
		t.Fatalf("failed to insert entry: %v", err)
	}
	if err := dataStore.InsertEntryDownload(picoshare.MustCreateEntryID("BBBBBBBBBB"), picoshare.DownloadRecord{
		Time:      mustParseTime("2025-02-02T00:00:00Z"),
		ClientIP:  "203.0.113.1",
		UserAgent: "curl/8.0",
	}); err != nil {
		t.Fatalf("failed to record download: %v", err)
	}
	s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}

	expectedBody := `[
		{
			"id": "BBBBBBBBBB",
			"filename": "newer.txt",
			"url": "http://example.com/-BBBBBBBBBB",
			"size": 10,
			"contentType": "text/plain",
			"uploaded": "2025-02-01T00:00:00Z",
			"expires": "2030-01-01T00:00:00Z",
			"note": null,
			"downloadCount": 1
		},
		{
			"id": "AAAAAAAAAA",
			"filename": "older.txt",
			"url": "http://example.com/-AAAAAAAAAA",
			"size": 10,
			"contentType": "text/plain",
			"uploaded": "2025-01-01T00:00:00Z",
			"expires": null,
			"note": null,
			"downloadCount": 0
		}
	]`
	var compactExpected bytes.Buffer
	if err := json.Compact(&compactExpected, []byte(expectedBody)); err != nil {
		t.Fatalf("failed to compact expected body: %v", err)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), compactExpected.String(); got != want {
		t.Errorf("body=%s, want=%s", got, want)
	}
	if strings.Contains(rec.Body.String(), "secret passphrase") {
		t.Errorf("response must not contain the download passphrase")
	}
}

func TestAPIFilesPost(t *testing.T) {
	for _, tt := range []struct {
		explanation         string
		authorizationHeader string
		expirationParam     string
		status              int
		expiresExpected     string
	}{
		{
			explanation:         "an upload without an expiration should use the default lifetime of 30 days",
			authorizationHeader: "Bearer ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			expirationParam:     "",
			status:              http.StatusOK,
			expiresExpected:     "2025-01-31T00:00:00Z",
		},
		{
			explanation:         "an upload with an explicit expiration should use that expiration",
			authorizationHeader: "Bearer ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			expirationParam:     "2026-06-01T00:00:00Z",
			status:              http.StatusOK,
			expiresExpected:     "2026-06-01T00:00:00Z",
		},
		{
			explanation:         "an upload with an invalid expiration should be rejected",
			authorizationHeader: "Bearer ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			expirationParam:     "banana",
			status:              http.StatusBadRequest,
		},
		{
			explanation:         "an upload without an API key should be rejected",
			authorizationHeader: "",
			expirationParam:     "",
			status:              http.StatusUnauthorized,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
				Hash:    mustCreateAPIKey(t, "ps_0123456789abcdefghijABCDEFGHIJ0123456789").Hash(),
				Created: mustParseTime("2025-01-01T00:00:00Z"),
			}); err != nil {
				t.Fatalf("failed to store API key: %v", err)
			}
			now := mustParseTime("2025-01-01T00:00:00Z")
			s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, func() time.Time { return now })

			formData, contentType := createMultipartFormBody("upload.txt", "", "", strings.NewReader("uploaded contents"))
			url := "http://example.com/api/v1/files"
			if tt.expirationParam != "" {
				url += "?expiration=" + tt.expirationParam
			}
			req := httptest.NewRequest(http.MethodPost, url, formData)
			req.Header.Set("Content-Type", contentType)
			if tt.authorizationHeader != "" {
				req.Header.Set("Authorization", tt.authorizationHeader)
			}
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if got, want := rec.Code, tt.status; got != want {
				t.Fatalf("status=%d, want=%d", got, want)
			}
			if rec.Code != http.StatusOK {
				return
			}

			var response struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("response is not valid JSON: %s", rec.Body.String())
			}
			if got, want := response.URL, "http://example.com/-"+response.ID; got != want {
				t.Errorf("url=%s, want=%s", got, want)
			}

			entry, err := dataStore.GetEntryMetadata(picoshare.MustCreateEntryID(response.ID))
			if err != nil {
				t.Fatalf("failed to get entry %s from data store: %v", response.ID, err)
			}
			if got, want := entry.Filename.String(), "upload.txt"; got != want {
				t.Errorf("filename=%s, want=%s", got, want)
			}
			if got, want := entry.Expires, mustParseExpirationTime(tt.expiresExpected); got != want {
				t.Errorf("expires=%v, want=%v", got, want)
			}
		})
	}
}

func TestAPIKeyWithoutPermissionsCannotModifyFiles(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		method      string
		route       string
		status      int
	}{
		{
			explanation: "deleting through the session API with an API key should be rejected",
			method:      http.MethodDelete,
			route:       "/api/entry/hR87apiUCj",
			status:      http.StatusUnauthorized,
		},
		{
			explanation: "editing through the session API with an API key should be rejected",
			method:      http.MethodPut,
			route:       "/api/entry/hR87apiUCj",
			status:      http.StatusUnauthorized,
		},
		{
			explanation: "deleting through the API key routes without the delete permission should be rejected",
			method:      http.MethodDelete,
			route:       "/api/v1/files/hR87apiUCj",
			status:      http.StatusForbidden,
		},
		{
			explanation: "editing through the API key routes without the edit permission should be rejected",
			method:      http.MethodPatch,
			route:       "/api/v1/files/hR87apiUCj",
			status:      http.StatusForbidden,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			apiKey := "ps_0123456789abcdefghijABCDEFGHIJ0123456789"
			if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
				Hash:    mustCreateAPIKey(t, apiKey).Hash(),
				Created: mustParseTime("2025-01-01T00:00:00Z"),
			}); err != nil {
				t.Fatalf("failed to store API key: %v", err)
			}
			if err := dataStore.InsertEntry(strings.NewReader("dummy data"), picoshare.UploadMetadata{
				ID:       picoshare.MustCreateEntryID("hR87apiUCj"),
				Filename: "dummy.txt",
				Uploaded: mustParseTime("2025-01-01T00:00:00Z"),
				Expires:  picoshare.NeverExpire,
				Size:     mustParseFileSize(len("dummy data")),
			}); err != nil {
				t.Fatalf("failed to insert entry: %v", err)
			}
			s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

			req := httptest.NewRequest(tt.method, tt.route, strings.NewReader(`{"filename":"renamed.txt"}`))
			req.Header.Set("Authorization", "Bearer "+apiKey)
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if got, want := rec.Code, tt.status; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}

			entry, err := dataStore.GetEntryMetadata(picoshare.MustCreateEntryID("hR87apiUCj"))
			if err != nil {
				t.Fatalf("entry should still exist: %v", err)
			}
			if got, want := entry.Filename.String(), "dummy.txt"; got != want {
				t.Errorf("filename=%s, want=%s", got, want)
			}
		})
	}
}

func TestAPIKeyPostRegeneratesKey(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2025-01-01T00:00:00Z")
	sessionServer := handlers.New(mockAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, func() time.Time { return now })
	apiServer := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, func() time.Time { return now })

	// A browser session alone does not grant access to the API key routes.
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
		rec := httptest.NewRecorder()
		sessionServer.Router().ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusUnauthorized; got != want {
			t.Fatalf("status=%d, want=%d", got, want)
		}
	}

	var firstKey string
	{
		req := httptest.NewRequest(http.MethodPost, "/api/settings/api-key", nil)
		rec := httptest.NewRecorder()
		sessionServer.Router().ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusOK; got != want {
			t.Fatalf("status=%d, want=%d", got, want)
		}
		var response struct {
			APIKey  string `json:"apiKey"`
			Created string `json:"created"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("response is not valid JSON: %s", rec.Body.String())
		}
		if got, want := response.Created, "2025-01-01T00:00:00Z"; got != want {
			t.Errorf("created=%s, want=%s", got, want)
		}
		firstKey = response.APIKey
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
		req.Header.Set("Authorization", "Bearer "+firstKey)
		rec := httptest.NewRecorder()
		apiServer.Router().ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusOK; got != want {
			t.Fatalf("status=%d, want=%d", got, want)
		}
	}

	now = mustParseTime("2025-02-01T00:00:00Z")
	var secondKey string
	{
		req := httptest.NewRequest(http.MethodPost, "/api/settings/api-key", nil)
		rec := httptest.NewRecorder()
		sessionServer.Router().ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusOK; got != want {
			t.Fatalf("status=%d, want=%d", got, want)
		}
		var response struct {
			APIKey string `json:"apiKey"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("response is not valid JSON: %s", rec.Body.String())
		}
		secondKey = response.APIKey
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
		req.Header.Set("Authorization", "Bearer "+firstKey)
		rec := httptest.NewRecorder()
		apiServer.Router().ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusUnauthorized; got != want {
			t.Errorf("old key status=%d, want=%d", got, want)
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
		req.Header.Set("Authorization", "Bearer "+secondKey)
		rec := httptest.NewRecorder()
		apiServer.Router().ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusOK; got != want {
			t.Errorf("new key status=%d, want=%d", got, want)
		}
	}

	record, err := dataStore.ReadAPIKey()
	if err != nil {
		t.Fatalf("failed to read API key: %v", err)
	}
	if got, want := record.Created, mustParseTime("2025-02-01T00:00:00Z"); !got.Equal(want) {
		t.Errorf("created=%v, want=%v", got, want)
	}
}

func TestAPIKeyPostRequiresSession(t *testing.T) {
	dataStore := test_sqlite.New(t)
	s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/api-key", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestAPIFilePatch(t *testing.T) {
	for _, tt := range []struct {
		explanation                 string
		allowEdit                   bool
		route                       string
		payload                     string
		status                      int
		filenameExpected            string
		noteExpected                string
		expiresExpected             picoshare.ExpirationTime
		passphraseProtectedExpected bool
	}{
		{
			explanation:                 "renaming a file should change only its filename",
			allowEdit:                   true,
			route:                       "/api/v1/files/hR87apiUCj",
			payload:                     `{"filename": "renamed.txt"}`,
			status:                      http.StatusOK,
			filenameExpected:            "renamed.txt",
			noteExpected:                "original note",
			expiresExpected:             mustParseExpirationTime("2030-01-01T00:00:00Z"),
			passphraseProtectedExpected: true,
		},
		{
			explanation:                 "an empty note and expiration should remove the note and make the file never expire",
			allowEdit:                   true,
			route:                       "/api/v1/files/hR87apiUCj",
			payload:                     `{"note": "", "expiration": ""}`,
			status:                      http.StatusOK,
			filenameExpected:            "original.txt",
			noteExpected:                "",
			expiresExpected:             picoshare.NeverExpire,
			passphraseProtectedExpected: true,
		},
		{
			explanation:                 "an empty download passphrase should remove the passphrase",
			allowEdit:                   true,
			route:                       "/api/v1/files/hR87apiUCj",
			payload:                     `{"downloadPassphrase": ""}`,
			status:                      http.StatusOK,
			filenameExpected:            "original.txt",
			noteExpected:                "original note",
			expiresExpected:             mustParseExpirationTime("2030-01-01T00:00:00Z"),
			passphraseProtectedExpected: false,
		},
		{
			explanation:                 "an invalid filename should be rejected without changing the file",
			allowEdit:                   true,
			route:                       "/api/v1/files/hR87apiUCj",
			payload:                     `{"filename": "../etc/passwd"}`,
			status:                      http.StatusBadRequest,
			filenameExpected:            "original.txt",
			noteExpected:                "original note",
			expiresExpected:             mustParseExpirationTime("2030-01-01T00:00:00Z"),
			passphraseProtectedExpected: true,
		},
		{
			explanation:                 "editing a file that doesn't exist should return not found",
			allowEdit:                   true,
			route:                       "/api/v1/files/doesNotExt",
			payload:                     `{"filename": "renamed.txt"}`,
			status:                      http.StatusNotFound,
			filenameExpected:            "original.txt",
			noteExpected:                "original note",
			expiresExpected:             mustParseExpirationTime("2030-01-01T00:00:00Z"),
			passphraseProtectedExpected: true,
		},
		{
			explanation:                 "a key without the edit permission should be rejected without changing the file",
			allowEdit:                   false,
			route:                       "/api/v1/files/hR87apiUCj",
			payload:                     `{"filename": "renamed.txt"}`,
			status:                      http.StatusForbidden,
			filenameExpected:            "original.txt",
			noteExpected:                "original note",
			expiresExpected:             mustParseExpirationTime("2030-01-01T00:00:00Z"),
			passphraseProtectedExpected: true,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			apiKey := "ps_0123456789abcdefghijABCDEFGHIJ0123456789"
			if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
				Hash:        mustCreateAPIKey(t, apiKey).Hash(),
				Created:     mustParseTime("2025-01-01T00:00:00Z"),
				Permissions: picoshare.APIKeyPermissions{AllowEdit: tt.allowEdit},
			}); err != nil {
				t.Fatalf("failed to store API key: %v", err)
			}
			note := "original note"
			if err := dataStore.InsertEntry(strings.NewReader("dummy data"), picoshare.UploadMetadata{
				ID:                 picoshare.MustCreateEntryID("hR87apiUCj"),
				Filename:           "original.txt",
				Note:               picoshare.FileNote{Value: &note},
				Uploaded:           mustParseTime("2025-01-01T00:00:00Z"),
				Expires:            mustParseExpirationTime("2030-01-01T00:00:00Z"),
				Size:               mustParseFileSize(len("dummy data")),
				DownloadPassphrase: mustCreateDownloadPassphrase(t, "secret passphrase"),
			}); err != nil {
				t.Fatalf("failed to insert entry: %v", err)
			}
			now := mustParseTime("2025-01-01T00:00:00Z")
			s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, func() time.Time { return now })

			req := httptest.NewRequest(http.MethodPatch, tt.route, strings.NewReader(tt.payload))
			req.Header.Set("Authorization", "Bearer "+apiKey)
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if got, want := rec.Code, tt.status; got != want {
				t.Fatalf("status=%d, want=%d", got, want)
			}

			entry, err := dataStore.GetEntryMetadata(picoshare.MustCreateEntryID("hR87apiUCj"))
			if err != nil {
				t.Fatalf("failed to get entry: %v", err)
			}
			if got, want := entry.Filename.String(), tt.filenameExpected; got != want {
				t.Errorf("filename=%s, want=%s", got, want)
			}
			gotNote := ""
			if entry.Note.Value != nil {
				gotNote = *entry.Note.Value
			}
			if got, want := gotNote, tt.noteExpected; got != want {
				t.Errorf("note=%q, want=%q", got, want)
			}
			if got, want := entry.Expires, tt.expiresExpected; got != want {
				t.Errorf("expires=%v, want=%v", got, want)
			}
			if got, want := !entry.DownloadPassphrase.Empty(), tt.passphraseProtectedExpected; got != want {
				t.Errorf("passphrase protected=%v, want=%v", got, want)
			}
		})
	}
}

func TestAPIFileDelete(t *testing.T) {
	for _, tt := range []struct {
		explanation         string
		allowDelete         bool
		route               string
		status              int
		entryExistsExpected bool
	}{
		{
			explanation:         "a key with the delete permission should delete the file",
			allowDelete:         true,
			route:               "/api/v1/files/hR87apiUCj",
			status:              http.StatusOK,
			entryExistsExpected: false,
		},
		{
			explanation:         "deleting a file that doesn't exist should return not found",
			allowDelete:         true,
			route:               "/api/v1/files/doesNotExt",
			status:              http.StatusNotFound,
			entryExistsExpected: true,
		},
		{
			explanation:         "a key without the delete permission should be rejected and keep the file",
			allowDelete:         false,
			route:               "/api/v1/files/hR87apiUCj",
			status:              http.StatusForbidden,
			entryExistsExpected: true,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			apiKey := "ps_0123456789abcdefghijABCDEFGHIJ0123456789"
			if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
				Hash:        mustCreateAPIKey(t, apiKey).Hash(),
				Created:     mustParseTime("2025-01-01T00:00:00Z"),
				Permissions: picoshare.APIKeyPermissions{AllowDelete: tt.allowDelete},
			}); err != nil {
				t.Fatalf("failed to store API key: %v", err)
			}
			if err := dataStore.InsertEntry(strings.NewReader("dummy data"), picoshare.UploadMetadata{
				ID:       picoshare.MustCreateEntryID("hR87apiUCj"),
				Filename: "dummy.txt",
				Uploaded: mustParseTime("2025-01-01T00:00:00Z"),
				Expires:  picoshare.NeverExpire,
				Size:     mustParseFileSize(len("dummy data")),
			}); err != nil {
				t.Fatalf("failed to insert entry: %v", err)
			}
			s := handlers.New(unauthenticatedAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

			req := httptest.NewRequest(http.MethodDelete, tt.route, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if got, want := rec.Code, tt.status; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}

			_, err := dataStore.GetEntryMetadata(picoshare.MustCreateEntryID("hR87apiUCj"))
			if got, want := err == nil, tt.entryExistsExpected; got != want {
				t.Errorf("entry exists=%v, want=%v (err=%v)", got, want, err)
			}
		})
	}
}

func TestAPIKeyPermissionsPut(t *testing.T) {
	for _, tt := range []struct {
		explanation         string
		apiKeyInStore       bool
		payload             string
		status              int
		permissionsExpected picoshare.APIKeyPermissions
	}{
		{
			explanation:   "enabling both permissions should save them",
			apiKeyInStore: true,
			payload:       `{"allowEdit": true, "allowDelete": true}`,
			status:        http.StatusOK,
			permissionsExpected: picoshare.APIKeyPermissions{
				AllowEdit:   true,
				AllowDelete: true,
			},
		},
		{
			explanation:   "enabling only editing should leave deleting disabled",
			apiKeyInStore: true,
			payload:       `{"allowEdit": true, "allowDelete": false}`,
			status:        http.StatusOK,
			permissionsExpected: picoshare.APIKeyPermissions{
				AllowEdit:   true,
				AllowDelete: false,
			},
		},
		{
			explanation:         "a request missing a permission should be rejected",
			apiKeyInStore:       true,
			payload:             `{"allowEdit": true}`,
			status:              http.StatusBadRequest,
			permissionsExpected: picoshare.APIKeyPermissions{},
		},
		{
			explanation:         "setting permissions before generating a key should return not found",
			apiKeyInStore:       false,
			payload:             `{"allowEdit": true, "allowDelete": true}`,
			status:              http.StatusNotFound,
			permissionsExpected: picoshare.APIKeyPermissions{},
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			if tt.apiKeyInStore {
				if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
					Hash:    mustCreateAPIKey(t, "ps_0123456789abcdefghijABCDEFGHIJ0123456789").Hash(),
					Created: mustParseTime("2025-01-01T00:00:00Z"),
				}); err != nil {
					t.Fatalf("failed to store API key: %v", err)
				}
			}
			s := handlers.New(mockAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

			req := httptest.NewRequest(http.MethodPut, "/api/settings/api-key/permissions", strings.NewReader(tt.payload))
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if got, want := rec.Code, tt.status; got != want {
				t.Fatalf("status=%d, want=%d", got, want)
			}
			if !tt.apiKeyInStore {
				return
			}

			record, err := dataStore.ReadAPIKey()
			if err != nil {
				t.Fatalf("failed to read API key: %v", err)
			}
			if got, want := record.Permissions, tt.permissionsExpected; got != want {
				t.Errorf("permissions=%+v, want=%+v", got, want)
			}
		})
	}
}

func TestAPIKeyRegenerationKeepsPermissions(t *testing.T) {
	dataStore := test_sqlite.New(t)
	if err := dataStore.UpdateAPIKey(picoshare.APIKeyRecord{
		Hash:    mustCreateAPIKey(t, "ps_0123456789abcdefghijABCDEFGHIJ0123456789").Hash(),
		Created: mustParseTime("2025-01-01T00:00:00Z"),
		Permissions: picoshare.APIKeyPermissions{
			AllowEdit:   true,
			AllowDelete: true,
		},
	}); err != nil {
		t.Fatalf("failed to store API key: %v", err)
	}
	s := handlers.New(mockAuthenticator{}, &dataStore, nilSpaceCheckFunc, nilGarbageCollector, time.Now)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/api-key", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}

	record, err := dataStore.ReadAPIKey()
	if err != nil {
		t.Fatalf("failed to read API key: %v", err)
	}
	if record.Hash.Equal(mustCreateAPIKey(t, "ps_0123456789abcdefghijABCDEFGHIJ0123456789").Hash()) {
		t.Errorf("regeneration should replace the API key")
	}
	if got, want := record.Permissions, (picoshare.APIKeyPermissions{AllowEdit: true, AllowDelete: true}); got != want {
		t.Errorf("permissions=%+v, want=%+v", got, want)
	}
}
