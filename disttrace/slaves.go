package disttrace

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// SlaveConfig holds the configuration for a dist-traceroute-slave
type SlaveConfig struct {
	ID         uuid.UUID     `json:",omitempty" valid:"-"`
	MasterHost string        `json:"-" valid:"-"`
	MasterPort string        `json:"-" valid:"-"`
	Targets    []TraceTarget `valid:"-"`
}

// Slave holds all infos about a slave
type Slave struct {
	ID     uuid.UUID `json:",omitempty" valid:"-"`
	Name   string    `valid:"alphanum,	required"`
	Secret string    `valid:"alphanum,	required"`
}

// CheckSlaveAuth checks supplied credentials for validity
func CheckSlaveAuth(db *DB, user string, secret string) (bool, uuid.UUID) {
	log.Debugf("CheckSlaveAuth: Checking auth for slave<%v> secret<%v> for validity...", user, secret)

	query := `
		SELECT strSlaveId 
		FROM t_Slaves
		WHERE strSlaveName = ? AND strSlaveSecret = ?
		LIMIT 1
		`

	var slaveID uuid.UUID

	row := db.QueryRow(query, user, secret)
	if err := row.Scan(&slaveID); err != nil {
		if err == sql.ErrNoRows {
			log.Debug("CheckSlaveAuth: No data found, returning false")
		} else {
			log.Warn("CheckSlaveAuth: Error while getting slave data, Error: ", err.Error())
		}
		return false, uuid.Nil
	}

	log.Debug("CheckSlaveAuth: Slave auth are valid...")
	return true, slaveID
}

// GetSlave returns the specified slave from DB
func GetSlave(slaveID uuid.UUID, db *DB) (Slave, error) {

	log.Debug("GetSlave: fetching slave with ID: ", slaveID)
	slave := Slave{}

	query := "SELECT strSlaveID, strSlaveName, strSlaveSecret FROM t_Slaves WHERE strSlaveId = ?"

	row := db.QueryRow(query, slaveID)
	if err := row.Scan(&slave.ID, &slave.Name, &slave.Secret); err != nil {
		if err == sql.ErrNoRows {
			log.Debug("GetSlave: Couldn't find specified slave in DB...")
			return Slave{}, nil
		}
		log.Warn("GetSlave: Error while getting slave from DB, Error: ", err)
		return Slave{}, errors.New("Error while getting slave from DB")
	}

	log.Debug("GetSlave: Returning slave name '%v' for ID '%v'", slave.Name, slave.ID)
	return slave, nil
}

// GetSlaves reads all slaves from the db
func GetSlaves(db *DB) ([]Slave, error) {

	log.Debug("GetSlaves: fetching slaves from db...")
	slaves := []Slave{}

	query := "SELECT strSlaveId, strSlaveName, strSlaveSecret FROM t_Slaves"
	rows, err := db.Query(query)
	if err != nil {
		log.Warn("GetSlaves: Couldn't get slaves from db, Error: ", err)
		return slaves, errors.New("Couldn't get slaves")
	}
	defer rows.Close()

	for rows.Next() {
		var slave = Slave{}
		if err := rows.Scan(&slave.ID, &slave.Name, &slave.Secret); err != nil {
			log.Warn("GetSlaves: Couldn't read results from db, Error: ", err)
			return []Slave{}, errors.New("Couldn't get slaves")
		}
		slaves = append(slaves, slave)
	}

	log.Debugf("GetSlaves: returning '%v' slaves from db...", len(slaves))
	return slaves, nil
}

// CreateSlave stores a new slave in the db
func CreateSlave(db *DB, slave Slave) (Slave, error) {
	log.Debug("CreateSlave: Creating new slave, name: ", slave.Name)

	query := "INSERT INTO t_Slaves (strSlaveId, strSlaveName, strSlaveSecret) VALUES (?, ?, ?)"

	slave.ID = uuid.New()
	_, err := db.Exec(query, slave.ID, slave.Name, slave.Secret)
	if err != nil {
		log.Warn("CreateSlave: Couldn't create slave, Error: ", err)
		return Slave{}, errors.New("Couldn't create slave")
	}

	log.Debugf("CreateSlave: Slave '%v' created with ID<%v>", slave.Name, slave.ID)
	return slave, nil
}

// UpdateSlave updates an existing new slave in the db
func UpdateSlave(db *DB, slave Slave) (Slave, error) {
	log.Debug("UpdateSlave: Updating slave '%v'...", slave.ID)

	query := "UPDATE t_Slaves SET strSlaveName = ?, strSlaveSecret = ? WHERE strSlaveId = ?"

	res, err := db.Exec(query, slave.Name, slave.Secret, slave.ID)
	if err != nil {
		log.Warn("UpdateSlave: Couldn't update slave, Error: ", err)
		return Slave{}, errors.New("Couldn't update slave")
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		log.Warn("UpdateSlave: Error: Can't get number of affected rows, Error: ", err)
		return Slave{}, errors.New("DB Error")
	}

	log.Debugf("UpdateSlave: Slave '%v' successfully updated, affected rows: '%v", slave.ID, numRows)
	return slave, nil
}

// DeleteSlave deletes an existing slave from the db
func DeleteSlave(db *DB, slaveID uuid.UUID) error {
	log.Debugf("DeleteSlave: Deleting slave '%v'...", slaveID)

	query := "DELETE FROM t_Slaves WHERE strSlaveId = ?"

	res, err := db.Exec(query, slaveID)
	if err != nil {
		log.Warn("DeleteSlave: Couldn't delete slave, Error: ", err)
		return errors.New("Couldn't delete slave")
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		log.Warn("DeleteSlave: Error: Can't get number of affected rows, Error: ", err)
		return errors.New("DB Error")
	}

	log.Debugf("DeleteSlave: Slave '%v' successfully deleted, rows: '%v'", slaveID, numRows)
	return nil

}
