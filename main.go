package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/kris-nova/photoprism-client-go"
	"github.com/kris-nova/photoprism-client-go/api/v1"
	"github.com/spf13/cobra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func openDatabase(dbPath string) (*gorm.DB, error) {
	// create or open sqlite db
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Picture{})
	return db, nil
}

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

type PhotoprismPicture struct {
	Album string
	Path  string
}

func syncPhotoprismAlbum(config *Config, client *photoprism.Client, photoprismAlbums []api.Album, albumTitle string, pictures []*PhotoprismPicture) error {
	photoprismAlbumPictures := map[string]*api.Photo{}

	// find album in existing photoprism albums
	var photoprismAlbum api.Album
	for _, a := range photoprismAlbums {
		if a.AlbumTitle == albumTitle {
			photoprismAlbum = a
			break
		}
	}

	// create new album or fetch existing pictures
	if photoprismAlbum.AlbumUID == "" {
		fmt.Printf("creating album %s\n", albumTitle)
		newAlbum, err := client.V1().CreateAlbum(api.Album{AlbumTitle: albumTitle})
		if err != nil {
			return err
		}

		photoprismAlbum = newAlbum
	} else {
		if config.Verbose {
			fmt.Printf("album %s exists already\n", albumTitle)
		}

		// fetch picture UIDs of existing album
		photos, err := client.V1().GetPhotos(&api.PhotoOptions{
			AlbumUID: photoprismAlbum.AlbumUID,
			Count:    1000,
		})
		if err != nil {
			return err
		}

		for _, photo := range photos {
			photo := photo
			photoprismAlbumPictures[photo.PhotoUID] = &photo
		}
	}

	// compile list of new picture UIDs for this album
	newPictures := []string{}
	for i, picture := range pictures {
		if config.Verbose {
			fmt.Printf("searching file %s in PhotoPrism [%.0f%%]\n", picture.Path, float64(i+1)/float64(len(pictures))*100)
		}
		photos, err := client.V1().GetPhotos(&api.PhotoOptions{
			Q:     url.QueryEscape(fmt.Sprintf("filename:\"*%s*\"", picture.Path)),
			Count: 1,
		})
		if err != nil {
			return err
		}

		if len(photos) == 0 {
			fmt.Printf("WARNING: picture %s not found in PhotoPrism, did you forget to index?\n", picture.Path)
			continue
		}
		photoUID := photos[0].PhotoUID

		_, ok := photoprismAlbumPictures[photoUID]
		if ok {
			if config.Verbose {
				fmt.Printf("skipping %s (already contained in %s)\n", picture.Path, albumTitle)
			}
			delete(photoprismAlbumPictures, photoUID)
		} else {
			fmt.Printf("adding %s to %s\n", picture.Path, albumTitle)
			newPictures = append(newPictures, photoUID)
		}
	}

	// add new pictures to album
	if len(newPictures) > 0 {
		err := client.V1().AddPhotosToAlbum(photoprismAlbum.AlbumUID, newPictures)
		if err != nil {
			return err
		}
	}

	// compile list of extra pictures of album to be deleted
	deletePictures := []string{}
	for _, picture := range photoprismAlbumPictures {
		if config.PhotoPrism.DeleteFromAlbum {
			fmt.Printf("deleting %s from %s\n", picture.PhotoName, albumTitle)
			deletePictures = append(deletePictures, picture.PhotoUID)
		} else {
			fmt.Printf("WARNING: %s is in PhotoPrism album %s but should not be there (use --delete to delete extra pictures)\n", picture.PhotoName, albumTitle)
		}
	}

	// delete extra pictures of album
	if len(deletePictures) > 0 {
		err := client.V1().DeletePhotosFromAlbum(photoprismAlbum.AlbumUID, deletePictures)
		if err != nil {
			return err
		}
	}

	return nil
}

func syncPhotoprismAlbums(config *Config, sql string) error {
	// open database
	db, err := openDatabase(config.DBPath)
	if err != nil {
		return err
	}

	// execute SQL command
	var pictures []*PhotoprismPicture
	result := db.Raw(sql).Scan(&pictures)
	if result.Error != nil {
		return result.Error
	}

	// create photoprism session
	client := photoprism.New(config.PhotoPrism.URL)
	err = client.Auth(photoprism.NewClientAuthLogin(config.PhotoPrism.User, config.PhotoPrism.Pass))
	if err != nil {
		return err
	}

	// group by album
	picturesByAlbum := map[string][]*PhotoprismPicture{}
	for _, picture := range pictures {
		album, ok := picturesByAlbum[picture.Album]
		if ok {
			picturesByAlbum[picture.Album] = append(album, picture)
		} else {
			picturesByAlbum[picture.Album] = []*PhotoprismPicture{picture}
		}
	}

	// fetch existing albums
	photoprismAlbums, err := client.V1().GetAlbums(nil)
	if err != nil {
		return err
	}

	// sync albums
	for album, pictures := range picturesByAlbum {
		err = syncPhotoprismAlbum(config, client, photoprismAlbums, album, pictures)
		if err != nil {
			return err
		}
	}

	return nil
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

func main() {
	var configFile string
	var config Config

	rootCmd := &cobra.Command{
		Use: "picture-db",
	}
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file")
	rootCmd.PersistentFlags().BoolVarP(&config.Verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&config.DBPath, "dbPath", "db.sqlite", "sqlite3 database")

	var forceReindex bool
	indexCmd := &cobra.Command{
		Use:   "index dir1 [dir2...]",
		Short: "index pictures of directories",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				args = append(args, ".")
			}

			for _, arg := range args {
				err := scan(&config, arg, forceReindex)
				if err != nil {
					log.Fatal(err)
					return
				}
			}
		},
	}
	indexCmd.Flags().BoolVarP(&forceReindex, "reindex", "r", false, "force reindex")
	rootCmd.AddCommand(indexCmd)

	photoprismCmd := &cobra.Command{
		Use:   "photoprism sql",
		Short: "create photoprism albums per the sql command",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if config.PhotoPrism.URL == "" {
				log.Fatal("missing PhotoPrism URL")
				return
			}
			if config.PhotoPrism.User == "" {
				log.Fatal("missing PhotoPrism username")
				return
			}
			if config.PhotoPrism.Pass == "" {
				log.Fatal("missing PhotoPrism password")
				return
			}

			err := syncPhotoprismAlbums(&config, args[0])
			if err != nil {
				log.Fatal(err)
				return
			}
		},
	}
	photoprismCmd.Flags().StringVar(&config.PhotoPrism.URL, "url", "", "PhotoPrism URL")
	photoprismCmd.Flags().StringVar(&config.PhotoPrism.User, "user", "", "username")
	photoprismCmd.Flags().StringVar(&config.PhotoPrism.Pass, "pass", "", "password")
	photoprismCmd.Flags().BoolVar(&config.PhotoPrism.DeleteFromAlbum, "delete", false, "delete extra pictures from album")
	rootCmd.AddCommand(photoprismCmd)

	sqlCmd := &cobra.Command{
		Use:   "sql command",
		Short: "run SQL commands in the picture database",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sqliteCmd := exec.Command("sqlite3", "-header", "-column", config.DBPath, args[0])
			sqliteCmd.Stdout = os.Stdout
			sqliteCmd.Stderr = os.Stderr

			err := sqliteCmd.Run()
			if err != nil {
				log.Fatal(err)
				return
			}
		},
	}
	rootCmd.AddCommand(sqlCmd)

	rootCmd.ParseFlags(os.Args[1:])
	if configFile != "" {
		buff, err := os.ReadFile(configFile)
		if err != nil {
			log.Fatalf("failed to read configFile %s: %s", configFile, err)
			return
		}

		err = json.Unmarshal(buff, &config)
		if err != nil {
			log.Fatalf("failed to parse configFile %s: %s", configFile, err)
			return
		}
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
