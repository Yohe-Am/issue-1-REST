package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/auth"
	"golang.org/x/crypto/bcrypt"
)

type jWtAuthRepository repository

// NewAuthRepository returns a struct that implements the auth.Repository using
// a PostgresSQL database.
// A database connection needs to be passed so that it can function.
func NewAuthRepository(DB *sql.DB, allRepos *map[string]interface{}) auth.Repository {
	return &jWtAuthRepository{DB, allRepos}
}

// Authenticate checks the given pass hash against the pass hash found in the database for the username.
func (repo *jWtAuthRepository) Authenticate(u *auth.User) (bool, error) {
	query := `SELECT pass_hash from users where username = $1 or email = $2`
	var passHash string
	err := repo.db.QueryRow(query, u.Username, u.Email).Scan(&passHash)
	if err == sql.ErrNoRows {
		return false, auth.ErrUserNotFound
	} else if err != nil {
		return false, fmt.Errorf("couldn't get passhash of user beacause: %w", err)
	}
	err = bcrypt.CompareHashAndPassword([]byte(passHash), []byte(u.Password))
	switch err {
	case nil:
		return true, nil
	case bcrypt.ErrHashTooShort:
		return false, err
	case bcrypt.ErrMismatchedHashAndPassword:
		return false, nil
	default:
		return false, fmt.Errorf("bcrypt hash compare failed because: %w", err)
	}
}

// AddToBlacklist - this method is only implemented in memory cache as of now.
func (repo *jWtAuthRepository) AddToBlacklist(tokenString string) error {
	panic("not implemented")
}

// IsInBlacklist - this method is only implemented in memory cache as of now.
func (repo *jWtAuthRepository) IsInBlacklist(token string) (bool, error) {
	panic("not implemented")
}
