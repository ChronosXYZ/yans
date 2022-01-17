package models

type Group struct {
	ID          int     `db:"id"`
	GroupName   string  `db:"group_name"`
	Description *string `db:"description"`
}
