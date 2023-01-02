package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/spf13/cobra"
)

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
			err := sql(&config, args[0])
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
