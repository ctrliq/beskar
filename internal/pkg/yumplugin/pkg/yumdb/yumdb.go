package yumdb

import (
	"context"
	"fmt"
	"os"

	"go.ciq.dev/beskar/pkg/doltdb"
)

type WalkPackageFunc func(*Package) error

type Package struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	Primary   []byte `db:"meta_primary"`
	Filelists []byte `db:"meta_filelists"`
	Other     []byte `db:"meta_other"`
}

var packageTable = `CREATE TABLE IF NOT EXISTS packages (
	id VARCHAR(64) PRIMARY KEY,
	name VARCHAR(512),
	meta_primary MEDIUMBLOB,
	meta_filelists MEDIUMBLOB,
	meta_other MEDIUMBLOB
);
`

type YumDB struct {
	*doltdb.DB
}

func Open(path string) (*YumDB, error) {
	db, err := doltdb.Open(path, "yum")
	if err != nil {
		return nil, fmt.Errorf("while opening dolt DB %s: %w", path, err)
	}

	_, err = db.Exec(packageTable)
	if err != nil {
		return nil, fmt.Errorf("while initializing yum DB: %w", err)
	}

	return &YumDB{db}, nil
}

func (db *YumDB) AddPackage(ctx context.Context, id, name, primary, filelists, other string) error {
	var err error

	dbPackage := &Package{
		ID:   id,
		Name: name,
	}

	dbPackage.Primary, err = os.ReadFile(primary)
	if err != nil {
		return err
	}
	dbPackage.Filelists, err = os.ReadFile(filelists)
	if err != nil {
		return err
	}
	dbPackage.Other, err = os.ReadFile(other)
	if err != nil {
		return err
	}

	result, err := db.NamedExecContext(ctx, "INSERT INTO packages VALUES(:id, :name, :meta_primary, :meta_filelists, :meta_other)", dbPackage)
	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("package not inserted into database")
	}

	return db.CommitAll(ctx, id)
}

func (db *YumDB) CountPackages(ctx context.Context) (int, error) {
	//nolint:sqlclosecheck // closed by caller
	rows, err := db.QueryxContext(ctx, "SELECT COUNT(id) AS id FROM packages")
	if err != nil {
		return 0, err
	}

	count := 0

	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}

	return count, nil
}

func (db *YumDB) WalkPackages(ctx context.Context, walkFn WalkPackageFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no walk package function provided")
	}

	//nolint:sqlclosecheck // closed by caller
	rows, err := db.QueryxContext(ctx, "SELECT * FROM packages")
	if err != nil {
		return err
	}

	for rows.Next() {
		pkg := new(Package)
		err := rows.StructScan(pkg)
		if err != nil {
			return err
		} else if err := walkFn(pkg); err != nil {
			return err
		}
	}

	return nil
}
