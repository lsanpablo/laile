package sql

import "github.com/jackc/pgx/v5/pgtype"

var (
	NilText = &pgtype.Text{
		String: "",
		Valid:  false,
	}
)
