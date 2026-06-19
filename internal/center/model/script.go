package model

import (
	"time"
)

// Script represents a user-uploaded script stored in the database.
type Script struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ScriptID    string    `gorm:"uniqueIndex;size:64;not null" json:"script_id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Description string    `gorm:"size:1024" json:"description"`
	Language    string    `gorm:"size:32;not null;default:python" json:"language"`
	Source      string    `gorm:"type:text;not null" json:"source"`
	Params      string    `gorm:"type:text" json:"params"` // JSON string of []Param
	Timeout     int       `gorm:"default:30" json:"timeout"`
	CreatedBy   string    `gorm:"size:64" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ScriptResult stores the execution result of a script run.
type ScriptResult struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ScriptID    string    `gorm:"index;size:64;not null" json:"script_id"`
	NodeID      string    `gorm:"index;size:64" json:"node_id"`
	Stdout      string    `gorm:"type:text" json:"stdout"`
	Stderr      string    `gorm:"type:text" json:"stderr"`
	ExitCode    int       `json:"exit_code"`
	Duration    int64     `json:"duration_ms"`
	Status      string    `gorm:"size:32;default:pending" json:"status"` // pending/running/completed/failed
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	CreatedAt   time.Time `json:"created_at"`
}
