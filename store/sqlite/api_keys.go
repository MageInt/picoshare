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
	var allowEdit bool
	var allowDelete bool
	if err := s.db.QueryRow(`
	SELECT
		key_hash,
		created,
		allow_edit,
		allow_delete
	FROM
		api_keys
	WHERE
		id = :row_id`, sql.Named("row_id", apiKeyRowID)).Scan(&hashRaw, &createdRaw, &allowEdit, &allowDelete); err != nil {
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
		Permissions: picoshare.APIKeyPermissions{
			AllowEdit:   allowEdit,
			AllowDelete: allowDelete,
		},
	}, nil
}

func (s Store) UpdateAPIKey(record picoshare.APIKeyRecord) error {
	log.Printf("saving API key created at %s with permissions %+v",
		formatTime(record.Created), record.Permissions)
	if _, err := s.db.Exec(`
	INSERT INTO api_keys (
		id,
		key_hash,
		created,
		allow_edit,
		allow_delete
	) VALUES (
		:row_id,
		:key_hash,
		:created,
		:allow_edit,
		:allow_delete
	)
	ON CONFLICT(id) DO UPDATE SET
		key_hash = excluded.key_hash,
		created = excluded.created,
		allow_edit = excluded.allow_edit,
		allow_delete = excluded.allow_delete`,
		sql.Named("row_id", apiKeyRowID),
		sql.Named("key_hash", record.Hash.Bytes()),
		sql.Named("created", formatTime(record.Created)),
		sql.Named("allow_edit", record.Permissions.AllowEdit),
		sql.Named("allow_delete", record.Permissions.AllowDelete)); err != nil {
		return err
	}

	return nil
}
