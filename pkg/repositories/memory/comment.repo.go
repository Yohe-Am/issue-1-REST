package memory

import (
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/comment"
)

type commentRepository struct {
	cache         map[int]comment.Comment
	secondaryRepo *comment.Repository
}

// NewCommentRepository returns a struct that implements the comment.Repository using
// a cached based implementation.
// A database implementation of the same interface needs to be passed so that it can be
// consulted when the caches aren't enough.
func NewCommentRepository(secondaryRepo *comment.Repository) comment.Repository {
	return &commentRepository{make(map[int]comment.Comment), secondaryRepo}
}

// AddComment calls the same method on the wrapped repo with a little caching in between.
func (repo *commentRepository) AddComment(c *comment.Comment) (*comment.Comment, error) {
	c, err := (*repo.secondaryRepo).AddComment(c)
	if err == nil {
		repo.cache[c.ID] = *c
	}
	return c, err
}

// GetComment returns the comment under the given id from the cache ,if found,
// or from the the wrapped repository,
func (repo *commentRepository) GetComment(id int) (*comment.Comment, error) {
	if c, ok := repo.cache[id]; ok == false {
		var err error
		c, err := (*repo.secondaryRepo).GetComment(id)
		if err != nil {
			return nil, err
		}
		repo.cache[id] = *c
		return c, nil
	} else {
		return &c, nil
	}
}

// GetComments calls the same method on the wrapped repo with a little caching in between.
func (repo *commentRepository) GetComments(postID int, by string, order string, limit, offset int) ([]*comment.Comment, error) {
	result, err := (*repo.secondaryRepo).GetComments(postID, by, order, limit, offset)
	if err == nil {
		for _, c := range result {
			repo.cache[c.ID] = *c
		}
	}
	return result, err
}

// GetReplies calls the same method on the wrapped repo with a little caching in between.
func (repo *commentRepository) GetReplies(commentID int, by string, order string, limit, offset int) ([]*comment.Comment, error) {
	result, err := (*repo.secondaryRepo).GetReplies(commentID, by, order, limit, offset)
	if err == nil {
		for _, c := range result {
			repo.cache[c.ID] = *c
		}
	}
	return result, err
}

// UpdateComment calls the same method on the wrapped repo with a little caching in between.
func (repo *commentRepository) UpdateComment(c *comment.Comment) (*comment.Comment, error) {
	c, err := (*repo.secondaryRepo).UpdateComment(c)
	if err == nil {
		repo.cache[c.ID] = *c
	}
	return c, err
}

// DeleteRelease calls the same method on the wrapped repo while also cleaning the cache
// when appropriate.
func (repo *commentRepository) DeleteComment(id int) error {
	err := (*repo.secondaryRepo).DeleteComment(id)
	if err == nil {
		delete(repo.cache, id)
	}
	return err
}
