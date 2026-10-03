-- API keys can upload and list files by default. These flags additionally
-- allow the key to edit or delete files.
ALTER TABLE api_keys ADD COLUMN allow_edit INTEGER NOT NULL DEFAULT 0 CHECK (
    allow_edit IN (0, 1)
);

ALTER TABLE api_keys ADD COLUMN allow_delete INTEGER NOT NULL DEFAULT 0 CHECK (
    allow_delete IN (0, 1)
);
