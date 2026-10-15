package model

import "time"

type Session struct {
	Account string
	Bucket  string
	Prefix  string
}

type Entry struct {
	Key        string
	Name       string
	IsDir      bool
	Size       int64
	Modified   time.Time
	ETag       string
	Count      int // objects under a folder, once its stats are known
	StatsKnown bool
	StatsError bool
}

type SortField int

const (
	SortName SortField = iota
	SortSize
	SortModified
)

type SortState struct {
	Field SortField
	Desc  bool
}

type PublicDomain struct {
	Domain  string
	Managed bool
}
