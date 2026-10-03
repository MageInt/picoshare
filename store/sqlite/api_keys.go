package sqlite

import (
	"database/sql"
	"errors"
	"log"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

// We only store one API key at a time, so we use a fixed row ID.
const apiKeyRowID = 1

func (s Store) ReadAPIKey() (picoshare.APIKeyRecord, error) {
	var hashRaw []byte
	var createdRaw string
	if err := s.db.QueryRow(`
	SELECT
		key_hash,
		created
	FROM
		api_keys
	WHERE
		id = :row_id`, sql.Named("row_id", apiKeyRowID)).Scan(&hashRaw, &createdRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return picoshare.APIKeyRecord{}, store.APIKeyNotFoundError{}
		}
		return picoshare.APIKeyRecord{}, err
	}

	hash, err := picoshare.APIKeyHashFromBytes(hashRaw)
	if err != nil {
		return picoshare.APIKeyRecord{}, err
	}

	created, err := parseDatetime(createdRaw)
	if err != nil {
		return picoshare.APIKeyRecord{}, err
	}

	return picoshare.APIKeyRecord{
		Hash:    hash,
		Created: created,
	}, nil
}

func (s Store) UpdateAPIKey(record picoshare.APIKeyRecord) error {
	log.Printf("saving new API key created at %s", formatTime(record.Created))
	if _, err := s.db.Exec(`
	INSERT INTO api_keys (
		id,
		key_hash,
		created
	) VALUES (
		:row_id,
		:key_hash,
		:created
	)
	ON CONFLICT(id) DO UPDATE SET
		key_hash = excluded.key_hash,
		created = excluded.created`,
		sql.Named("row_id", apiKeyRowID),
		sql.Named("key_hash", record.Hash.Bytes()),
		sql.Named("created", formatTime(record.Created))); err != nil {
		return err
	}

	return nil
}
