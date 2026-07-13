package database

import "database/sql"

func InsertMessage(db *sql.DB, sender, content string) error {

	query := `
	INSERT INTO messages (sender, content)
	VALUES ($1, $2)
	`

	-,err := db.Exec(query, sender, content)
	if err != nil{
		return err
	}

	return nil
}