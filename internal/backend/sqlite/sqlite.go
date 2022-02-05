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

func (sb *SQLiteBackend) GetArticlesCount(g *models.Group) (int, error) {
	var count int
	return count, sb.db.Get(&count, "SELECT COUNT(*) FROM articles_to_groups WHERE group_id = ?", g.ID)
}

func (sb *SQLiteBackend) GetGroupHighWaterMark(g *models.Group) (int, error) {
	var waterMark int
	return waterMark, sb.db.Get(&waterMark, "SELECT max(article_number) FROM articles_to_groups WHERE group_id = ?", g.ID)
}

func (sb *SQLiteBackend) GetGroupLowWaterMark(g *models.Group) (int, error) {
	var waterMark int
	return waterMark, sb.db.Get(&waterMark, "SELECT min(article_number) FROM articles_to_groups WHERE group_id = ?", g.ID)
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
		_, err = sb.db.Exec("INSERT INTO articles_to_groups (article_id, article_number, group_id) VALUES (?, (SELECT ifnull(max(article_number)+1, 1) FROM articles_to_groups), ?)", articleID, v)
		if err != nil {
			return err
		}
	}
	return err
}

func (sb *SQLiteBackend) GetArticle(messageID string) (models.Article, error) {
	var a models.Article
	if err := sb.db.Get(&a, "SELECT * FROM articles WHERE json_extract(articles.header, '$.Message-Id[0]') = ?", messageID); err != nil {
		return a, err
	}
	if err := sb.db.Get(&a.ArticleNumber, "SELECT article_number FROM articles_to_groups WHERE article_id = ?", a.ID); err != nil {
		return a, err
	}
	return a, json.Unmarshal([]byte(a.HeaderRaw), &a.Header)
}

func (sb *SQLiteBackend) GetArticleByNumber(g *models.Group, num int) (models.Article, error) {
	var a models.Article
	if err := sb.db.Get(&a, "SELECT articles.* FROM articles INNER JOIN articles_to_groups atg on atg.article_id = articles.id WHERE atg.article_number = ? AND atg.group_id = ?", num, g.ID); err != nil {
		return a, err
	}
	a.ArticleNumber = num
	return a, json.Unmarshal([]byte(a.HeaderRaw), &a.Header)
}

func (sb *SQLiteBackend) GetArticleNumbers(g *models.Group, low, high int64) ([]int64, error) {
	var numbers []int64

	if high == 0 && low == 0 {
		if err := sb.db.Select(&numbers, "SELECT article_number FROM articles_to_groups WHERE group_id = ?", g.ID); err != nil {
			return nil, err
		}
	} else if low == -1 && high != 0 {
		if err := sb.db.Select(&numbers, "SELECT article_number FROM articles_to_groups WHERE group_id = ? AND article_number = ?", g.ID, high); err != nil {
			return nil, err
		}
	} else if low != 0 && high == -1 {
		if err := sb.db.Select(&numbers, "SELECT article_number FROM articles_to_groups WHERE group_id = ? AND article_number > ?", g.ID, low); err != nil {
			return nil, err
		}
	} else if low == -1 && high == -1 {
		return nil, nil
	} else {
		if err := sb.db.Select(&numbers, "SELECT article_number FROM articles_to_groups WHERE group_id = ? AND article_number > ? AND article_number < ?", g.ID, low, high); err != nil {
			return nil, err
		}
	}

	return numbers, nil
}

func (sb *SQLiteBackend) GetNewArticlesSince(timestamp int64) ([]string, error) {
	var articleIds []string
	return articleIds, sb.db.Select(&articleIds, "SELECT json_extract(articles.header, '$.Message-Id[0]') FROM articles WHERE created_at > datetime(?, 'unixepoch')", timestamp)
}
