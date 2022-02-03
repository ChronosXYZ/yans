package models

import "time"

type Group struct {
	ID          int       `db:"id"`
	GroupName   string    `db:"group_name"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}
