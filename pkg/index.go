package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
)

func index(exifTool *exiftool.Exiftool, path string, picture *Picture) error {
	dir := filepath.Dir(path)
	picture.Dir = dir

	if dir[0] == filepath.Separator {
		dir = dir[1:]
	}
	dirSplit := strings.Split(dir, string(filepath.Separator))
	picture.Dir1 = dirSplit[0]
	if len(dirSplit) > 1 {
		picture.Dir2 = dirSplit[1]
	}
	if len(dirSplit) > 2 {
		picture.Dir3 = dirSplit[2]
	}

	// extract EXIF metadata
	fileInfos := exifTool.ExtractMetadata(path)
	if len(fileInfos) == 0 {
		return fmt.Errorf("cannot extract EXIF data from %s", path)
	}

	fileInfo := fileInfos[0]
	if fileInfo.Err != nil {
		return fileInfo.Err
	}

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
	if tags, err := fileInfo.GetStrings("Keywords"); err == nil {
		picture.Tags = ";" + strings.Join(tags, ";") + ";"
	}

	/*for k, v := range fileInfo.Fields {
		fmt.Printf("%v = %v\n", k, v)
	}*/

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

		if !forceReindex {
			result := db.Limit(1).Find(&picture, "path = ?", path)
			if result.RowsAffected == 1 {
				if stat.ModTime().Before(picture.UpdatedAt) {
					if config.Verbose {
						fmt.Printf("skipping %s [%.0f%%]\n", path, float64(i+1)/float64(len(paths))*100)
					}
					continue
				}
			}
		}

		fmt.Printf("indexing %s [%.0f%%]\n", path, float64(i+1)/float64(len(paths))*100)
		index(exifTool, path, &picture)

		result := db.Save(&picture)
		if result.Error != nil {
			return result.Error
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
			if config.Verbose {
				fmt.Printf("deleting %s from db (picture got deleted on disk)\n", picture.Path)
			}
			result := db.Delete(picture)
			if result.Error != nil {
				return result.Error
			}
		}
	}

	return nil
}
