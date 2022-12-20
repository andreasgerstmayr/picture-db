# picture-db
picture-db stores metadata of photo collections in a sqlite database.
This allows querying photo metadata via SQL.

## Usage
### Create or update database
```
picture-db index
```

### Run SQL queries
```
picture-db sql 'SELECT * FROM pictures'
picture-db sql 'SELECT path FROM pictures WHERE rating >= 4'
```

Note: This requires the `sqlite3` binary in your `$PATH`.

### Create PhotoPrism albums based on image metadata
```
picture-db photoprism \
  --url 'http://localhost:8080' --user 'admin' --pass 'admin' \
  'SELECT dir2 AS album, path FROM pictures WHERE rating >= 4'
```

### Using a configuration file
```
picture-db --config picturedb.json ...
```

## Table Schema
```
CREATE TABLE `pictures` (
    `path` text NOT NULL,
    `dir` text NOT NULL,
    `dir1` text NOT NULL,
    `dir2` text NOT NULL,
    `dir3` text NOT NULL,
    `created_at` datetime NOT NULL,
    `updated_at` datetime NOT NULL,
    `make` text,
    `model` text,
    `date_time_original` datetime,
    `rating` integer,
    `tags` text NOT NULL,
    PRIMARY KEY (`path`)
);
```
