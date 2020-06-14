package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/channel"
	"github.com/lib/pq"
	"time"
)

//channelRepository...
type channelRepository repository

// NewChannelRepository returns a new in PostgreSQL implementation of channel.Repository.
// the database connection must be passed as the first argument
// since for the repo to work.
// A map of all the other PostgreSQL based implementations of the Repository interfaces
// found in the different services of the project must be passed as a second argument as
// the Repository might make use of them to fetch objects instead of implementing redundant logic.
//Each none helper function if successful will try to cache.
func NewChannelRepository(DB *sql.DB, allRepos *map[string]interface{}) channel.Repository {
	return &channelRepository{DB, allRepos}
}

// AddChannel takes in a channel.Channel struct and persists it in the database.
func (repo *channelRepository) AddChannel(c *channel.Channel) (*channel.Channel, error) {
	var err error

	_, err = repo.db.Exec(`INSERT INTO "issue#1".channels (username, name, description)
							VALUES ($1, $2, $3)`, c.ChannelUsername, c.Name, c.Description)

	if err != nil {
		return nil, fmt.Errorf("insertion of channel failed because of: %s", err.Error())
	}
	err = repo.AddAdmin(c.ChannelUsername, c.OwnerUsername)
	if err != nil {
		return nil, fmt.Errorf("insertion of admin user failed because of: %s", err.Error())
	}
	err = repo.ChangeOwner(c.ChannelUsername, c.OwnerUsername)
	if err != nil {
		return nil, fmt.Errorf("insertion of admin user failed because of: %s", err.Error())
	}

	return c, nil
}

// GetChannel retrieves a channel.Channel user.User based on the username passed.
func (repo *channelRepository) GetChannel(channelUsername string) (*channel.Channel, error) {
	var err error
	var c = new(channel.Channel)

	var creationTimeString string
	err = repo.db.QueryRow(`SELECT name, COALESCE(description, ''), creation_time
							FROM "issue#1".channels
							WHERE username = $1`, channelUsername).Scan(&c.Name, &c.Description, &creationTimeString)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, channel.ErrChannelNotFound
		}
		return nil, fmt.Errorf("... %v", err)
	}

	creationTime, err := time.Parse(time.RFC3339, creationTimeString)
	if err != nil {
		return nil, fmt.Errorf("parsing of timestamp to time.Time failed because of: %s", err.Error())
	}
	c.CreationTime = creationTime

	admins, err := repo.GetAdmins(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get admins because of: %s", err.Error())
	}
	owner, err := repo.GetOwner(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get owner because of: %s", err.Error())
	}
	stickiedPosts, err := repo.GetStickiedPost(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get bookmarked posts because of: %s", err.Error())
	}
	posts, err := repo.GetPosts(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get posts because of: %s", err.Error())
	}
	unOfficialReleases, err := repo.GetUnOfficialRelease(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get UnOfficialRelease because of: %s", err.Error())
	}
	officialReleases, err := repo.GetOfficialRelease(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get UnOfficialRelease because of: %s", err.Error())
	}
	pictureURL, err := repo.GetPicture(channelUsername)
	if err != nil {
		return nil, fmt.Errorf("unable to get Picture because of: %s", err.Error())
	}
	c.AdminUsernames = admins
	c.OwnerUsername = owner
	c.StickiedPostIDs = stickiedPosts
	c.PostIDs = posts
	c.ReleaseIDs = unOfficialReleases
	c.OfficialReleaseIDs = officialReleases
	c.PictureURL = pictureURL
	c.ChannelUsername = channelUsername
	return c, nil
}

// UpdateChannel updates a channel based on the passed in channel.Channel struct into channelUsername.
func (repo *channelRepository) UpdateChannel(channelUsername string, c *channel.Channel) (*channel.Channel, error) {
	var err error

	if c.Name != "" {
		err = repo.execUpdateStatementOnColumn("name", c.Name, channelUsername)
		if err != nil {
			return nil, err
		}
	}
	if c.ChannelUsername != "" {
		err = repo.execUpdateStatementOnColumn("username", c.ChannelUsername, channelUsername)
		if err != nil {
			return nil, err
		}
	}
	if c.Description != "" {
		err = repo.execUpdateStatementOnColumn("description", c.Description, channelUsername)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// execUpdateStatementOnColumn is just a helper function that updates a certain column
func (repo *channelRepository) execUpdateStatementOnColumn(column, value, username string) error {
	_, err := repo.db.Exec(fmt.Sprintf(`UPDATE "issue#1".channels 
									SET %s = $1 
									WHERE username = $2`, column), value, username)
	if err != nil {
		return fmt.Errorf("updating failed of %s column with %s because of: %s", column, value, err.Error())
	}
	return nil
}

// DeleteChannel deletes a channel based on the passed in channelUsername.
func (repo *channelRepository) DeleteChannel(channelUsername string) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".channels
							WHERE username = $1`, channelUsername)
	if err != nil {
		return fmt.Errorf("deletion of tuple from channels because of: %s", err.Error())
	}
	return nil
}

// GetUnOfficialRelease is just a helper function that gets the UnOfficial Release List
func (repo *channelRepository) GetUnOfficialRelease(username string) ([]uint, error) {
	var UnOfficialList []uint
	var Release uint

	rows, err := repo.db.Query(`SELECT id
                FROM "issue#1".releases
                WHERE owner_channel = $1`, username)
	if err != nil {
		return nil, fmt.Errorf("querying for unofficial catalog failed because of: %v", err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		err := rows.Scan(&Release)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}

		UnOfficialList = append(UnOfficialList, Release)

		i++
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return UnOfficialList, nil
}

// GetOfficialRelease is just a helper function that gets the Official Release List
func (repo *channelRepository) GetOfficialRelease(username string) ([]uint, error) {
	var OfficialList []uint
	var Release uint

	rows, err := repo.db.Query(`SELECT release_id
                FROM "issue#1".channel_official_catalog
                WHERE channel_username = $1`, username)
	if err != nil {
		return nil, fmt.Errorf("querying for official catalog failed because of: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&Release)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}

		OfficialList = append(OfficialList, Release)

	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return OfficialList, nil
}

// GetPosts is just a helper function that gets the Post List
func (repo *channelRepository) GetPosts(username string) ([]uint, error) {
	var PostList []uint
	var Post uint

	rows, err := repo.db.Query(`SELECT id
                FROM "issue#1".posts
                WHERE channel_from = $1`, username)
	if err != nil {
		return nil, fmt.Errorf("querying for posts failed because of: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&Post)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}

		PostList = append(PostList, Post)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return PostList, nil
}

// GetUnOfficialRelease is just a helper function that gets the Stickied Post List
func (repo *channelRepository) GetStickiedPost(username string) ([]uint, error) {
	var stickied []uint

	var Post uint

	rows, err := repo.db.Query(`SELECT
  post_id
FROM
  "issue#1".channel_stickies
INNER JOIN "issue#1".posts ON channel_stickies.post_id =posts.id WHERE posts.channel_from=$1
 `, username)
	if err != nil {
		return nil, fmt.Errorf("querying for posts failed because of: %v", err)
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		err := rows.Scan(&Post)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		stickied = append(stickied, Post)
		i++
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}

	return stickied, nil
}

// GetOwner is just a helper function that gets the Owner
func (repo *channelRepository) GetOwner(username string) (string, error) {
	var owner string
	var Admin string
	var isOwner bool
	rows, err := repo.db.Query(`SELECT username,is_owner
                FROM "issue#1".channel_admins
                WHERE channel_username = $1`, username)
	if err != nil {
		return "", fmt.Errorf("querying for admins failed because of: %v", err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		err := rows.Scan(&Admin, &isOwner)
		if err != nil {
			return "", fmt.Errorf("scanning from rows failed because: %v", err)
		}
		if isOwner {
			owner = Admin
		}
		i++
	}
	err = rows.Err()
	if err != nil {
		return "", fmt.Errorf("scanning from rows faulty because: %v", err)
	}

	return owner, nil
}

// SearchChannel searches for channels according to the pattern.
// If no pattern is provided, it returns all channels.
// It makes use of pagination.
func (repo *channelRepository) SearchChannels(pattern string, sortBy channel.SortBy, sortOrder channel.SortOrder, limit, offset int) ([]*channel.Channel, error) {

	var channels = make([]*channel.Channel, 0)
	var err error
	var rows *sql.Rows
	var query string
	if pattern == "" {
		query = fmt.Sprintf(`(SELECT username,name, COALESCE(description, ''),creation_time 
												FROM "issue#1".channels) 
												ORDER BY %s %s NULLS LAST
												LIMIT $1 OFFSET $2`, sortBy, sortOrder)
		rows, err = repo.db.Query(query, limit, offset)
	} else {
		query = `SELECT username,name, COALESCE(description, ''),creation_time 
			from channels
			where username ilike '%' || $1 || '%'  OR name  ilike '%' || $1|| '%'
			LIMIT $2 OFFSET $3`
		rows, err = repo.db.Query(query, pattern, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("querying for channels failed because of: %v", err)
	}

	var creationTimeString string
	for rows.Next() {
		c := channel.Channel{}
		err := rows.Scan(&c.ChannelUsername, &c.Name, &c.Description, &creationTimeString)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %s", err.Error())
		}
		creationTime, err := time.Parse(time.RFC3339, creationTimeString)
		if err != nil {
			return nil, fmt.Errorf("parsing of timestamp to time.Time failed because of: %s", err.Error())
		}
		creationTime, errC := time.Parse(time.RFC3339, creationTimeString)
		if errC != nil {
			return nil, fmt.Errorf("parsing of timestamp to time.Time failed because of: %s", errC.Error())
		}
		c.CreationTime = creationTime

		admins, err := repo.GetAdmins(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get admins because of: %s", err.Error())
		}
		owner, err := repo.GetOwner(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get owner because of: %s", err.Error())
		}
		stickiedPosts, err := repo.GetStickiedPost(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get stickied posts because of: %s", err.Error())
		}
		posts, err := repo.GetPosts(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get posts because of: %s", err.Error())
		}
		unOfficialReleases, err := repo.GetUnOfficialRelease(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get official realeases because of: %s", err.Error())
		}
		officialReleases, err := repo.GetOfficialRelease(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get official realeases because of: %s", err.Error())
		}
		pictureURL, err := repo.GetPicture(c.ChannelUsername)
		if err != nil {
			return nil, fmt.Errorf("unable to get picture because of: %s", err.Error())
		}
		c.AdminUsernames = admins
		c.OwnerUsername = owner
		c.StickiedPostIDs = stickiedPosts
		c.PostIDs = posts
		c.ReleaseIDs = unOfficialReleases
		c.OfficialReleaseIDs = officialReleases
		c.PictureURL = pictureURL
		channels = append(channels, &c)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %s", err.Error())
	}
	return channels, nil
}

// AddAdmin adds a User adminUsername to the channel channelUsername as an admin
func (repo *channelRepository) AddAdmin(channelUsername string, adminUsername string) error {
	var err error
	owner := false
	_, err = repo.db.Exec(`INSERT INTO "issue#1".channel_admins (channel_username,username,is_owner)
							VALUES ($1, $2,$3)`, channelUsername, adminUsername, owner)
	if err != nil {
		const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
		const uniqueKeyViolationErrorCode = pq.ErrorCode("23505")
		pgErr := err.(*pq.Error)

		if pgErr.Code == foreignKeyViolationErrorCode {
			return channel.ErrAdminNotFound
		} else if pgErr.Code == uniqueKeyViolationErrorCode {
			return channel.ErrAdminAlreadyExists
		} else {

			return fmt.Errorf("inserting into admins failed because of: %s ", err.Error())

		}

	}
	return nil
}

// DeleteAdmin deletes role of a User adminUsername of the channel channelUsername as an admin
func (repo *channelRepository) DeleteAdmin(channelUsername string, adminUsername string) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".channel_admins
							WHERE channel_username = $1 AND username= $2`, channelUsername, adminUsername)
	if err != nil {
		return fmt.Errorf("deletion of tuple from channel_admins because of: %s", err.Error())
	}
	return nil
}

// GetAdmins gets a list of Admins of channel channelUsername
func (repo *channelRepository) GetAdmins(channelUsername string) ([]string, error) {
	var AdminList []string
	var Admin string
	rows, err := repo.db.Query(`SELECT username
                FROM "issue#1".channel_admins
                WHERE channel_username = $1`, channelUsername)
	if err != nil {
		return nil, fmt.Errorf("querying for admins failed because of: %v", err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		err := rows.Scan(&Admin)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		AdminList = append(AdminList, Admin)
		i++
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return AdminList, nil
}

// ChangeOwner gets the owner of channel channelUsername
func (repo *channelRepository) ChangeOwner(channelUsername string, ownerUsername string) error {
	var err error
	var owner bool = true

	_, err = repo.db.Exec(`UPDATE "issue#1".channel_admins
								  SET is_owner = $3 WHERE channel_username =$1 AND username=$2`, channelUsername, ownerUsername, owner)
	if err != nil {
		const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
		pgErr := err.(*pq.Error)

		if pgErr.Code == foreignKeyViolationErrorCode {
			return channel.ErrOwnerNotFound
		}
		return fmt.Errorf("changing of owner failed because of: %s", err.Error())
	}
	_, err = repo.db.Exec(`UPDATE "issue#1".channel_admins SET is_owner = $3 where channel_username=$1 AND  username<>$2`, channelUsername, ownerUsername, !owner)
	if err != nil {
		return fmt.Errorf("changing of owner failed because of: %s", err.Error())
	}
	return nil
}

// AddReleaseToOfficialCatalog adds a release releaseID into the Official Catalog channel channelUsername
func (repo *channelRepository) AddReleaseToOfficialCatalog(channelUsername string, releaseID uint, postID uint) error {

	_, err := repo.db.Exec(`INSERT INTO channel_official_catalog (channel_username,release_id,post_from_id)
							VALUES ($1, $2,$3)`, channelUsername, releaseID, postID)
	if err != nil {
		const uniqueKeyViolationErrorCode = pq.ErrorCode("23505")
		pgErr := err.(*pq.Error)
		if pgErr.Code == uniqueKeyViolationErrorCode {
			return channel.ErrReleaseAlreadyExists
		} else {

			return fmt.Errorf("addition of tuple of release channel_official_catalogs because of: %s", err.Error())

		}
	}

	return nil
}

// DeleteReleaseFromCatalog deletes a release releaseID from Catalog of channel channelUsername
func (repo *channelRepository) DeleteReleaseFromCatalog(channelUsername string, ReleaseID uint) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".releases
							WHERE owner_channel = $1 AND id = $2`, channelUsername, ReleaseID)
	if err != nil {
		return fmt.Errorf("deletion of tuple from channel_catalogs because of: %s", err.Error())
	}
	return nil
}

// DeleteReleaseFromOfficialCatalog deletes a release releaseID from Official Catalog of channel channelUsername
func (repo *channelRepository) DeleteReleaseFromOfficialCatalog(channelUsername string, ReleaseID uint) error {

	_, err := repo.db.Exec(`DELETE FROM "issue#1".channel_official_catalog
							WHERE channel_username = $1 AND release_id = $2`, channelUsername, ReleaseID)
	if err != nil {
		return fmt.Errorf("deletion of tuple from channel_catalogs because of: %s", err.Error())
	}
	return nil
}

// StickyPost stickies a post on channel channelUsername
func (repo *channelRepository) StickyPost(channelUsername string, postID uint) error {
	a, err := repo.GetStickiedPost(channelUsername)
	if err != nil {
		return fmt.Errorf("getting stickied posts failed because of: %s", err.Error())
	}
	if len(a) == 2 {
		return channel.ErrStickiedPostFull
	} else {
		_, err := repo.db.Exec(`INSERT INTO channel_stickies (post_id)
							VALUES ($1)
							ON CONFLICT DO NOTHING`, postID)
		const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
		if err != nil {
			if pgErr, isPGErr := err.(pq.Error); !isPGErr {
				if pgErr.Code != foreignKeyViolationErrorCode {
					return channel.ErrPostNotFound
				}
				return fmt.Errorf("inserting into channel_stickies failed because of: %s", err.Error())
			}
		}
	}
	return nil
}

// DeleteStickiedPost deletes a stickied post from channel channelUsername
func (repo *channelRepository) DeleteStickiedPost(channelUsername string, stickiedPostID uint) error {
	//TODO
	_, err := repo.db.Exec(`DELETE FROM "issue#1".channel_stickies
							WHERE post_id = $1`, stickiedPostID)
	if err != nil {
		return fmt.Errorf("deletion of tuple from channel_stickie because of: %s", err.Error())
	}
	return nil
}

// AddPicture persists the given name as the image_name for the user under the given username
func (repo *channelRepository) AddPicture(channelUsername string, name string) (string, error) {
	_, err := repo.db.Exec(`INSERT INTO "issue#1".channel_pictures (channelname, image_name) 
								VALUES ($1, $2)
								ON CONFLICT(channelname) DO UPDATE
								SET image_name = $1`, channelUsername, name)
	const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
	if err != nil {
		if pgErr, isPGErr := err.(pq.Error); !isPGErr {
			if pgErr.Code == foreignKeyViolationErrorCode {
				return "", channel.ErrChannelNotFound
			}
			return "", fmt.Errorf("inserting into channel_pictures failed because of: %v", err)
		}
	}
	return name, nil
}

// RemovePicture removes the username's tuple entry from the user_avatars table.
func (repo *channelRepository) RemovePicture(channelUsername string) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".channel_pictures
							WHERE channelname = $1`, channelUsername)
	if err != nil {
		return fmt.Errorf("deletion of tuple from channel_pictures failed because of: %v", err)
	}
	return nil
}

// GetPicture gets the username's tuple entry from the user_avatars table.
func (repo *channelRepository) GetPicture(ChannelUsername string) (string, error) {
	var pictureURL string

	rows, err := repo.db.Query(`SELECT image_name
                FROM "issue#1".channel_pictures
                WHERE channelname = $1`, ChannelUsername)
	if err != nil {
		return "", fmt.Errorf("querying for pictures failed because of: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&pictureURL)
		if err != nil {
			return "", err
		}

	}
	err = rows.Err()
	if err != nil {
		return "", err
	}

	return pictureURL, nil
}
