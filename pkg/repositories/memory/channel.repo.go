package memory

import (
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/channel"
)

//ChannelRepository...
type ChannelRepository struct {
	cache         map[string]channel.Channel
	secondaryRepo *channel.Repository
	allRepos      *map[string]interface{}
}

// NewChannelRepository returns a new in memory cache implementation of channel.Repository.
// The database implementation of channel.Repository must be passed as the first argument
// since to simplify logic, cache repos wrap the database repos.
// A map of all the other cache based implementations of the Repository interfaces
// found in the different services of the project must be passed as a second argument as
// the Repository might make use of them to fetch objects instead of implementing redundant logic.
func NewChannelRepository(dbRepo *channel.Repository, allRepos *map[string]interface{}) channel.Repository {
	return &ChannelRepository{make(map[string]channel.Channel, 100), dbRepo, allRepos}
}

// cacheChannel is just a helper function
func (repo *ChannelRepository) cacheChannel(channelUsername string) error {
	c, err := (*repo.secondaryRepo).GetChannel(channelUsername)
	if err != nil {
		return err
	}

	repo.cache[channelUsername] = *c

	return err
}

// AddChannel takes in a channel.Channel struct and persists it.
// Returns an error if the DB repository implementation returns an error.
func (repo *ChannelRepository) AddChannel(channel *channel.Channel) (*channel.Channel, error) {
	return (*repo.secondaryRepo).AddChannel(channel)
}

// GetChannel retrieves a channel.Channel based on the channelUsername passed.
func (repo *ChannelRepository) GetChannel(channelUsername string) (*channel.Channel, error) {
	if _, ok := repo.cache[channelUsername]; ok == false {

		err := repo.cacheChannel(channelUsername)
		if err != nil {
			return nil, err
		}
	}

	c := repo.cache[channelUsername]
	return &c, nil
}

// UpdateChannel updates a channel based on the passed channel.Channel struct.
func (repo *ChannelRepository) UpdateChannel(channelUsername string, c *channel.Channel) (*channel.Channel, error) {
	c, err := (*repo.secondaryRepo).UpdateChannel(channelUsername, c)
	if err == nil {
		if c.ChannelUsername != "" {
			err = repo.cacheChannel(c.ChannelUsername)
			if err != nil {
				return nil, err
			}
		} else {
			delete(repo.cache, c.ChannelUsername)
			err = repo.cacheChannel(channelUsername)
			if err != nil {
				return nil, err
			}
		}
	}
	return c, err
}

// DeleteChannel deletes a channel based on the passed channelUsername.
func (repo *ChannelRepository) DeleteChannel(channelUsername string) error {
	err := (*repo.secondaryRepo).DeleteChannel(channelUsername)
	if err == nil {
		delete(repo.cache, channelUsername)
	}
	return err
}

// SearchChannel calls the DB repo SearchChannel function.
// It also caches all the channels returned by the result.
func (repo *ChannelRepository) SearchChannels(pattern string, sortBy channel.SortBy, sortOrder channel.SortOrder, limit, offset int) ([]*channel.Channel, error) {
	result, err := (*repo.secondaryRepo).SearchChannels(pattern, sortBy, sortOrder, limit, offset)
	if err == nil {
		for _, c := range result {
			cTemp := *c
			repo.cache[c.ChannelUsername] = cTemp
		}
	}
	return result, err
}

// AddAdmin calls the DB repo AddAdmin function.
func (repo *ChannelRepository) AddAdmin(channelUsername string, adminUsername string) error {
	err := (*repo.secondaryRepo).AddAdmin(channelUsername, adminUsername)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// DeleteAdmin calls the DB repo DeleteAdmin function.
func (repo *ChannelRepository) DeleteAdmin(channelUsername string, adminUsername string) error {
	err := (*repo.secondaryRepo).DeleteAdmin(channelUsername, adminUsername)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// ChangeOwner calls the DB repo ChangeOwner function.
func (repo *ChannelRepository) ChangeOwner(channelUsername string, ownerUsername string) error {
	err := (*repo.secondaryRepo).ChangeOwner(channelUsername, ownerUsername)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// AddReleaseToOfficialCatalog calls the DB repo AddReleaseToOfficialCatalog function.
func (repo *ChannelRepository) AddReleaseToOfficialCatalog(channelUsername string, releaseID uint, postID uint) error {
	err := (*repo.secondaryRepo).AddReleaseToOfficialCatalog(channelUsername, releaseID, postID)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// DeleteReleaseFromCatalog calls the DB repo DeleteReleaseFromCatalog function.
func (repo *ChannelRepository) DeleteReleaseFromCatalog(channelUsername string, ReleaseID uint) error {
	err := (*repo.secondaryRepo).DeleteReleaseFromCatalog(channelUsername, ReleaseID)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// DeleteReleaseFromOfficialCatalog calls the DB repo DeleteReleaseFromOfficialCatalog function.
func (repo *ChannelRepository) DeleteReleaseFromOfficialCatalog(channelUsername string, ReleaseID uint) error {
	err := (*repo.secondaryRepo).DeleteReleaseFromOfficialCatalog(channelUsername, ReleaseID)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// StickyPost calls the DB repo StickyPost function.
func (repo *ChannelRepository) StickyPost(channelUsername string, postID uint) error {
	err := (*repo.secondaryRepo).StickyPost(channelUsername, postID)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// DeleteStickiedPost calls the DB repo DeleteStickiedPost function.
func (repo *ChannelRepository) DeleteStickiedPost(channelUsername string, stickiedPostID uint) error {
	err := (*repo.secondaryRepo).DeleteStickiedPost(channelUsername, stickiedPostID)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}

// AddPicture calls the same method on the wrapped repo with a lil caching in between.
func (repo *ChannelRepository) AddPicture(channelUsername, name string) (string, error) {
	a, err := (*repo.secondaryRepo).AddPicture(channelUsername, name)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return "", err
		}
	}
	return a, err
}

// RemovePicture calls the same method on the wrapped repo with a lil caching in between.
func (repo *ChannelRepository) RemovePicture(channelUsername string) error {
	err := (*repo.secondaryRepo).RemovePicture(channelUsername)
	if err == nil {
		err = repo.cacheChannel(channelUsername)
		if err != nil {
			return err
		}
	}
	return err
}
