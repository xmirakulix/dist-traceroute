package disttrace

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// TraceTarget contains information about a single dist-traceroute target
type TraceTarget struct {
	ID        uuid.UUID `valid:"-"`
	Name      string    `valid:"alphanum,	required"`
	Address   string    `valid:"host,		required"`
	Retries   int       `valid:"int,	required,	range(0|10)"`
	MaxHops   int       `valid:"int,	required,	range(1|100)"`
	TimeoutMs int       `valid:"int,	required,	range(1|10000)"`
}

// GetTarget returns the specified target from DB
func GetTarget(targetID uuid.UUID, db *DB) (TraceTarget, error) {

	log.Debug("GetTarget: fetching target with ID: ", targetID)
	target := TraceTarget{}

	query := "SELECT strTargetId, strDescription, strDestination, nRetries, nMaxHops, nTimeoutMSec FROM t_Targets WHERE strTargetId = ?"

	row := db.QueryRow(query, targetID)
	if err := row.Scan(&target.ID, &target.Name, &target.Address, &target.Retries, &target.MaxHops, &target.TimeoutMs); err != nil {
		if err == sql.ErrNoRows {
			log.Debug("GetTarget: Couldn't find specified target in DB...")
			return TraceTarget{}, nil
		}
		log.Debug("GetTarget: Error while getting target from DB, Error: ", err)
		return TraceTarget{}, errors.New("Error while getting target from DB")
	}

	log.Debug("GetTarget: Returning target name '%v' for ID '%v'", target.Name, target.ID)
	return target, nil
}

// GetTargets reads all targets from the db
func GetTargets(db *DB) ([]TraceTarget, error) {

	log.Debug("GetTargets: fetching targets from db...")
	targets := []TraceTarget{}

	query := "SELECT strTargetId, strDescription, strDestination, nRetries, nMaxHops, nTimeoutMSec FROM t_Targets"
	rows, err := db.Query(query)
	if err != nil {
		log.Warn("GetTargets: Couldn't get targets from db, Error: ", err)
		return targets, errors.New("Couldn't get targets")
	}
	defer rows.Close()

	for rows.Next() {
		var target = TraceTarget{}
		if err := rows.Scan(&target.ID, &target.Name, &target.Address, &target.Retries, &target.MaxHops, &target.TimeoutMs); err != nil {
			log.Warn("GetTargets: Couldn't read results from targets, Error: ", err)
			return []TraceTarget{}, errors.New("Couldn't get targets")
		}
		targets = append(targets, target)
	}

	log.Debugf("GetTargets: returning '%v' targets from db...", len(targets))
	return targets, nil
}

// CreateTarget stores a new target in the db
func CreateTarget(db *DB, target TraceTarget) (TraceTarget, error) {
	log.Debug("CreateTarget: Creating new target, name: ", target.Name)

	query := `
	INSERT INTO t_Targets (strTargetId, strDescription, strDestination, nRetries, nMaxHops, nTimeoutMSec) 
	VALUES (?, ?, ?, ?, ?, ?)
	`

	target.ID = uuid.New()
	_, err := db.Exec(query, target.ID, target.Name, target.Address, target.Retries, target.MaxHops, target.TimeoutMs)
	if err != nil {
		log.Warn("CreateTarget: Couldn't create target, Error: ", err)
		return TraceTarget{}, errors.New("Couldn't create target")
	}

	log.Debugf("CreateTarget: Target '%v' created with ID<%v>", target.Name, target.ID)
	return target, nil
}

// UpdateTarget updates an existing new target in the db
func UpdateTarget(db *DB, target TraceTarget) (TraceTarget, error) {
	log.Debug("UpdateTarget: Updating target '%v'...", target.ID)

	query := `UPDATE t_Targets 
	SET strDescription = ?, strDestination = ?, nRetries = ?, nMaxHops = ?, nTimeoutMSec = ?
	WHERE strTargetId = ?`

	res, err := db.Exec(query, target.Name, target.Address, target.Retries, target.MaxHops, target.TimeoutMs, target.ID)
	if err != nil {
		log.Warn("UpdateTarget: Couldn't update target, Error: ", err)
		return TraceTarget{}, errors.New("Couldn't update target")
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		log.Debug("UpdateTarget: Error: Can't get number of affected rows, Error: ", err)
		return TraceTarget{}, errors.New("DB Error")
	}

	log.Debugf("UpdateTarget: Target '%v' successfully updated, affected rows: '%v", target.ID, numRows)
	return target, nil
}

// DeleteTarget deletes an existing target from the db
func DeleteTarget(db *DB, targetID uuid.UUID) error {
	log.Debugf("DeleteTarget: Deleting target '%v'...", targetID)

	query := "DELETE FROM t_Targets WHERE strTargetId = ?"

	res, err := db.Exec(query, targetID)
	if err != nil {
		log.Warn("DeleteTarget: Couldn't delete target, Error: ", err)
		return errors.New("Couldn't delete target")
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		log.Debug("DeleteTarget: Error: Can't get number of affected rows, Error: ", err)
		return errors.New("DB Error")
	}

	log.Debugf("DeleteTarget: Target '%v' successfully deleted, rows: '%v'", targetID, numRows)
	return nil

}
