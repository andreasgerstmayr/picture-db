# picture-db
picture-db stores metadata of photo collections in a sqlite database.
This allows querying photo metadata via SQL.

## Usage
### Create or update database
```
picture-db index
picture-db index /path/to/photo/collection
```

### Run SQL queries
```
picture-db sql 'SELECT * FROM pictures'
picture-db sql 'SELECT path FROM pictures WHERE rating >= 4'
picture-db sql 'SELECT path FROM pictures NATURAL JOIN picture_tags WHERE tag = "food"'
picture-db sql 'SELECT tag, COUNT(*) FROM picture_tags GROUP BY tag ORDER BY COUNT(*) DESC'
```

### Create PhotoPrism albums based on image metadata
```
picture-db photoprism \
  --url 'http://localhost:8080' --user 'admin' --pass 'admin' \
  'SELECT json_path->>6 AS album, path FROM pictures WHERE rating >= 4'

picture-db photoprism \
  --url 'http://localhost:8080' --user 'admin' --pass 'admin' \
  'SELECT "Favorites" AS album, path, SUBSTR(path, 44) AS photoprism_path FROM pictures WHERE rating = 5'
```

### Using a configuration file
```
picture-db --config picturedb.json ...
```

## Table Schema
```
CREATE TABLE `pictures` (
    `path` text NOT NULL,
    `json_path` text NOT NULL,
    `dir` text NOT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    `make` text,
    `model` text,
    `date_time_original` datetime,
    `rating` integer,
    PRIMARY KEY (`path`)
);

CREATE TABLE `picture_tags` (
    `path` text NOT NULL,
    `tag` text NOT NULL,
    PRIMARY KEY (`path`,`tag`),
    CONSTRAINT `fk_pictures_tags` FOREIGN KEY (`path`) REFERENCES `pictures`(`path`) ON DELETE CASCADE
);
```
