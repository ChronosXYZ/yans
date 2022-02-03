package backend

import "github.com/ChronosX88/yans/internal/models"

const (
	SupportedBackendList = "sqlite"
)

type StorageBackend interface {
	ListGroups() ([]models.Group, error)
	ListGroupsByPattern(pattern string) ([]models.Group, error)
	GetGroup(groupName string) (models.Group, error)
	GetNewGroupsSince(timestamp int64) ([]models.Group, error)
	GetArticlesCount(g models.Group) (int, error)
	GetGroupLowWaterMark(g models.Group) (int, error)
	GetGroupHighWaterMark(g models.Group) (int, error)
	SaveArticle(article models.Article, groups []string) error
	GetArticle(messageID string) (models.Article, error)
}
