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

## Table Schema
```
CREATE TABLE `pictures` (
    `path` text,
    `dir` text,
    `dir1` text,
    `dir2` text,
    `dir3` text,
    `created_at` datetime,
    `updated_at` datetime,
    `make` text,
    `model` text,
    `date_time_original` datetime,
    `rating` integer,
    `tags` text,
    PRIMARY KEY (`path`)
);
```
