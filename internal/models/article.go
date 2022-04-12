package models

import (
	"database/sql"
	"github.com/jhillyerd/enmime"
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
	Envelope      *enmime.Envelope     `db:"-"`
	ArticleNumber int                  `db:"-"`
	Attachments   []Attachment
}

type Attachment struct {
	ContentType string `db:"content_type"`
	FileName    string `db:"attachment_id"`
}
