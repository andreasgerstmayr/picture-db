package main

import (
	"time"
)

type Picture struct {
	Path      string    `gorm:"primaryKey;not null"`
	Dir       string    `gorm:"not null"`
	Dir1      string    `gorm:"not null"`
	Dir2      string    `gorm:"not null"`
	Dir3      string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`

	Make             *string
	Model            *string
	DateTimeOriginal *time.Time
	Rating           *int64
	Tags             string `gorm:"not null"`
}

type PhotoPrismConfig struct {
	URL             string `json:"url"`
	User            string `json:"user"`
	Pass            string `json:"pass"`
	DeleteFromAlbum bool
}

type Config struct {
	Verbose    bool             `json:"verbose"`
	DBPath     string           `json:"dbPath"`
	PhotoPrism PhotoPrismConfig `json:"photoprism"`
}
