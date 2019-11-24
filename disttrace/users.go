package disttrace

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"math"
	"math/rand"
	"strconv"

	"github.com/google/uuid"
)

// User contains information about a single dist-traceroute user
type User struct {
	ID                  uuid.UUID
	Name                string
	Password            string
	Salt                int
	PasswordNeedsChange bool
}

// GetUser returns the specified user from DB
func GetUser(userID uuid.UUID, db *DB) (User, error) {

	log.Debug("GetUser: fetching user with ID: ", userID)
	user := User{}

	query := "SELECT strUserId, strUserName, strPassword, nSalt, nPassNeedsChange FROM t_Users WHERE strUserId = ?"

	row := db.QueryRow(query, userID)
	if err := row.Scan(&user.ID, &user.Name, &user.Password, &user.Salt, &user.PasswordNeedsChange); err != nil {
		if err == sql.ErrNoRows {
			log.Debug("GetUser: Couldn't find specified user in DB...")
			return User{}, nil
		}
		log.Warn("GetUser: Error while getting user from DB, Error: ", err)
		return User{}, errors.New("Error while getting user from DB")
	}

	log.Debug("GetUser: Returning user name '%v' for ID '%v'", user.Name, user.ID)
	return user, nil
}

// GetUsers reads all users from the db
func GetUsers(db *DB) ([]User, error) {

	log.Debug("GetUsers: fetching users from db...")
	users := []User{}

	query := "SELECT strUserId, strUserName, strPassword, nSalt, nPassNeedsChange FROM t_Users"
	rows, err := db.Query(query)
	if err != nil {
		log.Warn("GetUsers: Couldn't get users from db, Error: ", err)
		return users, errors.New("Couldn't get users")
	}
	defer rows.Close()

	for rows.Next() {
		var user = User{}
		if err := rows.Scan(&user.ID, &user.Name, &user.Password, &user.Salt, &user.PasswordNeedsChange); err != nil {
			log.Warn("GetUsers: Couldn't read results from users, Error: ", err)
			return []User{}, errors.New("Couldn't get users")
		}
		users = append(users, user)
	}

	log.Debugf("GetUsers: returning '%v' users from db...", len(users))
	return users, nil
}

// CreateUser stores a new user in the db
func CreateUser(db *DB, user User) (User, error) {
	log.Debug("CreateUser: Creating new user, name: ", user.Name)

	query := `
	INSERT INTO t_Users (strUserId, strUserName, strPassword, nSalt, nPassNeedsChange) 
	VALUES (?, ?, ?, ?, ?)
	`

	user.ID = uuid.New()
	user.Salt = rand.Intn(math.MaxInt32)
	password := sha256.Sum256([]byte(user.Password + strconv.Itoa(user.Salt)))
	user.Password = string(password[:])

	_, err := db.Exec(query, user.ID, user.Name, user.Password, user.Salt, user.PasswordNeedsChange)
	if err != nil {
		log.Warn("CreateUser: Couldn't create user, Error: ", err)
		return User{}, errors.New("Couldn't create user")
	}

	log.Debugf("CreateUser: User '%v' created with ID<%v>", user.Name, user.ID)
	return user, nil
}

// UpdateUser updates an existing user in the db
func UpdateUser(db *DB, user User) (User, error) {
	log.Debug("UpdateUser: Updating user '%v'...", user.ID)

	oldUser, err := GetUser(user.ID, db)
	if err != nil {
		log.Warn("UpdateUser: Couldn't get old userinfo from DB, Error: ", err)
		return User{}, errors.New("Couldn't get old userinfo from DB")
	}

	if oldUser.Password != user.Password {
		log.Debug("UpdateUser: PW has changed, setting new hashed pw...")
		user.Salt = rand.Intn(math.MaxInt32)
		password := sha256.Sum256([]byte(user.Password + strconv.Itoa(user.Salt)))
		user.Password = string(password[:])
	}

	query := `UPDATE t_Users 
	SET strUserName = ?, strPassword = ?, nSalt = ?, nPassNeedsChange = ?
	WHERE strUserId = ?`

	res, err := db.Exec(query, user.Name, user.Password, user.Password, user.PasswordNeedsChange)
	if err != nil {
		log.Warn("UpdateUser: Couldn't update user, Error: ", err)
		return User{}, errors.New("Couldn't update user")
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		log.Warn("UpdateUser: Error: Can't get number of affected rows, Error: ", err)
		return User{}, errors.New("DB Error")
	}

	log.Debugf("UpdateUser: User '%v' successfully updated, affected rows: '%v", user.ID, numRows)
	return user, nil
}

// DeleteUser deletes an existing user from the db
func DeleteUser(db *DB, userID uuid.UUID) error {
	log.Debugf("DeleteUser: Deleting user '%v'...", userID)

	query := "DELETE FROM t_Users WHERE strUserId = ?"

	res, err := db.Exec(query, userID)
	if err != nil {
		log.Warn("DeleteUser: Couldn't delete user, Error: ", err)
		return errors.New("Couldn't delete user")
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		log.Warn("DeleteUser: Error: Can't get number of affected rows, Error: ", err)
		return errors.New("DB Error")
	}

	log.Debugf("DeleteUser: User '%v' successfully deleted, rows: '%v'", userID, numRows)
	return nil

}
