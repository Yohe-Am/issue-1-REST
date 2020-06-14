package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/comment"
	"github.com/lib/pq"
)

type commentRepository repository

// NewCommentRepository returns a struct that implements the comment.Repository using
// a PostgresSQL database.
// A database connection needs to be passed so that it can function.
func NewCommentRepository(DB *sql.DB, allRepos *map[string]interface{}) comment.Repository {
	return &commentRepository{DB, allRepos}
}

// AddComment persists the given struct into the database.
func (repo commentRepository) AddComment(c *comment.Comment) (*comment.Comment, error) {
	query := `INSERT INTO comments (post_from, reply_to, content, commented_by)
				VALUES ($1, $2, $3, $4)
				RETURNING id`
	err := repo.db.QueryRow(query, c.OriginPost, c.ReplyTo, c.Content, c.Commenter).Scan(&c.ID)
	if err != nil {
		if pgErr, isPGErr := err.(pq.Error); !isPGErr {
			switch pgErr.Column {
			case "post_from":
				return nil, comment.ErrPostNotFound
			case "commented_by":
				return nil, comment.ErrUserNotFound
			default:
				return nil, fmt.Errorf("insertion of user failed because of: %s", err.Error())
			}
		}
		return nil, fmt.Errorf("insertion of comment failed because of: %v", err)
	}
	return c, nil
}

// GetComment returns a comment.Comment under the given id from the database.
func (repo commentRepository) GetComment(id int) (*comment.Comment, error) {
	var err error
	var c = new(comment.Comment)

	query := `SELECT post_from,commented_by,content,reply_to,creation_time
				FROM comments
				WHERE id = $1`
	err = repo.db.QueryRow(query, id).Scan(&c.OriginPost, &c.Commenter, &c.Content, &c.ReplyTo, &c.CreationTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, comment.ErrCommentNotFound
		}
		return nil, fmt.Errorf("unable to get comment from db becaues: %v", err)
	}
	c.ID = id
	return c, nil
}

// GetComments returns all comments in the database that match the given post
// id.
func (repo commentRepository) GetComments(postID int, by string, order string, limit, offset int) ([]*comment.Comment, error) {
	{ // block checks if post exits
		var found bool
		err := repo.db.QueryRow(`
				SELECT EXISTS(
               SELECT *
               FROM posts
               WHERE id = $1)`, postID).Scan(&found)
		if err != nil {
			return nil, fmt.Errorf("unable to check if post exists")
		}
		if !found {
			return nil, comment.ErrPostNotFound
		}
	}
	var comments = make([]*comment.Comment, 0)
	var err error
	var rows *sql.Rows
	query := fmt.Sprintf(`SELECT id,commented_by,content,reply_to,creation_time
			FROM comments
			WHERE post_from = $1
			ORDER BY %s %s
			LIMIT $2 OFFSET $3`, by, order)
	rows, err = repo.db.Query(query, postID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying for comments failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		c := new(comment.Comment)
		// TODO check if type casting works
		err := rows.Scan(&c.ID, &c.Commenter, &c.Content, &c.ReplyTo, &c.CreationTime)
		if err != nil {
			return nil, fmt.Errorf("scanning from row failed because: %v", err)
		}
		c.OriginPost = postID

		comments = append(comments, c)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return comments, nil
}

// GetComments returns all comments in the database that match the given reply_to
// id.
func (repo commentRepository) GetReplies(commentID int, by string, order string, limit, offset int) ([]*comment.Comment, error) {
	{ // block checks if root comment exits
		var found bool
		err := repo.db.QueryRow(`
				SELECT EXISTS(
               SELECT *
               FROM comments
               WHERE id = $1)`, commentID).Scan(&found)
		if err != nil {
			return nil, fmt.Errorf("unable to check if comment exists")
		}
		if !found {
			return nil, comment.ErrCommentNotFound
		}
	}
	var comments = make([]*comment.Comment, 0)
	var err error
	var rows *sql.Rows
	query := fmt.Sprintf(`SELECT id,commented_by,content,post_from,creation_time
			FROM comments
			WHERE reply_to = $1
			ORDER BY %s %s
			LIMIT $2 OFFSET $3`, by, order)
	rows, err = repo.db.Query(query, commentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying for comments failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		c := new(comment.Comment)
		// TODO check if type casting works
		err := rows.Scan(&c.ID, &c.Commenter, &c.Content, &c.OriginPost, &c.CreationTime)
		if err != nil {
			return nil, fmt.Errorf("scanning from row failed because: %v", err)
		}
		c.ReplyTo = commentID

		comments = append(comments, c)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return comments, nil
}

// UpdateComment updates a comment in the database according to the given struct.
func (repo commentRepository) UpdateComment(c *comment.Comment) (*comment.Comment, error) {
	var errs []error

	if c.Content != "" {
		query := `UPDATE comments
				SET content = $1 
				WHERE id = $2`
		_, err := repo.db.Exec(query, c.Content, c.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	const maxNoOfPossibleErr = 1
	if len(errs) == maxNoOfPossibleErr {
		return nil, fmt.Errorf("was unable to update any data because of %v", errs)
	}
	c, err := repo.GetComment(c.ID)
	if err == nil {
		if len(errs) > 0 {
			err = comment.ErrSomeCommentDataNotPersisted
		}
	}
	return c, err
}

// DeleteComment removes the comment under the given id from the database.
func (repo commentRepository) DeleteComment(id int) error {
	_, err := repo.db.Exec(`DELETE FROM comments
							WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deletion of comment failed because of: %v", err)
	}
	return nil
}
