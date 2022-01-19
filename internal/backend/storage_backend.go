package backend

import "github.com/ChronosX88/yans/internal/models"

const (
	SupportedBackendList = "sqlite"
)

type StorageBackend interface {
	ListGroups() ([]models.Group, error)
	GetArticlesCount(g models.Group) (int, error)
}
