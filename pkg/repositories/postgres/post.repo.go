package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/post"
)

type postRepository repository

// NewPostRepository returns a struct that implements the release.Repository using
//a postgres database
func NewPostRepository(DB *sql.DB, allRepos *map[string]interface{}) post.Repository {
	return &postRepository{DB, allRepos}
}

// GetPost gets the Post stored under the given id.
func (repo *postRepository) GetPost(id uint) (*post.Post, error) {
	var err error
	var p = new(post.Post)

	err = repo.db.QueryRow(`
								SELECT COALESCE(posted_by, ''), COALESCE(channel_from, ''), COALESCE(title, ''), COALESCE(description, ''),creation_time
								FROM "issue#1".posts
								WHERE posts.id = $1`, id).Scan(&p.PostedByUsername, &p.OriginChannel, &p.Title, &p.Description, &p.CreationTime)
	if err != nil {
		//checkErr(err)
		return nil, post.ErrPostNotFound
	}
	ContentList, errr := repo.getContents(id)
	if errr != nil {
		return nil, errr
	}
	StarList, e := repo.getStars(id)
	if e != nil {
		return nil, e
	}
	CommentList, d := repo.getComments(id)
	if d != nil {
		return nil, fmt.Errorf("Comments Not found because of: %v", d)
	}

	p.ID = id
	p.ContentsID = ContentList
	p.CommentsID = CommentList
	p.Stars = StarList

	return p, nil

}
func (repo *postRepository) getContents(id uint) ([]uint, error) {

	var ContentList = []uint{}
	var (
		releaseID uint
	)

	rows, err := repo.db.Query(`SELECT release_id
								FROM "issue#1".post_contents
								WHERE post_id = $1`, id)
	if err != nil {
		//checkErr(err)
		return nil, fmt.Errorf("querying for post contents failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&releaseID)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		ContentList = append(ContentList, releaseID)

	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return ContentList, nil
}

func (repo *postRepository) getComments(id uint) ([]int, error) {

	var CommentList = []int{}
	var (
		commentID int
	)

	rows, err := repo.db.Query(`SELECT id
								FROM "issue#1".comments
								WHERE post_from = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("querying for post contents failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&commentID)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		CommentList = append(CommentList, commentID)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return CommentList, nil
}

func (repo *postRepository) getStars(id uint) (map[string]uint, error) {
	// TODO test this method
	var StarList = make(map[string]uint, 0)
	var (
		username  string
		starCount uint
	)

	rows, err := repo.db.Query(`SELECT username, star_count
								FROM "issue#1".post_stars 
								WHERE post_id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("querying for star list failed because of: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&username, &starCount)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		StarList[username] = starCount
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return StarList, nil
}

// DeletePost Deletes the Post stored under the given id.
func (repo *postRepository) DeletePost(id uint) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".posts
							WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deletion of post failed because of: %v", err)
	}
	return nil
}

// AddPost Adds the Post stored under its id from given post struct.
func (repo *postRepository) AddPost(p *post.Post) (*post.Post, error) {

	query := `INSERT INTO "issue#1".posts (posted_by,channel_from, title,description) 
				VALUES ($1,$2,$3,$4)
				RETURNING id`
	errs := repo.db.QueryRow(query, p.PostedByUsername, p.OriginChannel, p.Title, p.Description).Scan(&p.ID)
	if errs != nil {
		//checkErr(errs)
		return nil, post.ErrSomePostDataNotPersisted
	}
	p.PostedByUsername = ""
	p.OriginChannel = ""
	p.Title = ""
	p.Description = ""
	return repo.UpdatePost(p, p.ID)

}

//UpdatePost updates the post with given id and post struct
func (repo *postRepository) UpdatePost(pos *post.Post, id uint) (*post.Post, error) {
	var errs []error

	if pos.PostedByUsername != "" {
		err := repo.execUpdateStatementOnColumnIntoPost("posted_by", pos.PostedByUsername, id)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if pos.OriginChannel != "" {
		err := repo.execUpdateStatementOnColumnIntoPost("channel_from", pos.OriginChannel, id)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if pos.Title != "" {
		err := repo.execUpdateStatementOnColumnIntoPost("title", pos.Title, id)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if pos.Description != "" {
		err := repo.execUpdateStatementOnColumnIntoPost("description", pos.Description, id)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(pos.ContentsID) != 0 {
		err := repo.execUpdateStatementOnColumnIntoContents("release_id", pos.ContentsID, id)
		if err != nil {
			errs = append(errs, err)
		}
	}
	const maxNoOfPossibleErr = 5
	if len(errs) == maxNoOfPossibleErr {
		return nil, fmt.Errorf("was unable to update any data because of %v", errs)
	}

	p, d := repo.GetPost(id)
	if d == nil {
		if len(errs) > 0 {

			d = post.ErrSomePostDataNotPersisted
		}
	} else {
		d = post.ErrPostNotFound
	}
	fmt.Printf("%+v", p)
	return p, d
}

func (repo *postRepository) execUpdateStatementOnColumnIntoPost(column string, value string, id uint) error {
	query := fmt.Sprintf(`UPDATE posts
								SET %s = $1 
								WHERE id = $2`, column)
	_, err := repo.db.Exec(query, value, id)
	if err != nil {
		return fmt.Errorf("updating failed of %s column with %s because of: %v", column, value, err)
	}
	return nil
}
func (repo *postRepository) execUpdateStatementOnColumnIntoContents(column string, value []uint, id uint) error {
	query := `INSERT INTO post_contents values($2, $1);`
	for _, v := range value {
		_, err := repo.db.Exec(query, v, id)
		if err != nil {
			return fmt.Errorf("updating failed of %s column with %d because of: %v", column, v, err)
		}
	}

	return nil
}
func (repo *postRepository) execUpdateStatementOnColumnIntoStars(value map[string]int, id uint) error {
	query := `UPDATE post_stars
				SET username = $1, star_count=$2
				WHERE post_id = $3`
	for k, v := range value {
		_, err := repo.db.Exec(query, k, v, id)
		if err != nil {
			return fmt.Errorf("updating failed of username-%s starcount-%d column of postid %d because of: %v", k, v, id, err)
		}
	}

	return nil
}

// SearchPost gets all Posts under specfications
func (repo *postRepository) SearchPost(pattern string, by post.SortBy, order post.SortOrder, limit int, offset int) ([]*post.Post, error) {
	var posts = make([]*post.Post, 0)
	var err error
	var rows *sql.Rows
	var query string
	if pattern == "" {
		query = fmt.Sprintf(`
		SELECT id, COALESCE(posted_by, ''), COALESCE(channel_from, ''), COALESCE(title, ''), COALESCE(description, ''),creation_time
		FROM "issue#1".posts
		ORDER BY %s %s NULLS LAST
		LIMIT $1 OFFSET $2`, by, order)
		rows, err = repo.db.Query(query, limit, offset)
	} else {
		query = `
			  SELECT id,
			   posted_by,
			   channel_from,
			   title,
			   COALESCE(description, ''),
			   creation_time
		FROM (
				 SELECT ts_rank(vector, query) as rank, *
				 FROM (
						  select post_id as id, vector, query
						  from tsvs_posts,
							   websearch_to_tsquery('english', $1) query
						  where vector @@ query
					  ) as rti
						  NATURAL JOIN
					  posts
			 ) as "r*"
		ORDER BY rank DESC`
		if by != "" {
			query = fmt.Sprintf(`%s, %s %s NULLS LAST`, query, by, order)
		}
		query = fmt.Sprintf(`%s 
		  LIMIT $2 OFFSET $3`, query)
		rows, err = repo.db.Query(query, pattern, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("querying for posts failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		p := post.Post{}
		err := rows.Scan(&p.ID, &p.PostedByUsername, &p.OriginChannel, &p.Title, &p.Description, &p.CreationTime)
		if err != nil {
			return nil, post.ErrPostNotFound
		}
		ContentList, errr := repo.getContents(p.ID)
		if errr != nil {
			return nil, err
		}
		StarList, e := repo.getStars(p.ID)
		if e != nil {
			return nil, e
		}
		CommentList, d := repo.getComments(p.ID)
		if d != nil {
			return nil, fmt.Errorf("Comments Not found because of: %v", d)
		}

		p.ContentsID = ContentList
		p.CommentsID = CommentList
		p.Stars = StarList

		posts = append(posts, &p)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return posts, nil

}

// GetPostStar gets the star stored under the given postid and username.
func (repo *postRepository) GetPostStar(id uint, username string) (*post.Star, error) {
	s := post.Star{}

	err := repo.db.QueryRow(`SELECT username, star_count
								FROM "issue#1".post_stars 
								WHERE post_id = $1 AND username=$2`, id, username).Scan(&s.Username, &s.NumOfStars)
	if err != nil {
		//checkErr(err)
		return nil, post.ErrStarNotFound
	}
	return &s, nil

}

//DeletePostStar deletes the star stored under given postid and username
func (repo *postRepository) DeletePostStar(id uint, username string) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".post_stars
							WHERE post_id = $1 AND username=$2`, id, username)
	if err != nil {
		//checkErr(err)
		return post.ErrStarNotFound
	}
	return nil
}

//AddPostStar adds a star given postid, number of stars and username
func (repo *postRepository) AddPostStar(id uint, star *post.Star) (*post.Star, error) {
	query := `INSERT INTO "issue#1".post_stars (post_id,username, star_count) 
				VALUES ($1,$2,$3)`
	_, errs := repo.db.Exec(query, id, star.Username, star.NumOfStars)
	if errs != nil {
		//checkErr(errs)
		return nil, post.ErrStarNotFound
	}
	return repo.GetPostStar(id, star.Username)
}

//UpdatePostStar updates a star stored given postid, number of stars and username
func (repo *postRepository) UpdatePostStar(id uint, star *post.Star) (*post.Star, error) {
	query := `UPDATE "issue#1".post_stars
								SET star_count=$2 
								WHERE post_id = $3 AND username = $1`
	_, errs := repo.db.Exec(query, star.Username, star.NumOfStars, id)
	if errs != nil {

		return nil, post.ErrStarNotFound
	}
	return repo.GetPostStar(id, star.Username)
}

// func checkErr(errs error) {
// 	if pgErr, isPGErr := errs.(pq.Error); !isPGErr {
// 		fmt.Printf("prin\n%v", pgErr)
// 	}
// 	fmt.Printf("error here")
// }
