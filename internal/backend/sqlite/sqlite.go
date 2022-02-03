package sqlite

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/ChronosX88/yans/internal/config"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/ChronosX88/yans/internal/utils"
	"github.com/dlclark/regexp2"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

type SQLiteBackend struct {
	db *sqlx.DB
}

func regexHelper(re, s string) (bool, error) {
	return regexp2.MustCompile(re, regexp2.None).MatchString(s)
}

func NewSQLiteBackend(cfg config.SQLiteBackendConfig) (*SQLiteBackend, error) {
	sql.Register("sqlite3_with_regexp",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				return conn.RegisterFunc("regexp", regexHelper, true)
			},
		})

	db, err := sqlx.Open("sqlite3_with_regexp", cfg.Path)
	if err != nil {
		return nil, err
	}
	goose.SetBaseFS(migrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}

	if err := goose.Up(db.DB, "migrations"); err != nil {
		return nil, err
	}

	return &SQLiteBackend{
		db: db,
	}, nil
}

func (sb *SQLiteBackend) ListGroups() ([]models.Group, error) {
	var groups []models.Group
	return groups, sb.db.Select(&groups, "SELECT * FROM groups")
}

func (sb *SQLiteBackend) ListGroupsByPattern(pattern string) ([]models.Group, error) {
	var groups []models.Group
	w, err := utils.ParseWildmat(pattern)
	if err != nil {
		return nil, err
	}
	r, err := w.ToRegex()
	if err != nil {
		return nil, err
	}
	return groups, sb.db.Select(&groups, "SELECT * FROM groups WHERE group_name REGEXP ?", r.String())
}

func (sb *SQLiteBackend) GetArticlesCount(g models.Group) (int, error) {
	var count int
	return count, sb.db.Get(&count, "SELECT COUNT(*) FROM articles_to_groups WHERE group_id = ?", g.ID)
}

func (sb *SQLiteBackend) GetGroupHighWaterMark(g models.Group) (int, error) {
	var waterMark int
	return waterMark, sb.db.Get(&waterMark, "SELECT article_id FROM articles_to_groups WHERE group_id = ? ORDER BY article_id DESC LIMIT 1", g.ID)
}

func (sb *SQLiteBackend) GetGroupLowWaterMark(g models.Group) (int, error) {
	var waterMark int
	return waterMark, sb.db.Get(&waterMark, "SELECT article_id FROM articles_to_groups WHERE group_id = ? ORDER BY article_id LIMIT 1", g.ID)
}

func (sb *SQLiteBackend) GetGroup(groupName string) (models.Group, error) {
	var group models.Group
	return group, sb.db.Get(&group, "SELECT * FROM groups WHERE group_name = ?", groupName)
}

func (sb *SQLiteBackend) GetNewGroupsSince(timestamp int64) ([]models.Group, error) {
	var groups []models.Group
	return groups, sb.db.Select(&groups, "SELECT * FROM groups WHERE created_at > datetime(?, 'unixepoch')", timestamp)
}

func (sb *SQLiteBackend) SaveArticle(a models.Article, groups []string) error {
	res, err := sb.db.Exec("INSERT INTO articles (header, body, thread) VALUES (?, ?, ?)", a.HeaderRaw, a.Body, a.Thread)
	articleID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	var groupIDs []int
	for _, v := range groups {
		v = strings.TrimSpace(v)
		g, err := sb.GetGroup(v)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("no such newsgroup")
			} else {
				return err
			}
		}
		groupIDs = append(groupIDs, g.ID)
	}

	for _, v := range groupIDs {
		_, err = sb.db.Exec("INSERT INTO articles_to_groups (article_id, group_id) VALUES (?, ?)", articleID, v)
		if err != nil {
			return err
		}
	}
	return err
}

func (sb *SQLiteBackend) GetArticle(messageID string) (models.Article, error) {
	var a models.Article
	if err := sb.db.Get(&a, "SELECT * FROM articles WHERE json_extract(articles.header, '$.Message-ID[0]') = ?", messageID); err != nil {
		return a, err
	}
	return a, json.Unmarshal([]byte(a.HeaderRaw), &a.Header)
}
