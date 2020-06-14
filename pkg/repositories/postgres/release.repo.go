package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/slim-crown/issue-1-REST/pkg/services/domain/release"
)

type releaseRepository repository

// NewReleaseRepository returns a struct that implements the release.Repository using
// a PostgresSQL database.
// A database connection needs to be passed so that it can function.
func NewReleaseRepository(db *sql.DB, allRepos *map[string]interface{}) release.Repository {
	return &releaseRepository{db: db, allRepos: allRepos}
}

// GetRelease returns a release.Release under the given id from the database.
func (repo releaseRepository) GetRelease(id int) (*release.Release, error) {
	var err error
	var r = new(release.Release)

	var typeString string
	query := `SELECT type, owner_channel, creation_time
				FROM releases
				WHERE id = $1`
	err = repo.db.QueryRow(query, id).Scan(&typeString, &r.OwnerChannel, &r.CreationTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, release.ErrReleaseNotFound
		}
		return nil, fmt.Errorf("unable to get release from db becaues: %v", err)
	}
	r.Type = release.Type(typeString)
	content, err := repo.getContent(id, r.Type)
	if err != nil {
		return nil, err
	}
	r.Content = content

	metadata, err := repo.getMetadata(id)
	if err != nil {
		return nil, err
	}
	r.Metadata = *metadata

	r.ID = id
	return r, nil
}

// SearchRelease searches the database for releases that satisfy the given arguments.
func (repo releaseRepository) SearchRelease(pattern string, by release.SortBy, order release.SortOrder, limit int, offset int) ([]*release.Release, error) {
	var releases = make([]*release.Release, 0)
	var err error
	var rows *sql.Rows
	var query string
	if pattern == "" {
		query = fmt.Sprintf(`
				SELECT id, owner_channel, content, type, creation_time
				FROM (
				         SELECT *
				         FROM releases
				                  LEFT JOIN
				              (
				                  SELECT release_id, content
				                  FROM (
				                           SELECT release_id, image_name as content
				                           FROM releases_image_based
				                       ) AS ric
				                  UNION
				                  SELECT *
				                  FROM releases_text_based
				              ) AS cs
				              ON releases.id = cs.release_id
				     ) AS "r*"
				         NATURAL JOIN
				     (
				         SELECT release_id
						FROM "issue#1".channel_official_catalog
				     ) AS "coc*"
				ORDER BY %s %s NULLS LAST
				LIMIT $1 OFFSET $2`, by, order)
		rows, err = repo.db.Query(query, limit, offset)
	} else {
		query = `
				SELECT id, owner_channel, content, type, creation_time
				FROM (
				         SELECT *
				         FROM (
				                  SELECT ts_rank(vector, query, 32) as rank, *
				                  FROM (
				                           SELECT release_id as id, vector, query
				                           FROM tsvs_release,
				                                websearch_to_tsquery('english', $1) query
				                           where vector @@ query
				                       ) AS rti
				                           NATURAL JOIN
				                       releases
				              ) AS rc
				                  LEFT JOIN
				              (
				                  SELECT release_id, content
				                  FROM (
				                           SELECT release_id, image_name as content
				                           FROM releases_image_based
				                       ) AS ric
				                  UNION
				                  SELECT *
				                  FROM releases_text_based
				              ) AS cs
				              ON rc.id = cs.release_id
				     ) AS "rc**"
				         NATURAL JOIN
				     (
				         SELECT release_id
				         FROM channel_official_catalog
				     ) AS "coc*"
				ORDER BY rank DESC`
		if by != "" {
			query = fmt.Sprintf(`%s, %s %s NULLS LAST`, query, by, order)
		}
		query = fmt.Sprintf(`%s 
				LIMIT $2 OFFSET $3`, query)
		rows, err = repo.db.Query(query, pattern, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("querying for releases failed because of: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		r := new(release.Release)
		err := rows.Scan(&r.ID, &r.OwnerChannel, &r.Content, &r.Type, &r.CreationTime)
		if err != nil {
			return nil, fmt.Errorf("scanning from rows failed because: %v", err)
		}

		metadata, err := repo.getMetadata(r.ID)
		if err != nil {
			return nil, err
		}
		r.Metadata = *metadata

		releases = append(releases, r)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("scanning from rows faulty because: %v", err)
	}
	return releases, nil
}

// DeleteRelease removes the release under the given id from the database.
func (repo releaseRepository) DeleteRelease(id int) error {
	_, err := repo.db.Exec(`DELETE FROM releases
							WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deletion of release failed because: %v", err)
	}
	return nil
}

// AddRelease persists the given struct into the database.
func (repo releaseRepository) AddRelease(r *release.Release) (*release.Release, error) {
	query := `INSERT INTO releases (owner_channel, type) 
				VALUES ($1, $2)
				RETURNING id`
	err := repo.db.QueryRow(query, r.OwnerChannel, r.Type).Scan(&r.ID)
	if err != nil {
		return nil, fmt.Errorf("insertion of release failed because of: %v", err)
	}
	r.OwnerChannel = ""
	return repo.UpdateRelease(r)
}

// UpdateRelease updates a release in the database according to the given struct.
func (repo releaseRepository) UpdateRelease(rel *release.Release) (*release.Release, error) {
	var errs []error
	// Checks if value is to be updated before attempting.
	// This way, there won't be columns with Go's zero string value of "" instead of null
	if rel.OwnerChannel != "" {
		err := repo.execUpdateStatementOnColumnIntoReleases("owner_channel", rel.OwnerChannel, rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if rel.Content != "" && rel.Type != "" {
		err := repo.execUpdateStatementForContent(rel.Type, rel.Content, rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if !rel.ReleaseDate.IsZero() {
		err := repo.execUpdateStatementOnColumnIntoMetadata("release_date", rel.ReleaseDate, rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if rel.Title != "" {
		err := repo.execUpdateStatementOnColumnIntoMetadata("title", rel.Title, rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if rel.GenreDefining != "" {
		err := repo.execUpdateStatementOnColumnIntoMetadata("genre_defining", rel.GenreDefining, rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if rel.Description != "" {
		err := repo.execUpdateStatementOnColumnIntoMetadata("description", rel.Description, rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	}
	otherJSONRaw, err := json.Marshal(rel.Other)
	if err == nil {
		//jsonbString := fmt.Sprintf("to_jsonb(%s::text)", string(otherJSONRaw))
		err := repo.execUpdateStatementOnColumnIntoMetadata("other", string(otherJSONRaw), rel.ID)
		if err != nil {
			errs = append(errs, err)
		}
	} else {
		errs = append(errs, err)
	}
	const maxNoOfPossibleErr = 6
	if len(errs) == maxNoOfPossibleErr {
		return nil, fmt.Errorf("was unable to update any data because of %v", errs)
	}
	r, err := repo.GetRelease(rel.ID)
	if err == nil {
		if len(errs) > 0 {
			fmt.Printf("%+v", errs)
			err = release.ErrSomeReleaseDataNotPersisted
		}
	}
	return r, err
}

func (repo releaseRepository) execUpdateStatementOnColumnIntoReleases(column, value string, id int) error {
	query := fmt.Sprintf(`UPDATE releases
								SET %s = $1 
								WHERE id = $2`, column)
	_, err := repo.db.Exec(query, value, id)
	if err != nil {
		return fmt.Errorf("updating failed of %s column with %s because of: %v", column, value, err)
	}
	return nil
}

func (repo releaseRepository) execUpdateStatementOnColumnIntoMetadata(column string, value interface{}, id int) error {
	query := fmt.Sprintf(`INSERT INTO release_metadata (release_id, %s)
								VALUES ($1, $2)
								ON CONFLICT(release_id) DO UPDATE
								SET %s = $2`, column, column)
	_, err := repo.db.Exec(query, id, value)
	if err != nil {
		return fmt.Errorf("upsertion failed of %s column to metadat with %s because of: %v", column, value, err)
	}
	return nil
}

func (repo releaseRepository) execUpdateStatementForContent(t release.Type, value string, id int) error {
	var query string
	if t == release.Image {
		query = `INSERT INTO releases_image_based (release_id, image_name)
				VALUES ($1, $2)
				ON CONFLICT(release_id) DO UPDATE
				SET image_name = $2`
	} else {
		query = `INSERT INTO releases_text_based (release_id, content)
				VALUES ($1, $2)
				ON CONFLICT(release_id) DO UPDATE
				SET content = $2`
	}
	_, err := repo.db.Exec(query, id, value)
	if err != nil {
		return fmt.Errorf("upserting failed of %s type with %s because of: %v", string(t), value, err)
	}
	return nil
}

func (repo releaseRepository) getContent(id int, t release.Type) (string, error) {
	var content, query string
	if t == release.Image {
		query = `SELECT COALESCE(image_name, '') 
				FROM releases_image_based 
				WHERE release_id = $1`
	} else {
		query = `SELECT COALESCE(content, '') 
				FROM releases_text_based 
				WHERE release_id = $1`
	}
	err := repo.db.QueryRow(query, id).Scan(&content)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("unable to get release content because: %v", err)
	}
	return content, nil
}

func (repo releaseRepository) getMetadata(id int) (*release.Metadata, error) {
	var err error
	var meta = new(release.Metadata)

	var otherJSON string

	query := `SELECT COALESCE(title, ''), COALESCE(description, ''), COALESCE(genre_defining, ''), COALESCE(release_date, to_timestamp(0)), COALESCE(other, jsonb_build_object())
				FROM release_metadata
				WHERE release_id = $1`
	err = repo.db.QueryRow(query, id).Scan(&meta.Title, &meta.Description, &meta.GenreDefining, &meta.ReleaseDate, &otherJSON)
	if err != nil {
		return nil, fmt.Errorf("metadata for release not found because: %v", err)
	}

	err = json.Unmarshal([]byte(otherJSON), &meta.Other)
	if err != nil {
		return nil, fmt.Errorf("parsing of 'Other' json blob for metadata failed because: %v", err)
	}

	return meta, nil
}
