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

// SlaveCredentials hold authentication information for slaves on master
type SlaveCredentials struct {
	ID       uuid.UUID `valid:"-"`
	Name     string    `valid:"alphanum,	required"`
	Password string    `valid:"alphanum,	required"`
}

// CheckSlaveCreds checks supplied credentials for validity
func CheckSlaveCreds(db *DB, user string, secret string) bool {
	log.Debugf("CheckSlaveCreds: Checking creds for slave<%v> secret<%v> for validity...", user, secret)

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
			log.Debug("CheckSlaveCreds: No data found, returning false")
		} else {
			log.Warn("CheckSlaveCreds: Error while getting slave data, Error: ", err.Error())
		}
		return false
	}

	log.Debug("CheckSlaveCreds: Slave creds are valid...")
	return true
}

// GetSlaves reads all slaves from the db
func GetSlaves(db *DB, slaveName string) ([]SlaveCredentials, error) {

	slaves := []SlaveCredentials{}

	query := "SELECT strSlaveId, strSlaveName, strSlaveSecret FROM t_Slaves"
	rows, err := db.Query(query)
	if err != nil {
		log.Warn("getMasterConfigFromDB: Couldn't get slaves from db, Error: ", err)
		return slaves, errors.New("Couldn't get slaves")
	}
	defer rows.Close()

	for rows.Next() {
		var slaveCfg = SlaveCredentials{}
		if err := rows.Scan(&slaveCfg.ID, &slaveCfg.Name, &slaveCfg.Password); err != nil {
			log.Warn("getMasterConfigFromDB: Couldn't read results from slaves, Error: ", err)
			return []SlaveCredentials{}, errors.New("Couldn't get slaves")
		}
		slaves = append(slaves, slaveCfg)
	}

	return slaves, nil
}
