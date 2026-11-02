package pgcatalog

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgUUID wraps a uuid for COPY. uuid.UUID implements driver.Valuer, which pgx would encode
// through the slow database/sql path; pgtype.UUID takes the binary codec directly.
func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }
