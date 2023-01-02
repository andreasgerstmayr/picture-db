package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
	"gorm.io/gorm"
)

func index(exifTool *exiftool.Exiftool, db *gorm.DB, path string, picture *Picture) error {
	// extract path information
	jsonPath := strings.Split(path, string(filepath.Separator))
	if jsonPath[0] == "" {
		// skip first empty element in case of an absolute path
		jsonPath = jsonPath[1:]
	}
	jsonPathBytes, err := json.Marshal(jsonPath)
	if err != nil {
		return err
	}
	picture.JsonPath = string(jsonPathBytes)
	picture.Dir = filepath.Dir(path)

	// extract EXIF metadata
	fileInfos := exifTool.ExtractMetadata(path)
	if len(fileInfos) == 0 {
		return fmt.Errorf("cannot extract EXIF data from %s", path)
	}

	fileInfo := fileInfos[0]
	if fileInfo.Err != nil {
		return fileInfo.Err
	}

	// extract exif infos
	if dateTimeOriginal, err := fileInfo.GetInt("DateTimeOriginal"); err == nil {
		dt := time.Unix(dateTimeOriginal, 0)
		picture.DateTimeOriginal = &dt
	}
	if make, err := fileInfo.GetString("Make"); err == nil {
		picture.Make = &make
	}
	if model, err := fileInfo.GetString("Model"); err == nil {
		picture.Model = &model
	}
	if rating, err := fileInfo.GetInt("Rating"); err == nil {
		picture.Rating = &rating
	}

	/*for k, v := range fileInfo.Fields {
		fmt.Printf("%v = %v\n", k, v)
	}*/

	// create or update picture before adding tags due to FK constraints
	if picture.CreatedAt.IsZero() {
		result := db.Create(&picture)
		if result.Error != nil {
			return result.Error
		}
	} else {
		result := db.Save(&picture)
		if result.Error != nil {
			return result.Error
		}
	}

	// extract tags
	tags, err := fileInfo.GetStrings("Keywords")
	pictureTags := []PictureTag{}
	if err == nil {
		for _, tag := range tags {
			pictureTags = append(pictureTags, PictureTag{Tag: tag})
		}
	}

	// add or delete tags
	// workaround for https://github.com/go-gorm/gorm/issues/5899
	if len(tags) > 0 {
		err = db.Model(&picture).Association("Tags").Append(pictureTags)
		if err != nil {
			return err
		}

		err = db.Delete(&PictureTag{}, "path = ? AND tag NOT IN ?", picture.Path, tags).Error
		if err != nil {
			return err
		}
	} else {
		err = db.Delete(&PictureTag{}, "path = ?", picture.Path).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func scan(config *Config, searchPath string, forceReindex bool) error {
	db, err := openDatabase(config.DBPath)
	if err != nil {
		return err
	}

	// init exiftool
	exifTool, err := exiftool.NewExiftool(exiftool.DateFormant("%s"))
	if err != nil {
		return err
	}
	defer exifTool.Close()

	paths := map[string]bool{}
	err = filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".heic", ".jpg":
			paths[path] = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	// index pictures from disk
	i := 0
	for path := range paths {
		i++

		stat, err := os.Stat(path)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}

		picture := Picture{
			Path: path,
		}

		result := db.Limit(1).Find(&picture)
		if result.RowsAffected == 1 {
			if !forceReindex && stat.ModTime().Before(picture.UpdatedAt) {
				if config.Verbose {
					fmt.Printf("skipping %s [%.0f%%]\n", path, float64(i)/float64(len(paths))*100)
				}
				continue
			}
		}

		fmt.Printf("indexing %s [%.0f%%]\n", path, float64(i)/float64(len(paths))*100)
		err = index(exifTool, db, path, &picture)
		if err != nil {
			log.Printf("error indexing %s: %v", path, err)
			continue
		}
	}

	// purge pictures deleted on disk from DB
	var pictures []Picture
	result := db.Select("path").Where("dir||'/' LIKE ?", searchPath+"/%").Find(&pictures)
	if result.Error != nil {
		return result.Error
	}

	for _, picture := range pictures {
		_, ok := paths[picture.Path]
		if !ok {
			fmt.Printf("removing %s from index (file got moved or deleted on disk)\n", picture.Path)
			result := db.Delete(picture)
			if result.Error != nil {
				return result.Error
			}
		}
	}

	return nil
}
