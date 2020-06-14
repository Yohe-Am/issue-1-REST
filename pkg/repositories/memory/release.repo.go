package memory

import (
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/release"
)

//releaseRepository ...
type releaseRepository struct {
	cache         map[int]release.Release
	secondaryRepo *release.Repository
}

// NewReleaseRepository returns a struct that implements the release.Repository using
// a cached based implementation.
// A database implementation of the same interface needs to be passed so that it can be
// consulted when the caches aren't enough.
func NewReleaseRepository(secondaryRepo *release.Repository) release.Repository {
	return &releaseRepository{cache: make(map[int]release.Release), secondaryRepo: secondaryRepo}
}

// GetRelease returns the release under the given id from the cache ,if found,
// or from the the wrapped repository,
func (repo *releaseRepository) GetRelease(id int) (*release.Release, error) {
	if _, ok := repo.cache[id]; ok == false {
		r, err := (*repo.secondaryRepo).GetRelease(id)
		if err != nil {
			return nil, err
		}
		repo.cache[id] = *r
	}
	r := repo.cache[id]
	return &r, nil
}

// SearchRelease calls the same method on the wrapped repo with a little caching in between.
func (repo *releaseRepository) SearchRelease(pattern string, by release.SortBy, order release.SortOrder, limit int, offset int) ([]*release.Release, error) {
	result, err := (*repo.secondaryRepo).SearchRelease(pattern, by, order, limit, offset)
	if err == nil {
		for _, r := range result {
			rTemp := *r
			repo.cache[r.ID] = rTemp
		}
	}
	return result, err
}

// DeleteRelease calls the same method on the wrapped repo while also cleaning the cache
// when appropriate.
func (repo *releaseRepository) DeleteRelease(id int) error {
	err := (*repo.secondaryRepo).DeleteRelease(id)
	if err == nil {
		// If deletion is successful, it also tries to delete the user from its cache.
		delete(repo.cache, id)
	}
	return err
}

// AddRelease calls the same method on the wrapped repo with a little caching in between.
func (repo *releaseRepository) AddRelease(r *release.Release) (*release.Release, error) {
	r, err := (*repo.secondaryRepo).AddRelease(r)
	if err == nil {
		repo.cache[r.ID] = *r
	}
	return r, err
}

// UpdateRelease calls the same method on the wrapped repo with a little caching in between.
func (repo *releaseRepository) UpdateRelease(rel *release.Release) (*release.Release, error) {
	r, err := (*repo.secondaryRepo).UpdateRelease(rel)
	if err == nil {
		repo.cache[r.ID] = *r
	}
	return r, err
}
