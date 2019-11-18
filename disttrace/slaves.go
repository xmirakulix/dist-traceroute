package disttrace

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// SlaveConfig holds the configuration for a dist-traceroute-slave
type SlaveConfig struct {
	MasterHost string        `json:"-" valid:"-"`
	MasterPort string        `json:"-" valid:"-"`
	Targets    []TraceTarget `valid:"-"`
}

// Slave holds all infos about a slave
type Slave struct {
	ID       uuid.UUID `valid:"-"`
	Name     string    `valid:"alphanum,	required"`
	Password string    `valid:"alphanum,	required"`
}

// CheckSlaveAuth checks supplied credentials for validity
func CheckSlaveAuth(db *DB, user string, secret string) bool {
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
		return false
	}

	log.Debug("CheckSlaveAuth: Slave auth are valid...")
	return true
}

// GetSlaves reads all slaves from the db
func GetSlaves(db *DB) ([]Slave, error) {

	slaves := []Slave{}

	query := "SELECT strSlaveId, strSlaveName, strSlaveSecret FROM t_Slaves"
	rows, err := db.Query(query)
	if err != nil {
		log.Warn("getMasterConfigFromDB: Couldn't get slaves from db, Error: ", err)
		return slaves, errors.New("Couldn't get slaves")
	}
	defer rows.Close()

	for rows.Next() {
		var slave = Slave{}
		if err := rows.Scan(&slave.ID, &slave.Name, &slave.Password); err != nil {
			log.Warn("getMasterConfigFromDB: Couldn't read results from slaves, Error: ", err)
			return []Slave{}, errors.New("Couldn't get slaves")
		}
		slaves = append(slaves, slave)
	}

	return slaves, nil
}
