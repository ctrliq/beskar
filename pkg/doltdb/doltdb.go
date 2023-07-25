package doltdb

import (
	"context"
	"fmt"

	// import dolt sql driver
	_ "github.com/dolthub/driver"
	"github.com/jmoiron/sqlx"
)

type DB struct {
	*sqlx.DB
}

func Open(path, database string) (*DB, error) {
	source := fmt.Sprintf(
		"file://%s?commitname=beskar&commitemail=beskar@ciq.com&database=%s&multistatements=true",
		path, database,
	)

	db, err := sqlx.Open("dolt", source)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", database))
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(fmt.Sprintf("USE %s", database))
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) Close() error {
	err := db.DB.Close()
	return err
}

func (db *DB) CommitAll(ctx context.Context, message string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("CALL DOLT_COMMIT('-Am', '%s')", message))
	return err
}
