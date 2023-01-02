package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func sql(config *Config, statement string) error {
	// open database
	db, err := openDatabase(config.DBPath)
	if err != nil {
		return err
	}

	// execute SQL command
	rows, err := db.Raw(statement).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	// print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	for _, col := range cols {
		fmt.Fprintf(w, "%s\t", col)
	}
	fmt.Fprintln(w)

	for _, col := range cols {
		fmt.Fprintf(w, "%s\t", strings.Repeat("-", len(col)))
	}
	fmt.Fprintln(w)

	// print rows
	vals := make([]interface{}, len(cols))
	for i := range vals {
		var ii interface{}
		vals[i] = &ii
	}
	for rows.Next() {
		rows.Scan(vals...)

		for i := range cols {
			val := *(vals[i].(*interface{}))
			fmt.Fprintf(w, "%v\t", val)
		}
		fmt.Fprintln(w)
	}

	// print table to stdout
	return w.Flush()
}
