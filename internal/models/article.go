package models

import (
	"database/sql"
	"net/textproto"
	"time"
)

type Article struct {
	ID        int            `db:"id"`
	CreatedAt time.Time      `db:"created_at"`
	HeaderRaw string         `db:"header"`
	Body      string         `db:"body"`
	Thread    sql.NullString `db:"thread"`

	Header        textproto.MIMEHeader `db:"-"`
	ArticleNumber int                  `db:"-"`
}
