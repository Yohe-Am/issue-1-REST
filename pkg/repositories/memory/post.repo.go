package memory

import (
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/post"
)

//postRepository ...
type postRepository struct {
	cache         map[uint]post.Post
	secondaryRepo *post.Repository
}

// NewPostRepository returns a struct that implements the post.Repository using
func NewPostRepository(secondaryRepo *post.Repository) post.Repository {
	return &postRepository{cache: make(map[uint]post.Post), secondaryRepo: secondaryRepo}
}

// cachePost is just a helper function to update the cache with new states of the struct
func (repo *postRepository) cachePost(id uint) error {
	u, err := (*repo.secondaryRepo).GetPost(id)
	if err != nil {
		return err
	}
	repo.cache[id] = *u
	return nil
}

// GetPost gets the Post stored under the given id.
func (repo *postRepository) GetPost(id uint) (*post.Post, error) {

	if _, ok := repo.cache[id]; ok == false {
		r, err := (*repo.secondaryRepo).GetPost(id)
		if err != nil {
			return nil, err
		}
		repo.cache[id] = *r
	}
	r := repo.cache[id]
	return &r, nil

}

// DeletePost Deletes the Post stored under the given id.
func (repo *postRepository) DeletePost(id uint) error {
	_, found := repo.cache[id]
	if !found {
		return post.ErrPostNotFound
	}
	err := (*repo.secondaryRepo).DeletePost(id)
	if err == nil {
		delete(repo.cache, id)
	}
	return err
}

// AddPost Adds the Post stored under the given id.
func (repo *postRepository) AddPost(p *post.Post) (*post.Post, error) {
	p, err := (*repo.secondaryRepo).AddPost(p)
	if err == nil {
		repo.cache[p.ID] = *p
	}
	return p, err
}

//UpdatePost updates the post with given id and post struct
func (repo *postRepository) UpdatePost(pos *post.Post, id uint) (*post.Post, error) {
	_, found := repo.cache[id]
	if !found {
		return nil, post.ErrPostNotFound
	}
	p, err := (*repo.secondaryRepo).UpdatePost(pos, id)
	if err == nil {
		repo.cache[p.ID] = *p
	}
	return p, err
}

// SearchPost gets all Posts under specfications
func (repo *postRepository) SearchPost(pattern string, by post.SortBy, order post.SortOrder, limit int, offset int) ([]*post.Post, error) {
	pos, err := (*repo.secondaryRepo).SearchPost(pattern, by, order, limit, offset)
	if err == nil {
		for _, p := range pos {
			repo.cache[p.ID] = *p

		}
	}
	return pos, err
}

// GetPostStar gets the star stored under the given postid and username.
func (repo *postRepository) GetPostStar(id uint, username string) (*post.Star, error) {
	_, found := repo.cache[id]
	if !found {
		return nil, post.ErrPostNotFound
	}
	s, err := (*repo.secondaryRepo).GetPostStar(id, username)
	if err != nil {
		return nil, err
	}
	return s, nil
}

//DeletePostStar deletes the star stored under given postid and username
func (repo *postRepository) DeletePostStar(id uint, username string) error {
	_, found := repo.cache[id]
	if !found {
		return post.ErrPostNotFound
	}
	err := (*repo.secondaryRepo).DeletePostStar(id, username)
	if err != nil {
		return err
	}
	err = repo.cachePost(id)
	return err
}

//AddPostStar adds a star given postid, number of stars and username
func (repo *postRepository) AddPostStar(id uint, star *post.Star) (*post.Star, error) {
	_, found := repo.cache[id]
	if !found {
		return nil, post.ErrPostNotFound
	}
	s, err := (*repo.secondaryRepo).AddPostStar(id, star)
	if err != nil {
		return nil, err
	}
	errs := repo.cachePost(id)
	if errs != nil {
		return nil, err
	}
	return s, nil
}

//UpdatePostStar updates a star stored given postid, number of stars and username
func (repo *postRepository) UpdatePostStar(id uint, star *post.Star) (*post.Star, error) {
	_, found := repo.cache[id]
	if !found {
		return nil, post.ErrPostNotFound
	}
	s, err := (*repo.secondaryRepo).UpdatePostStar(id, star)
	if err != nil {
		return nil, err
	}
	errs := repo.cachePost(id)
	if errs != nil {
		return nil, err
	}
	return s, nil
}
