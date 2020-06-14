package memory

import (
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/feed"
)

//feedRepository ...
type feedRepository struct {
	cache         map[string]feed.Feed
	secondaryRepo *feed.Repository
	allRepos      *map[string]interface{}
}

// NewFeedRepository returns a new in memory cache implementation of feed.Repository.
// The database implementation of feed.Repository must be passed as the first argument
// since to simplify logic, cache repos wrap the database repos.
// A map of all the other cache based implementations of the Repository interfaces
// found in the different services of the project must be passed as a second argument as
// the Repository might make use of them to fetch objects instead of implementing redundant logic.
func NewFeedRepository(secondaryRepo *feed.Repository, allRepos *map[string]interface{}) feed.Repository {
	return &feedRepository{cache: make(map[string]feed.Feed), secondaryRepo: secondaryRepo, allRepos: allRepos}
}

// GetFeed returns the feed belonging to the given username from either the
// cache if found there or the secondary repos it wraps.
func (repo *feedRepository) GetFeed(username string) (*feed.Feed, error) {
	if _, ok := repo.cache[username]; ok == false {
		err := repo.cacheFeed(username)
		if err != nil {
			return nil, err
		}
	}
	f := repo.cache[username]
	return &f, nil
}

//GetChannels directly calls the same method on the secondary repos it wraps to
// get all the channels the given feed has subscribed to.
func (repo *feedRepository) GetChannels(f *feed.Feed, sortBy string, sortOrder string) ([]*feed.Channel, error) {
	return (*repo.secondaryRepo).GetChannels(f, sortBy, sortOrder)
}

//AddFeed directly calls the same method on the secondary repos it wraps to
// persist a feed to the username contained in the feed struct.
func (repo *feedRepository) AddFeed(f *feed.Feed) error {
	return (*repo.secondaryRepo).AddFeed(f)
}

// cacheFeed is a helper function used to refresh items to the cache
func (repo *feedRepository) cacheFeed(username string) error {
	u, err := (*repo.secondaryRepo).GetFeed(username)
	if err != nil {
		return err
	}
	repo.cache[username] = *u
	return nil
}

//GetPosts directly calls the same method on the secondary repos it wraps to
// retrieve a list of posts collected from the channels the given feed has
// subscribed to sorted according to the given method.
func (repo *feedRepository) GetPosts(f *feed.Feed, sort feed.Sorting, limit, offset int) ([]*feed.Post, error) {
	return (*repo.secondaryRepo).GetPosts(f, sort, limit, offset)
}

// UpdateFeed directly calls the same method on the secondary repos it wraps to
// update the feed at the given id according to the given struct and if successful,
// refreshes the feed entry of the cache.
func (repo *feedRepository) UpdateFeed(id uint, f *feed.Feed) error {
	err := (*repo.secondaryRepo).UpdateFeed(id, f)
	if err == nil {
		err = repo.cacheFeed(f.OwnerUsername)
	}
	return err
}

// Subscribe directly calls the same method on the secondary repos it wraps to
// add the channel to the list of channels that the feed collects posts from.
func (repo *feedRepository) Subscribe(f *feed.Feed, channelname string) error {
	return (*repo.secondaryRepo).Subscribe(f, channelname)
}

//Unsubscribe directly calls the same method on the secondary repos it wraps to
// remove the channel to the list of channels that the feed collects posts from.
func (repo *feedRepository) Unsubscribe(f *feed.Feed, channelname string) error {
	return (*repo.secondaryRepo).Unsubscribe(f, channelname)
}
