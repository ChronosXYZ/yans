package sqlite

import (
	"embed"
	"github.com/ChronosX88/yans/internal/config"
	"github.com/ChronosX88/yans/internal/models"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

type SQLiteBackend struct {
	db *sqlx.DB
}

func NewSQLiteBackend(cfg config.SQLiteBackendConfig) (*SQLiteBackend, error) {
	db, err := sqlx.Open("sqlite3", cfg.Path)
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
