package main

import (
	"fmt"
	"net/url"

	"github.com/kris-nova/photoprism-client-go"
	"github.com/kris-nova/photoprism-client-go/api/v1"
)

type PhotoprismPicture struct {
	Album          string
	Path           string
	PhotoprismPath string
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

		queryPath := picture.PhotoprismPath
		if queryPath == "" {
			queryPath = picture.Path
		}

		photos, err := client.V1().GetPhotos(&api.PhotoOptions{
			Q:     url.QueryEscape(fmt.Sprintf("filename:\"*%s*\"", queryPath)),
			Count: 1,
		})
		if err != nil {
			return err
		}

		if len(photos) == 0 {
			fmt.Printf("WARNING: %s not found in PhotoPrism, did you forget to index?\n", queryPath)
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
			fmt.Printf("deleting %s/%s from %s\n", picture.PhotoPath, picture.PhotoName, albumTitle)
			deletePictures = append(deletePictures, picture.PhotoUID)
		} else {
			fmt.Printf("WARNING: %s/%s is in PhotoPrism album %s but should not be there (use --delete to delete extra pictures)\n", picture.PhotoPath, picture.PhotoName, albumTitle)
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
