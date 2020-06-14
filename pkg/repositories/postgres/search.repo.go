package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/search"
)

type searchRepository repository

// NewSearchRepository returns a struct that implements the search.Repository using
// a PostgresSQL database.
// A database connection needs to be passed so that it can function.
func NewSearchRepository(DB *sql.DB, allRepos *map[string]interface{}) search.Repository {
	return &searchRepository{DB, allRepos}
}

func (repo searchRepository) SearchComments(pattern string, by string, order string, limit, offset int) ([]*search.Comment, error) {

	var comments = make([]*search.Comment, 0)
	var err error
	var rows *sql.Rows
	query := `
				SELECT id, commented_by, content, reply_to, creation_time
				FROM (
				         SELECT ts_rank(vector, query, 32) as rank, *
				         FROM (
				               (select comment_id as id, vector, query
				                from tsvs_comment,
				                     websearch_to_tsquery('english', $1) query
				                where vector @@ query
				               ) as rti
				                  NATURAL JOIN comments
				             )
				     ) as "c*"
				ORDER BY rank DESC`
	if by != "" {
		query = fmt.Sprintf(`%s, %s %s NULLS LAST`, query, by, order)
	}
	query = fmt.Sprintf(`%s 
				LIMIT $2 OFFSET $3`, query)
	rows, err = repo.db.Query(query, pattern, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying for comments failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		c := new(search.Comment)
		err := rows.Scan(&c.ID, &c.Commenter, &c.Content, &c.ReplyTo, &c.CreationTime)
		if err != nil {
			return nil, fmt.Errorf("scanning from row failed because: %v", err)
		}

		comments = append(comments, c)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return comments, nil
}
