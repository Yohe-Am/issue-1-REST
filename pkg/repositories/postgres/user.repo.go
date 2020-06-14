package postgres

import (
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/user"
	"golang.org/x/crypto/bcrypt"
	"time"

	"github.com/lib/pq"
)

// userRepository ...
type userRepository repository

// NewUserRepository returns a new in PostgreSQL implementation of user.Repository.
// the database connection must be passed as the first argument
// since for the repo to work.
// A map of all the other PostgreSQL based implementations of the Repository interfaces
// found in the different services of the project must be passed as a second argument as
// the Repository might make user of them to fetch objects instead of implementing redundant logic.
func NewUserRepository(DB *sql.DB, allRepos *map[string]interface{}) user.Repository {
	return &userRepository{DB, allRepos}
}

// AddUser takes in a user.User struct and persists it in the database.
func (repo *userRepository) AddUser(u *user.User) (*user.User, error) {
	var err error
	passHash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("unable to generate bcrypt has because: %w", err)
	}
	_, err = repo.db.Exec(`INSERT INTO "issue#1".users (username, email, pass_hash)
							VALUES ($1, $2, $3)`, u.Username, u.Email, string(passHash))
	if err != nil {
		return nil, fmt.Errorf("insertion of user failed because of: %w", err)
	}

	// set the username to zero to avoid call to UpdateUser won't do redundant updating of username
	username := u.Username
	u.Username = ""
	u.Email = ""
	u.Password = ""

	// using UpdateUser to set the rest of the values so that null values will be preserved
	// (instead of columns with go's zero value of "")
	return repo.UpdateUser(username, u)
}

// GetUser retrieves a user.User based on the username passed.
func (repo *userRepository) GetUser(username string) (*user.User, error) {
	var err error
	var u = new(user.User)

	err = repo.db.QueryRow(`
								SELECT email, COALESCE(first_name, ''), COALESCE(middle_name, ''), COALESCE(last_name, ''), creation_time, COALESCE(bio, ''), COALESCE(image_name, '')
								FROM users LEFT JOIN users_bio ub on users.username = ub.username LEFT JOIN user_avatars ua on users.username = ua.username
								WHERE users.username = $1`, username).Scan(&u.Email, &u.FirstName, &u.MiddleName, &u.LastName, &u.CreationTime, &u.Bio, &u.PictureURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, user.ErrUserNotFound
		}
		return nil, fmt.Errorf("unable to get user from db becaues: %v", err)
	}

	bookmarkedPosts, err := repo.getBookmarkedPosts(username)
	if err != nil {
		return nil, fmt.Errorf("unable to get bookmarked posts because of: %v", err)
	}
	u.BookmarkedPosts = bookmarkedPosts

	u.Username = username
	return u, nil
}

// getBookmarkedPosts is just a helper function
func (repo *userRepository) getBookmarkedPosts(username string) (map[time.Time]int, error) {
	var bookmarkedPosts = make(map[time.Time]int, 0)

	rows, err := repo.db.Query(`SELECT post_id, creation_time
								FROM "issue#1".user_bookmarks 
								WHERE username = $1`, username)
	if err != nil {
		return nil, fmt.Errorf("querying for user_bookmarks failed because of: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			postID       int
			creationTime time.Time
		)
		err := rows.Scan(&postID, &creationTime)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		bookmarkedPosts[creationTime] = postID
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return bookmarkedPosts, nil
}

// UpdateUser updates a user based on the passed user.User struct.
// If updating in the DB repo is successful, it updates its cache by getting
// the new user.User and converting it into a cache able format.
func (repo *userRepository) UpdateUser(username string, u *user.User) (*user.User, error) {
	var errs []error

	// if err != nil {
	// 	return fmt.Errorf("creation of update statement failed because of: %v", err)
	// }
	// Checks if value is to be updated before attempting.
	// This way, there won't be columns with go's zero string value of "" instead of null
	if u.Password != "" {

		passHash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("unable to generate bcrypt has because: %w", err)
		}

		err = repo.execUpdateStatementOnColumn("pass_hash", string(passHash), username)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if u.Email != "" {
		err := repo.execUpdateStatementOnColumn("email", u.Email, username)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if u.FirstName != "" {
		err := repo.execUpdateStatementOnColumn("first_name", u.FirstName, username)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if u.MiddleName != "" {
		err := repo.execUpdateStatementOnColumn("middle_name", u.MiddleName, username)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if u.LastName != "" {
		err := repo.execUpdateStatementOnColumn("last_name", u.LastName, username)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if u.Bio != "" {
		_, err := repo.db.Exec(`INSERT INTO "issue#1".users_bio(bio, username)
								VALUES ($1, $2)
								ON CONFLICT(username) DO UPDATE
								SET bio = $1`, u.Bio, username)
		if err != nil {
			errs = append(errs, fmt.Errorf("upsertion of bio failed because of: %v", err))
		}
	}
	if u.Username != "" {
		err := repo.execUpdateStatementOnColumn("username", u.Username, username)
		if err != nil {
			errs = append(errs, err)
		}
		// change username for subsequent calls if username changed
		username = u.Username
	}
	/*
		if u.PictureURL != "" {
			err := repo.AddPicture(u.Username, u.PictureURL)
			if err != nil {
				errs = append(errs, err)
			}
		}*/
	const maxNoOfPossibleErr = 7
	if len(errs) == maxNoOfPossibleErr {
		return nil, fmt.Errorf("was unable to update any data because of %v", errs)
	}
	u, err := repo.GetUser(username)
	if err == nil {
		if len(errs) > 0 {
			fmt.Printf("%+v", errs)
			err = user.ErrSomeUserDataNotPersisted
		}
	}
	return u, err
}

// execUpdateStatementOnColumn is just a helper function
func (repo *userRepository) execUpdateStatementOnColumn(column, value, username string) error {
	_, err := repo.db.Exec(fmt.Sprintf(`UPDATE "issue#1".users 
									SET %s = $1 
									WHERE username = $2`, column), value, username)
	if err != nil {
		return fmt.Errorf("updating failed of %s column with %s because of: %s", column, value, err.Error())
	}
	return nil
}

// DeleteUser deletes a user based on the passed in username.
// If deletion is successful, it also tries to delete the user from its cache.
func (repo *userRepository) DeleteUser(username string) error {
	_, err := repo.db.Exec(`DELETE FROM users
							WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("deletion of user failed because of: %v", err)
	}
	return nil
}

// SearchUser searches for users according to the pattern.
// If no pattern is provided, it returns all users.
// It makes use of pagination.
func (repo *userRepository) SearchUser(pattern, sortBy, sortOrder string, limit, offset int) ([]*user.User, error) {
	var users = make([]*user.User, 0)
	var err error
	var rows *sql.Rows

	if pattern == "" {
		rows, err = repo.db.Query(fmt.Sprintf(`
		SELECT users.username, email, COALESCE(first_name, ''), COALESCE(middle_name, ''), COALESCE(last_name, ''), creation_time, COALESCE(bio, ''), COALESCE(image_name, '')
		FROM users LEFT JOIN users_bio ub on users.username = ub.username LEFT JOIN user_avatars ua on users.username = ua.username
		ORDER BY %s %s NULLS LAST
		LIMIT $1 OFFSET $2`, sortBy, sortOrder), limit, offset)
	} else {
		query := fmt.Sprintf(`
		SELECT users.username, email, COALESCE(first_name, ''), COALESCE(middle_name, ''), COALESCE(last_name, ''), creation_time, COALESCE(bio, ''), COALESCE(image_name, '')
		FROM users LEFT JOIN users_bio ub on users.username = ub.username LEFT JOIN user_avatars ua on users.username = ua.username
		WHERE users.username ILIKE '%%' || $3 || '%%' OR first_name ILIKE '%%' || $3 || '%%' OR last_name ILIKE '%%' || $3 || '%%'
		ORDER BY %s %s NULLS LAST
		LIMIT $1 OFFSET $2`, sortBy, sortOrder)
		rows, err = repo.db.Query(query, limit, offset, pattern)
	}
	if err != nil {
		return nil, fmt.Errorf("querying for users failed because of: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		u := user.User{}
		err := rows.Scan(&u.Username, &u.Email, &u.FirstName, &u.MiddleName, &u.LastName, &u.CreationTime, &u.Bio, &u.PictureURL)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}
		bookmarkedPosts, err := repo.getBookmarkedPosts(u.Username)
		if err != nil {
			return nil, fmt.Errorf("unable to get bookmarked posts because of: %s", err.Error())
		}
		u.BookmarkedPosts = bookmarkedPosts

		users = append(users, &u)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return users, nil
}

// Authenticate checks the given pass hash against the pass hash found in the database for the username.
func (repo *userRepository) Authenticate(u *user.User) (bool, error) {
	/*	var hashedPassword string
		{
			// this block hashes the password
			cat := u.Password + u.Username
			hashedPasswordArr := sha512.Sum512([]byte(cat))
			hashedPassword = hex.EncodeToString(hashedPasswordArr[:])
		}
	*/
	query := `
SELECT EXISTS(
               SELECT username
               FROM users
               WHERE username = $2
                 AND pass_hash = sha512(
                       ($1 || $2)::bytea
                   )::text
           )
           OR
       EXISTS(
               SELECT username
               FROM users
               WHERE email = $3
                 AND pass_hash = sha512(
                       ($1
                           ||
                        (
                            SELECT username
                            from users
                            where email = $3
                        )
                           )::bytea
                   )::text
           )`
	var accepted bool
	err := repo.db.QueryRow(query, u.Password, u.Username, u.Email).Scan(&accepted)
	if err != nil {
		return false, fmt.Errorf("couldn't authenticate user beacause: %w", err)
	}
	return accepted, nil
}

// BookmarkPost bookmarks the given postID for the user of the given username.
func (repo *userRepository) BookmarkPost(username string, postID int) error {
	_, err := repo.db.Exec(`INSERT INTO user_bookmarks (username, post_id)
							VALUES ($1, $2)
							ON CONFLICT DO NOTHING`, username, postID)
	const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
	if err != nil {
		if pgErr, isPGErr := err.(pq.Error); !isPGErr {
			if pgErr.Code != foreignKeyViolationErrorCode {
				return user.ErrPostNotFound
			}
			return fmt.Errorf("inserting into user_bookmarks failed because of: %v", err)
		}
	}
	return nil
}

// DeleteBookmark removes the given ID from the given user's bookmarks
func (repo *userRepository) DeleteBookmark(username string, postID int) error {
	_, err := repo.db.Exec(`DELETE FROM "issue#1".user_bookmarks
							WHERE username = $1 AND post_id = $2`, username, postID)
	if err != nil {
		return fmt.Errorf("deletion of tuple from user_bookmarks because of: %v", err)
	}
	return nil
}

// UsernameOccupied checks if the given username is occupied by another user or a channel
func (repo *userRepository) UsernameOccupied(username string) (bool, error) {
	var occupied bool
	err := repo.db.QueryRow(`
				SELECT EXISTS(
               SELECT username
               FROM ((select username from users)
                   UNION
                   (select username from channels)
                        ) as R
               WHERE username = $1)`, username).Scan(&occupied)
	if err != nil {
		return true, fmt.Errorf("unable to check if email occupied")
	}
	return occupied, nil
}

// EmailOccupied checks if the given email is occupied by another user
func (repo *userRepository) EmailOccupied(email string) (bool, error) {
	var occupied bool
	err := repo.db.QueryRow(`SELECT EXISTS(SELECT username FROM "issue#1".users
									WHERE email = $1)`, email).Scan(&occupied)
	if err != nil {
		return true, fmt.Errorf("unable to check if email occupied")
	}
	return occupied, nil
}

// AddPicture persists the given name as the image_name for the user under the given username
func (repo *userRepository) AddPicture(username, name string) error {
	_, err := repo.db.Exec(`INSERT INTO user_avatars (username, image_name) 
								VALUES ($1, $2)
								ON CONFLICT(username) DO UPDATE
								SET image_name = $1`, username, name)
	const foreignKeyViolationErrorCode = pq.ErrorCode("23503")
	if err != nil {
		if pgErr, isPGErr := err.(pq.Error); !isPGErr {
			if pgErr.Code != foreignKeyViolationErrorCode {
				return user.ErrUserNotFound
			}
			return fmt.Errorf("userting into user_avatars failed because of: %v", err)
		}
	}
	return nil
}

// RemovePicture removes the username's tuple entry from the user_avatars table.
func (repo *userRepository) RemovePicture(username string) error {
	_, err := repo.db.Exec(`DELETE FROM user_avatars
							WHERE username = $1`, username)
	if err != nil {
		return fmt.Errorf("deletion of tuple from user_avatars failed because of: %v", err)
	}
	return nil
}
