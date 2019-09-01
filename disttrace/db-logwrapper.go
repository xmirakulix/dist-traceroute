package disttrace

import (
	"database/sql"
	"time"

	// init sqlite3
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps sql.DB
type DB struct {
	*sql.DB
}

// Tx wraps sql.Tx
type Tx struct {
	*sql.Tx
}

// Stmt wraps sql.Stmt
type Stmt struct {
	*sql.Stmt
}

// open creates the database connection and returns a db reference
func openDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		log.Warn("DB Open: Error while opening the database connection, Error: ", err)
		return nil, err
	}
	log.Debug("DB Open: Successfully opened database: ", dataSourceName)
	return &DB{db}, nil
}

// Begin creates and returns a new transaction
func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		log.Warn("DB Begin: Error while creating new transaction, Error: ", err)
		return nil, err
	}
	log.Debug("DB Begin: Successfully started transaction...")
	return &Tx{tx}, nil
}

// Commit commits a transaction to the database
func (tx *Tx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		log.Warn("DB Commit: Error while commiting the transaction, Error: ", err)
		return err
	}
	log.Debug("DB Commit: Successfully commited transaction")
	return nil
}

// Rollback aborts the transaction
func (tx *Tx) Rollback() error {
	if err := tx.Tx.Rollback(); err != nil {
		log.Warn("DB Rollback: Error while rolling the transaction back, Error: ", err)
		return err
	}
	log.Debug("DB Rollback: Successfully rolled transaction back")
	return nil
}

// Query executes a query and returns a reference to the resultset
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {

	startTime := time.Now()

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		log.Warnf("DB Query: Error while executing query <%v>, duration: %v, Error: %v", query, time.Since(startTime), err)
		return nil, err
	}

	log.Debugf("DB Query: Successfully executed query <%v>, duration: %v", query, time.Since(startTime))
	return rows, nil
}

// QueryRow executes a query and returns a single row
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {

	startTime := time.Now()

	row := db.DB.QueryRow(query, args...)

	log.Debugf("DB QueryRow: Successfully executed query <%v>, duration: %v", query, time.Since(startTime))
	return row
}

// Exec executes a statement and returns the result
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {

	startTime := time.Now()

	result, err := db.DB.Exec(query, args...)
	if err != nil {
		log.Warnf("DB Exec: Error while executing statement <%v>, duration: %v, Error: %v", query, time.Since(startTime), err)
		return nil, err
	}

	log.Debugf("DB Exec: Successfully executed statement <%v>, duration: %v", query, time.Since(startTime))
	return result, nil
}

// Ping verifies the connection to the database and establishes it if needed
func (db *DB) Ping() error {
	if err := db.DB.Ping(); err != nil {
		log.Warn("DB Ping: Error while pinging database, Error: ", err)
		return err
	}
	return nil
}

// Close closes the connection to the database
func (db *DB) Close() error {
	if err := db.DB.Close(); err != nil {
		log.Warn("DB Close: Error while closing connection, Error: ", err)
		return err
	}
	return nil
}

// Query executes a query on the given transaction and returns a reference to the resultset
func (tx *Tx) Query(query string, args ...interface{}) (*sql.Rows, error) {

	startTime := time.Now()

	rows, err := tx.Tx.Query(query, args...)
	if err != nil {
		log.Warnf("DB QueryTx: Error while executing query <%v>, duration: %v, Error: %v", query, time.Since(startTime), err)
		return nil, err
	}

	log.Debugf("DB QueryTx: Successfully executed query <%v>, duration: %v", query, time.Since(startTime))
	return rows, nil
}

// QueryRow executes a query on the given transaction and returns a single row
func (tx *Tx) QueryRow(query string, args ...interface{}) *sql.Row {

	startTime := time.Now()

	row := tx.Tx.QueryRow(query, args...)

	log.Debugf("DB QueryRowTx: Successfully executed query <%v>, duration: %v", query, time.Since(startTime))
	return row
}

// Exec executes a statement on the given transaction and returns the result
func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {

	startTime := time.Now()

	result, err := tx.Tx.Exec(query, args...)
	if err != nil {
		log.Warnf("DB ExecTx: Error while executing statement <%v>, duration: %v, Error: %v", query, time.Since(startTime), err)
		return nil, err
	}

	log.Debugf("DB ExecTx: Successfully executed statement <%v>, duration: %v", query, time.Since(startTime))
	return result, nil
}

// Prepare creates a prepared statement
func (tx *Tx) Prepare(query string) (*Stmt, error) {

	startTime := time.Now()

	stmt, err := tx.Tx.Prepare(query)

	if err != nil {
		log.Warnf("DB Prepare: Error while preparing statement <%v>, duration: %v, Error: %v", query, time.Since(startTime), err)
		return nil, err
	}

	log.Debugf("DB Prepare: Successfully prepared statement for query <%v>, duration: %v", query, time.Since(startTime))
	return &Stmt{stmt}, nil
}

// Exec executes a prepared statement
func (stmt *Stmt) Exec(args ...interface{}) (sql.Result, error) {

	startTime := time.Now()

	result, err := stmt.Stmt.Exec(args...)
	if err != nil {
		log.Warnf("DB ExecStmt: Error while executing prepared statement, duration: %v, Error: %v", time.Since(startTime), err)
		return nil, err
	}

	log.Debugf("DB ExecStmt: Successfully executed prepared statement, duration: %v", time.Since(startTime))
	return result, nil
}
