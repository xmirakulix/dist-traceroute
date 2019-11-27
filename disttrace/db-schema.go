package disttrace

import (
	"database/sql"
	"errors"
)

// getSchemaVersion returns the current schema version of the database
func (db *DB) getSchemaVersion() (int, error) {

	log.Debug("getSchemaVersion: Reading current schema version of database...")

	// check if schema info table exists
	var foundTable string

	query := "SELECT tbl_name FROM sqlite_master WHERE type='table' AND tbl_name='t_SchemaInfo'"
	err := db.QueryRow(query).Scan(&foundTable)
	switch {
	case err == sql.ErrNoRows:
		log.Debug("getSchemaVersion: Schema info table doesn't exist, assuming emtpy database")
		return 0, nil

	case err != nil:
		log.Warn("getSchemaVersion: Error: Couldn't determine db version, Error: ", err)
		return -1, errors.New("Couldn't determine db version")

	case foundTable != "t_SchemaInfo":
		// what happened?!
		log.Warnf("getSchemaVersion: Received unexpected reply '%v' while looking for table 't_SchemaInfo', can't determine db schema version", foundTable)
		return -1, errors.New("Couldn't determine db version")
	}

	// read db version from schema info
	var version int

	query = "SELECT nVersion FROM t_SchemaInfo"
	if err := db.QueryRow(query).Scan(&version); err != nil {
		log.Warn("getSchemaVersion: Error: Couldn't read db version from schemainfo, Error: ", err)
		return -1, errors.New("Couldn't read db version from schemainfo")
	}

	log.Debug("getSchemaVersion: Read dbversion from schema info: ", version)
	return version, nil
}

// createAndUpdateDbSchema checks if the database schema is at newest state and upgrades it if needed
func (db *DB) createAndUpdateDbSchema() error {

	var err error
	log.Debug("createAndUpdateDbSchema: Checking if database schema needs upgrading...")

	// define the schema info for all db versions
	const maxDBVersion = 2
	var schemaUpdate [maxDBVersion + 1][]string

	schemaUpdate[0] = []string{}

	schemaUpdate[1] = []string{
		`CREATE TABLE IF NOT EXISTS t_SchemaInfo (
			nVersion INTEGER PRIMARY KEY
		)`,

		"INSERT INTO t_SchemaInfo VALUES (1)",
	}

	schemaUpdate[2] = []string{
		`CREATE TABLE IF NOT EXISTS t_Traceroutes (
			strTracerouteId TEXT PRIMARY KEY,
			strSlaveId TEXT NOT NULL,
			strTargetId TEXT NOT NULL,
			dtStart INTEGER NOT NULL,
			strAnnotations TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS t_Hops (
			strHopId TEXT PRIMARY KEY, 
			strTracerouteId INTEGER NOT NULL, 
			nHopIndex INTEGER NOT NULL,
			strHopIPAddress TEXT,
			strHopDNSName TEXT, 
			dDurationSec INTEGER,
			strPreviousHopId INTEGER,
			strAnnotations TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS t_Targets (
			strTargetId TEXT PRIMARY KEY,
			strDescription TEXT UNIQUE, 
			strDestination TEXT NOT NULL,
			nRetries INTEGER NOT NULL,
			nMaxHops INTEGER NOT NULL,
			nTimeoutMSec INTEGER NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS t_Slaves (
			strSlaveId TEXT PRIMARY KEY, 
			strSlaveName TEXT NOT NULL UNIQUE,
			strSlaveSecret TEXT NOT NULL 
		)`,

		`CREATE TABLE IF NOT EXISTS t_MasterConfig (
			strConfigId TEXT PRIMARY KEY, 
			strReportURL TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS t_Users (
			strUserId TEXT PRIMARY KEY, 
			strUserName TEXT NOT NULL UNIQUE,
			strPassword TEXT NOT NULL,
			nSalt INTEGER NOT NULL,
			nPassNeedsChange INTEGER NOT NULL
		)`,

		// create admin:123 user
		`INSERT INTO t_Users (strUserId, strUserName, strPassword, nSalt, nPassNeedsChange) 
			VALUES ('998dd43d-86b1-44a3-8f28-d31cd2822927', 'admin', 
			X'd68f3b8ca9aef9120b30823ea284c7bd001fc1beef4b3796fd609bc641031db8', 1298498081, 1); 
		`,

		`UPDATE t_SchemaInfo SET nVersion = 3`,
	}

	// get current DB schema version
	var currentDBVersion int
	if currentDBVersion, err = db.getSchemaVersion(); err != nil {
		log.Warn("createAndUpdateDbSchema: Error: Couldn't get db version: ", err)
		return errors.New("Couldn't get db version")
	}

	// check if schema needs upgrading
	if currentDBVersion >= maxDBVersion {
		log.Infof("createAndUpdateDbSchema: Schema is at version %v, no upgrade needed", currentDBVersion)
		return nil
	}

	log.Warnf("createAndUpdateDbSchema: Database schema needs to be upgraded, current version: %v, upgrading to: %v", currentDBVersion, maxDBVersion)

	// execute upgrade commands beginning at the next version
	for _, cmds := range schemaUpdate[currentDBVersion+1:] {
		currentDBVersion++
		log.Infof("createAndUpdateDbSchema: Upgrading database schema to version: %v", currentDBVersion)
		var tx *Tx
		if tx, err = db.Begin(); err != nil {
			log.Warn("createAndUpdateDbSchema: Couldn't start transaction, Error: ", err)
			return errors.New("Couldn't start transaction")
		}

		// loop through upgrade comnands
		count := 0
		for _, cmd := range cmds {
			count++
			log.Debugf("createAndUpdateDbSchema: Executing command %v of %v...", count, len(cmds))
			// exec Update
			if _, err := tx.Exec(cmd); err != nil {
				log.Warnf("createAndUpdateDbSchema: Error while executing query <%v>, Error: %v", cmd, err)
				log.Warn("createAndUpdateDbSchema: Doing rollback of transaction...")
				// error, rollback
				if err := tx.Rollback(); err != nil {
					log.Warn("createAndUpdateDbSchema: Couldn't rollback transaction, Error: ", err)
					return errors.New("Couldn't rollback transaction")
				}
				log.Warn("createAndUpdateDbSchema: Rollback complete")
				return errors.New("Error while executing query")
			}
		}
		if err := tx.Commit(); err != nil {
			log.Warn("createAndUpdateDbSchema: Couldn't commit transaction, Error: ", err)
			return errors.New("Couldn't commit transaction")
		}
		log.Debug("createAndUpdateDbSchema: Successfully upgraded database schema to version: ", currentDBVersion)
	}

	log.Info("createAndUpdateDbSchema: Finished upgrading the database schema. Now on version: ", currentDBVersion)
	return nil
}

// InitDBConnectionAndUpdate initializes a connection to the database and upgrades the schema if needed
func InitDBConnectionAndUpdate(dataSourceName string) (*DB, error) {
	var db *DB
	var err error

	log.Debug("InitDBConnectionAndUpdate: Opening db connection to: ", dataSourceName)

	if db, err = openDB(dataSourceName); err != nil {
		log.Warn("InitDBConnectionAndUpdate: Error while opening database connection, Error: ", err)
		return nil, errors.New("Error while opening database connection")
	}

	if err := db.createAndUpdateDbSchema(); err != nil {
		log.Warn("InitDBConnectionAndUpdate: Error while checking database schema, Error: ", err)
		return nil, errors.New("Error while checking database schema")
	}

	log.Debug("InitDBConnectionAndUpdate: Successfully established database connection")
	return db, nil
}
