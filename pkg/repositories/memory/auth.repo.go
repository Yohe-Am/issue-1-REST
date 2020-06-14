package memory

import (
	"github.com/Yohe-Am/issue-1-REST/pkg/services/auth"
)

type jWtAuthRepository struct {
	blacklist     map[string]struct{}
	secondaryRepo *auth.Repository
}

// NewAuthRepository returns a new in memory cache implementation of auth.Repository.
// The database implementation of auth.Repository must be passed as the first argument
// since to simplify logic, cache repos wrap the database repos.
func NewAuthRepository(dbRepo *auth.Repository) auth.Repository {
	return &jWtAuthRepository{
		blacklist:     make(map[string]struct{}),
		secondaryRepo: dbRepo,
	}
}

// Authenticate checks whether the given User struct holds appropriate credentials
func (repo *jWtAuthRepository) Authenticate(user *auth.User) (bool, error) {
	return (*repo.secondaryRepo).Authenticate(user)
}

// AddToBlacklist adds a given token to the list of tokens that can not be used no more.
func (repo *jWtAuthRepository) AddToBlacklist(tokenString string) error {
	(*repo).blacklist[tokenString] = struct{}{}
	return nil
}

// IsInBlacklist checks whether a given token is invalidated previously.
func (repo *jWtAuthRepository) IsInBlacklist(token string) (bool, error) {
	_, ok := (*repo).blacklist[token]

	// ok will be true if it's in the blacklist
	return ok, nil
}
