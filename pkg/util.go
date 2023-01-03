package main

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDatabase(dbPath string) (*gorm.DB, error) {
	// create or open sqlite db
	db, err := gorm.Open(sqlite.Open("file:"+dbPath+"?_foreign_keys=on&_journal_mode=WAL"), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Picture{}, &PictureTag{})
	return db, nil
}
