package models

import (
	"database/sql"
	"net/textproto"
	"time"
)

type Article struct {
	ID        int                  `db:"id"`
	CreatedAt time.Time            `db:"created_at"`
	HeaderRaw string               `db:"header"`
	Header    textproto.MIMEHeader `db:"-"`
	Body      string               `db:"body"`
	Thread    sql.NullString       `db:"thread"`
}

//func ParseArticle(lines []string) (Article, error) {
//	article := Article{}
//	headerBlock := true
//	for _, v := range lines {
//		if v == "" {
//			headerBlock = false
//		}
//
//		if headerBlock {
//			kv := strings.Split(v, ":")
//			if len(kv) < 2 {
//				return Article{}, fmt.Errorf("invalid header format")
//			}
//
//			kv[0] = strings.TrimSpace(kv[0])
//			kv[1] = strings.TrimSpace(kv[1])
//
//			if !protocol.IsMessageHeaderAllowed(kv[0]) {
//				return Article{}, fmt.Errorf("invalid header element")
//			}
//			if kv[1] == "" {
//				return Article{}, fmt.Errorf("header value should not be empty")
//			}
//
//			switch kv[0] {
//			case "Archive":
//				{
//					if kv[1] == "yes" {
//						article.Archive = true
//					} else {
//						article.Archive = false
//					}
//				}
//			case "Injection-Date":
//				{
//					date, err := mail.ParseDate(kv[1])
//					if err != nil {
//						return Article{}, err
//					}
//					article.InjectionDate = date
//				}
//			case "Date":
//				{
//					date, err := mail.ParseDate(kv[1])
//					if err != nil {
//						return Article{}, err
//					}
//					article.Date = date
//				}
//			case "Expires":
//				{
//					date, err := mail.ParseDate(kv[1])
//					if err != nil {
//						return Article{}, err
//					}
//					article.Expires = date
//				}
//			}
//
//		} else {
//		}
//	}
//	return article, nil
//}
