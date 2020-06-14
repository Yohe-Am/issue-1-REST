package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/feed"
	"github.com/lib/pq"
)

// feedRepository ...
type feedRepository repository

// NewFeedRepository returns a new in PostgreSQL implementation of user.Repository.
// the database connection must be passed as the first argument
// since for the repo to work.
func NewFeedRepository(db *sql.DB, allRepos *map[string]interface{}) feed.Repository {
	return &feedRepository{db: db, allRepos: allRepos}
}

// AddFeed persists a feed entity to the DB according to the feed.Feed struct passed in.
func (repo *feedRepository) AddFeed(f *feed.Feed) error {
	var err error
	var sorting string
	switch f.Sorting {
	case feed.SortHot:
		sorting = "hot"
	case feed.SortNew:
		sorting = "new"
	default:
		sorting = "top"
	}
	result, err := repo.db.Exec(`INSERT INTO feeds (owner_username,sorting)
										VALUES ($1, $2)`, f.OwnerUsername, sorting)
	if err != nil {
		return fmt.Errorf("insertion of user failed because of: %s", err.Error())
	}

	id64, err := result.LastInsertId()
	if err == nil {
		return fmt.Errorf("unable to get LastInsertId")
	}
	username := f.OwnerUsername
	f.OwnerUsername = ""
	id := uint(id64)
	err = repo.UpdateFeed(id, f)
	if err != nil {
		return fmt.Errorf("the feed was created for user %s but some data wasn't persisted because of: %s", username, err.Error())
	}
	return nil
}

// GetFeed retrieve the feed entity in the database belonging to the user of the passed
// in username.
func (repo *feedRepository) GetFeed(username string) (*feed.Feed, error) {
	var err error
	f := feed.Feed{OwnerUsername: username}
	var sorting string
	err = repo.db.QueryRow(`SELECT id, sorting
	 								FROM feeds
	 								WHERE owner_username = $1`, username).Scan(&f.ID, &sorting)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, feed.ErrFeedNotFound
		}
		return nil, fmt.Errorf("unable to get feed from db becaues: %v", err)
	}
	switch sorting {
	case "hot":
		f.Sorting = feed.SortHot
	case "new":
		f.Sorting = feed.SortNew
	default:
		f.Sorting = feed.SortTop
	}
	return &f, nil
}

// GetChannels retrieves the all the channels the given feed has subscribed to.
func (repo *feedRepository) GetChannels(f *feed.Feed, sortBy string, sortOrder string) ([]*feed.Channel, error) {
	channelSubscriptions := make([]*feed.Channel, 0)
	rows, err := repo.db.Query(fmt.Sprintf(`
		SELECT username, name, subscription_time
		FROM (
			(
				SELECT channel_username, subscription_time
				FROM feed_subscriptions
				WHERE feed_id = $1
			) AS S (username, subscription_time)
			NATURAL JOIN
			channels
		)
		ORDER BY %s %s NULLS LAST`, sortBy, sortOrder), f.ID)
	if err != nil {
		return nil, fmt.Errorf("querying for feed_subscriptions failed because of: %s", err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		c := new(feed.Channel)
		err := rows.Scan(&c.Channelname, &c.Name, &c.SubscriptionTime)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %s", err.Error())
		}
		channelSubscriptions = append(channelSubscriptions, c)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %s", err.Error())
	}
	return channelSubscriptions, nil
}

// GetPosts returns a list of posts collected from the channels
// the given feed has subscribed to sorted according to the given
// method.
func (repo *feedRepository) GetPosts(f *feed.Feed, sort feed.Sorting, limit, offset int) ([]*feed.Post, error) {
	var err error

	var rows *sql.Rows
	switch sort {
	// TODO test queries with actual posts
	case feed.SortNew:
		rows, err = repo.db.Query(`
		SELECT id
		FROM	(SELECT id, creation_time
				      FROM (
				               SELECT channel_username
				               FROM feed_subscriptions
				               WHERE feed_id = $1
				           ) AS C (channel_from)
				               NATURAL JOIN
				           posts
				     ) AS P
		ORDER BY creation_time DESC NULLS LAST LIMIT $2 OFFSET $3`, f.ID, limit, offset)
	case feed.SortHot:
		rows, err = repo.db.Query(`
		SELECT post_id
		FROM(SELECT LP.post_id, comment_count
			FROM(
			    SELECT *
				FROM (SELECT id, creation_time
				      FROM (
				               SELECT channel_username
				               FROM feed_subscriptions
				               WHERE feed_id = $1
				           ) AS C (channel_from)
				               NATURAL JOIN
				           posts
				     ) AS P
				ORDER BY creation_time DESC NULLS LAST
				) AS LP (post_id)
				LEFT JOIN
				(SELECT post_from, COALESCE(COUNT(*), 0)
				 FROM comments
				 GROUP BY post_from
				) AS PS (post_id, comment_count) ON LP.post_id = PS.post_id
			ORDER BY creation_time DESC
		) AS F ORDER BY comment_count DESC NULLS LAST 
		LIMIT $2 OFFSET $3`, f.ID, limit, offset)
	case feed.NotSet:
		fallthrough
	case feed.SortTop:
		fallthrough
	default:
		rows, err = repo.db.Query(`
		SELECT post_id
		FROM(SELECT Lp.post_id, total_star_count
			FROM(
			    SELECT *
				FROM (SELECT id, creation_time
				      FROM (
				               SELECT channel_username
				               FROM feed_subscriptions
				               WHERE feed_id = $1
				           ) AS C (channel_from)
				               NATURAL JOIN
				           posts
				     ) AS P
				ORDER BY creation_time DESC NULLS LAST
				) AS LP (post_id)
				LEFT JOIN
				(SELECT post_id,SUM(star_count)
				 FROM post_stars
				 GROUP BY post_id
				) AS PS (post_id, total_star_count) ON LP.post_id = PS.post_id
			ORDER BY creation_time DESC
		) AS F ORDER BY total_star_count DESC NULLS LAST 
		LIMIT $2 OFFSET $3`, f.ID, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("querying for feed_subscriptions failed because of: %s", err.Error())
	}
	defer rows.Close()

	var id int
	posts := make([]*feed.Post, 0)

	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %s", err.Error())
		}
		posts = append(posts, &feed.Post{ID: id})
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %s", err.Error())
	}

	return posts, nil
}

// UpdateFeed updates the feed entity under the given id based on the passed in feed.Feed struct.
func (repo *feedRepository) UpdateFeed(id uint, f *feed.Feed) error {
	var sorting string
	switch f.Sorting {
	case feed.SortHot:
		sorting = "hot"
	case feed.SortNew:
		sorting = "new"
	default:
		sorting = "top"
	}
	/*
		if f.OwnerUsername != "" {
			err = repo.execUpdateStatementOnColumn("owner_username", f.OwnerUsername, id)
			if err != nil {
				return err
			}
			// change username for subsequent calls if username changed
		}
	*/
	err := repo.execUpdateStatementOnColumn("sorting", sorting, id)
	if err != nil {
		return err
	}
	return nil
}

// execUpdateStatementOnColumn is just a helper function
func (repo *feedRepository) execUpdateStatementOnColumn(column, value string, id uint) error {
	_, err := repo.db.Exec(fmt.Sprintf(`
			UPDATE feeds 
			SET %s = $1 
			WHERE id = $2`, column), value, id)
	if err != nil {
		return fmt.Errorf("updating failed of %s column with %s because of: %w", column, value, err)
	}
	return nil
}

// Subscribe adds the given channel to the list of channels that the feed collects posts from.
func (repo *feedRepository) Subscribe(f *feed.Feed, channelname string) error {
	_, err := repo.db.Exec(`
		INSERT INTO feed_subscriptions (feed_id,channel_username)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		`, f.ID, channelname)
	const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
	if err != nil {
		if pgErr, isPGErr := err.(pq.Error); !isPGErr {
			if pgErr.Code != foreignKeyViolationErrorCode {
				return feed.ErrChannelNotFound
			}
			return fmt.Errorf("insertion of user failed because of: %s", err.Error())
		}
	}
	return nil
}

// Unsubscribe removes the channel to the list of channels that the feed collects posts from.
func (repo *feedRepository) Unsubscribe(f *feed.Feed, channelname string) error {
	_, err := repo.db.Exec(`
		DELETE FROM feed_subscriptions
		WHERE feed_id = $1 AND channel_username = $2`, f.ID, channelname)
	if err != nil {
		return fmt.Errorf("deletion of user failed because of: %s", err.Error())
	}
	return nil
}
