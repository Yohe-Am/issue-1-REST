package rest

import (
	"encoding/json"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/channel"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/release"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// postRelease returns a handler for POST /releases requests
func postRelease(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusCreated

		newRelease := new(release.Release)
		var tmpFile *os.File
		{ // this block parses the JSON part of the request
			err := json.Unmarshal([]byte(r.PostFormValue("JSON")), newRelease)
			if err != nil {
				err = json.NewDecoder(r.Body).Decode(newRelease)
				if err != nil {
					response.Data = jSendFailData{
						ErrorReason:  "request format",
						ErrorMessage: "use multipart for for posting Image Releases. A part named 'JSON' with format\r\n{\n  \"ownerChannel\": \"ownerChannel\",\n  \"type\": \"image or text\",\n  \"content\": \"content if type is text\",\n  \"metadata\": {\n    \"title\": \"title\",\n    \"releaseDate\": \"unix timestamp\",\n    \"genreDefining\": \"genreDefining\",\n    \"description\": \"description\",\n    \"Other\": { \"authors\": [], \"genres\": [] }\n  }\n}\nfor Release data and a file called 'image' if release is of image type. We accept JPG/PNG formats.",
					}
					statusCode = http.StatusBadRequest
				}
			}
		}
		if response.Data == nil {
			{
				// this block checks for required fields
				if newRelease.OwnerChannel == "" {
					response.Data = jSendFailData{
						ErrorReason:  "OwnerChannel",
						ErrorMessage: "OwnerChannel is required",
					}
					// if required fields aren't present
					s.Logger.Printf("bad add release request")
					statusCode = http.StatusBadRequest
				} else {
					{ // this block secure the route
						c, err := s.ChannelService.GetChannel(newRelease.OwnerChannel)
						switch err {
						case nil:
							isAdmin := false
							for _, admin := range c.AdminUsernames {
								if r.Header.Get("authorized_username") == admin {
									isAdmin = true
									break
								}
							}
							if !isAdmin {
								s.Logger.Printf("unauthorized add release request")
								addCors(w)
								w.WriteHeader(http.StatusUnauthorized)
								return
							}
						case channel.ErrChannelNotFound:
							s.Logger.Printf("replease post attempt on non-existent channel %s", newRelease.OwnerChannel)
							response.Data = jSendFailData{
								ErrorReason:  "ownerChannel",
								ErrorMessage: fmt.Sprintf("channel of channelUsername %s not found", newRelease.OwnerChannel),
							}
							statusCode = http.StatusNotFound
						default:
							s.Logger.Printf("adding of release failed during auth because: %v", err)
							response.Status = "error"
							response.Message = "server error when adding release"

						}
					}
				}
			}
			if response.Data == nil {
				{ // this block extracts the image file if necessary
					switch newRelease.Type {
					case release.Image:
						var fileName string
						var err error
						tmpFile, fileName, err = saveImageFromRequest(r, "image")
						switch err {
						case nil:
							defer tmpFile.Close()
							defer os.Remove(tmpFile.Name())
							s.Logger.Printf(fmt.Sprintf("temp file saved: %s", tmpFile.Name()))
							newRelease.Content = generateFileNameForStorage(fileName, "release")
						case errUnacceptedType:
							response.Data = jSendFailData{
								ErrorMessage: "image-type",
								ErrorReason:  "only types image/jpeg & image/png are accepted",
							}
							statusCode = http.StatusBadRequest
						case errReadingFromImage:
							response.Data = jSendFailData{
								ErrorReason:  "image",
								ErrorMessage: "unable to read image file\nuse multipart-form for for posting Image Releases. A part named 'JSON' for Release data \nand a file called 'image' of image type JPG/PNG.",
							}
							statusCode = http.StatusBadRequest
						default:
							s.Logger.Printf("adding of release failed during image parsing because: %v", err)
							response.Status = "error"
							response.Message = "server error when adding release"

						}
					case release.Text:
						if newRelease.Content == "" {
							response.Data = jSendFailData{
								ErrorReason:  "content",
								ErrorMessage: "content is required for text based releases",
							}
							statusCode = http.StatusBadRequest
						}
					default:
						statusCode = http.StatusBadRequest
						response.Data = jSendFailData{
							ErrorMessage: "type can only be 'text' or 'image'",
							ErrorReason:  "type",
						}
					}
				}
				if response.Data == nil {
					newRelease, err := s.ReleaseService.AddRelease(newRelease)
					switch err {
					case nil:
						if newRelease.Type == release.Image {
							err := saveTempFilePermanentlyToPath(tmpFile, s.ImageStoragePath+newRelease.Content)
							if err != nil {
								s.Logger.Printf("adding of release failed because: %v", err)
								response.Status = "error"
								response.Message = "server error when adding release"
								statusCode = http.StatusInternalServerError
								_ = s.ReleaseService.DeleteRelease(newRelease.ID)
							}
						}
						if response.Message == "" {
							response.Status = "success"
							if newRelease.Type == release.Image {
								newRelease.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(newRelease.Content)
							}
							response.Data = *newRelease
							s.Logger.Printf("success adding release %d to channel %s", newRelease.ID, newRelease.OwnerChannel)
						}
					case release.ErrSomeReleaseDataNotPersisted:
						fallthrough
					default:
						if newRelease != nil && newRelease.ID != 0 {
							_ = s.ReleaseService.DeleteRelease(newRelease.ID)
						}
						s.Logger.Printf("adding of release failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when adding release"
						statusCode = http.StatusInternalServerError
					}
				}
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getRelease returns a handler for GET /releases/{id} requests
func getRelease(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["id"]

		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("fetch attempt of non invalid release id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "releaseID",
				ErrorMessage: fmt.Sprintf("invalid releaseID %s", idRaw),
			}
			statusCode = http.StatusBadRequest
		}

		if response.Data == nil {
			rel, err := s.ReleaseService.GetRelease(id)
			switch err {
			case nil:
				if rel.Type == release.Image {
					rel.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(rel.Content)
				}
				// TODO secure route
				{ // this block sanitizes the returned User if it's not the user herself accessing the route
					c, err := s.ChannelService.GetChannel(rel.OwnerChannel)
					switch err {
					case nil:
						isOfficial := false
						for _, relID := range c.OfficialReleaseIDs {

							if (uint(id)) == relID {
								isOfficial = true
							}
						}
						if isOfficial { // return the release if official
							response.Status = "success"
							response.Data = *rel
							s.Logger.Printf("success fetching release %d from an offical catalog", id)
							break
						}
						// if not official, send release back only for an admin
						isAdmin := false
						for _, admin := range c.AdminUsernames {
							if r.Header.Get("authorized_username") == admin {
								isAdmin = true
								break
							}
						}
						if isAdmin {
							response.Status = "success"
							if rel.Type == release.Image {
								rel.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(rel.Content)
							}
							response.Data = *rel
							s.Logger.Printf("success fetching release %d from an offical catalog", id)
							break
						}
						response.Data = jSendFailData{
							ErrorReason:  "releaseID",
							ErrorMessage: fmt.Sprintf("release of releaseID %d not found", id),
						}
						statusCode = http.StatusNotFound
						s.Logger.Printf("fetch attempt of unofficial release %d by non admin", id)
					case channel.ErrChannelNotFound:
						response.Data = jSendFailData{
							ErrorReason:  "releaseID",
							ErrorMessage: fmt.Sprintf("release of releaseID %d not found", id),
						}
						statusCode = http.StatusNotFound
						s.Logger.Printf("fetch attempt of release %d with out an owner", id)
					default:
						s.Logger.Printf("fetching of release failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when fetching release"
						statusCode = http.StatusInternalServerError
					}
				}
			case release.ErrReleaseNotFound:
				response.Data = jSendFailData{
					ErrorReason:  "releaseID",
					ErrorMessage: fmt.Sprintf("release of releaseID %d not found", id),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("fetching of release failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching release"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getReleases returns a handler for GET /releases?sort=new&limit=5&offset=0&pattern=Joe requests
func getReleases(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		pattern := ""
		limit := 25
		offset := 0
		sortBy := release.SortCreationTime
		sortOrder := release.SortDescending

		{ // this block reads the query strings if any
			pattern = r.URL.Query().Get("pattern")

			if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
				limit, err = strconv.Atoi(limitPageRaw)
				if err != nil || limit < 0 {
					s.Logger.Printf("bad get releases request, limit")
					response.Data = jSendFailData{
						ErrorReason:  "limit",
						ErrorMessage: "bad request, limit can't be negative",
					}
					statusCode = http.StatusBadRequest
				}
			}
			if offsetRaw := r.URL.Query().Get("offset"); offsetRaw != "" {
				offset, err = strconv.Atoi(offsetRaw)
				if err != nil || offset < 0 {
					s.Logger.Printf("bad request, offset")
					response.Data = jSendFailData{
						ErrorReason:  "offset",
						ErrorMessage: "bad request, offset can't be negative",
					}
					statusCode = http.StatusBadRequest
				}
			}

			sort := r.URL.Query().Get("sort")
			sortSplit := strings.Split(sort, "_")

			sortOrder = release.SortAscending
			switch sortByQuery := sortSplit[0]; sortByQuery {
			case "type":
				sortBy = release.SortByType
			case "channel":
				sortBy = release.SortByChannel
			default:
				sortBy = release.SortCreationTime
				sortOrder = release.SortDescending
			}
			if len(sortSplit) > 1 {
				switch sortOrderQuery := sortSplit[1]; sortOrderQuery {
				case "dsc":
					sortOrder = release.SortDescending
				default:
					sortOrder = release.SortAscending
				}
			}

		}
		// if queries are clean
		if response.Data == nil {
			releases, err := s.ReleaseService.SearchRelease(pattern, sortBy, sortOrder, limit, offset)
			if err != nil {
				s.Logger.Printf("fetching of releases failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when getting releases"
				statusCode = http.StatusInternalServerError
			} else {
				response.Status = "success"
				for _, rel := range releases {
					if rel.Type == release.Image {
						rel.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(rel.Content)
					}
				}
				response.Data = releases
				s.Logger.Printf("success fetching releases")
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// patchRelease returns a handler for PUT /releases/{id} requests
func patchRelease(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["id"]

		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("patch attempt on non invalid release id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "releaseID",
				ErrorMessage: fmt.Sprintf("invalid releaseID %s", idRaw),
			}
			statusCode = http.StatusBadRequest
		}
		if response.Data == nil {
			temp, err := s.ReleaseService.GetRelease(id)
			switch err {
			case nil:
				c, err := s.ChannelService.GetChannel(temp.OwnerChannel)
				switch err {
				case nil:
					isAdmin := false
					for _, admin := range c.AdminUsernames {
						if r.Header.Get("authorized_username") == admin {
							isAdmin = true
							break
						}
					}
					if !isAdmin {
						s.Logger.Printf("unauthorized add release request")
						addCors(w)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					rel := new(release.Release)
					var tmpFile *os.File
					{ // this block parses the JSON part of the request
						err := json.Unmarshal([]byte(r.PostFormValue("JSON")), rel)
						if err != nil {
							err = json.NewDecoder(r.Body).Decode(rel)
							if err != nil {
								// TODO send back format
								response.Data = jSendFailData{
									ErrorReason:  "message",
									ErrorMessage: "use multipart for for posting Image Releases. A part named 'JSON' for Release data \r\nand a file called 'image' if release is of image type JPG/PNG.",
								}
								statusCode = http.StatusBadRequest
							}
						} else {
							{ // this block extracts the image file if necessary
								switch rel.Type {
								case release.Text:
								case release.Image:
									fallthrough
								default:
									var fileName string
									var err error
									tmpFile, fileName, err = saveImageFromRequest(r, "image")
									switch err {
									case nil:
										s.Logger.Printf("image found on put request")
										defer os.Remove(tmpFile.Name())
										defer tmpFile.Close()
										s.Logger.Printf(fmt.Sprintf("temp file saved: %s", tmpFile.Name()))
										rel.Content = generateFileNameForStorage(fileName, "release")
										rel.Type = release.Image
									case errUnacceptedType:
										response.Data = jSendFailData{
											ErrorMessage: "image-type",
											ErrorReason:  "only types image/jpeg & image/png are accepted",
										}
										statusCode = http.StatusBadRequest
									case errReadingFromImage:
										s.Logger.Printf("image not found on put request")
										if rel.Type == release.Image {
											response.Data = jSendFailData{
												ErrorReason:  "image",
												ErrorMessage: "unable to read image file\nuse multipart-form for for posting Image Releases. A part named 'JSON' for Release data \nand a file called 'image' of image type JPG/PNG.",
											}
											statusCode = http.StatusBadRequest
										}
									default:
										response.Status = "error"
										response.Message = "server error when adding release"
										statusCode = http.StatusInternalServerError
									}
								}
							}
						}
					}
					// if JSON parsing doesn't fail
					if response.Data == nil {
						if rel.Content == "" && rel.Title == "" && rel.GenreDefining == "" &&
							rel.Description == "" && len(rel.Genres) == 0 && len(rel.Authors) == 0 &&
							rel.OwnerChannel == "" {
							//no patchable data found
							rel, err = s.ReleaseService.GetRelease(id)
							switch err {
							case nil:
								s.Logger.Printf("success patch release at id %d: no new data found on request", id)
								response.Status = "success"
								if rel.Type == release.Image {
									rel.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(rel.Content)
								}
								response.Data = *rel
							default:
								s.Logger.Printf("update of user failed because: %v", err)
								response.Status = "error"
								response.Message = "server error when updating user"
								statusCode = http.StatusInternalServerError
							}
						}
						if response.Data == nil {
							rel.ID = id
							rel, err = s.ReleaseService.UpdateRelease(rel)
							switch err {
							case nil:
								if rel.Type == release.Image {
									err := saveTempFilePermanentlyToPath(tmpFile, s.ImageStoragePath+rel.Content)
									if err != nil {
										s.Logger.Printf("updating of release failed because: %v", err)
										response.Status = "error"
										response.Message = "server error when updating release"
										statusCode = http.StatusInternalServerError
										_ = s.ReleaseService.DeleteRelease(rel.ID)
									}
								}
								if response.Message == "" {
									s.Logger.Printf("success updating release %d", id)
									response.Status = "success"
									if rel.Type == release.Image {
										rel.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(rel.Content)
									}
									response.Data = *rel
									// TODO delete old image if image updated
								}
							case release.ErrAttemptToChangeReleaseType:
								s.Logger.Printf("update attempt of release type for release %d", id)
								response.Data = jSendFailData{
									ErrorReason:  "type",
									ErrorMessage: "release type cannot be changed",
								}
								statusCode = http.StatusNotFound
							case release.ErrSomeReleaseDataNotPersisted:
								fallthrough
							default:
								s.Logger.Printf("update of release failed because: %v", err)
								response.Status = "error"
								response.Message = "server error when adding release"
								statusCode = http.StatusInternalServerError
							}
						}
					}
				case channel.ErrChannelNotFound:
					s.Logger.Printf("replease post attempt on non-existent channel %s", temp.OwnerChannel)
					response.Data = jSendFailData{
						ErrorReason:  "ownerChannel",
						ErrorMessage: fmt.Sprintf("channel of channelUsername %s not found", temp.OwnerChannel),
					}
					statusCode = http.StatusNotFound
				default:
					s.Logger.Printf("patching of release failed during auth because: %v", err)
					response.Status = "error"
					response.Message = "server error when adding release"
				}

			case release.ErrReleaseNotFound:
				s.Logger.Printf("update attempt of non existing release %s", idRaw)
				response.Data = jSendFailData{
					ErrorReason:  "releaseID",
					ErrorMessage: fmt.Sprintf("release of id %s not found", idRaw),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("patching of release failed during auth because: %v", err)
				response.Status = "error"
				response.Message = "server error when adding release"
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// deleteRelease returns a handler for DELETE /releases/{id} requests
func deleteRelease(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["id"]

		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("delete attempt of non invalid release id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "releaseID",
				ErrorMessage: fmt.Sprintf("invalid releaseID %d", id),
			}
			statusCode = http.StatusBadRequest
		} else {
			temp, err := s.ReleaseService.GetRelease(id)
			switch err {
			case nil:
				{ // this block secure the route
					c, err := s.ChannelService.GetChannel(temp.OwnerChannel)
					switch err {
					case nil:
						isAdmin := false
						for _, admin := range c.AdminUsernames {
							if r.Header.Get("authorized_username") == admin {
								isAdmin = true
								break
							}
						}
						if !isAdmin {
							s.Logger.Printf("unauthorized add release request")
							addCors(w)
							w.WriteHeader(http.StatusUnauthorized)
							return
						}
						// TODO delete image if image type
						err = s.ReleaseService.DeleteRelease(id)
						switch err {
						case nil:
							fallthrough
						case release.ErrReleaseNotFound:
							response.Status = "success"
							s.Logger.Printf("success deleting release %d", id)
							statusCode = http.StatusOK
						default:
							s.Logger.Printf("deletion of release failed because: %v", err)
							response.Status = "error"
							response.Message = "server error when adding release"
							statusCode = http.StatusInternalServerError
						}
					case channel.ErrChannelNotFound:
						response.Status = "success"
						s.Logger.Printf("success deleting release %d", id)
						statusCode = http.StatusOK
					default:
						s.Logger.Printf("deletion of release failed during auth because: %v", err)
						response.Status = "error"
						response.Message = "server error when adding release"
					}
				}
			case release.ErrReleaseNotFound:
				response.Status = "success"
				s.Logger.Printf("success deleting release %d", id)
				statusCode = http.StatusOK
			default:
				s.Logger.Printf("patching of release failed during auth because: %v", err)
				response.Status = "error"
				response.Message = "server error when adding release"
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}
