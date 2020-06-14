package rest

import (
	//"bytes"
	"encoding/json"
	"fmt"
	//"io"

	// "html"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	// "github.com/microcosm-cc/bluemonday"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/user"
)

func sanitizeUser(u *user.User, s *Setup) {
	u.Username = s.StrictSanitizer.Sanitize(u.Username)
	// TODO validate email
	u.FirstName = s.StrictSanitizer.Sanitize(u.FirstName)
	u.MiddleName = s.StrictSanitizer.Sanitize(u.MiddleName)
	u.LastName = s.StrictSanitizer.Sanitize(u.LastName)
	u.Bio = s.StrictSanitizer.Sanitize(u.Bio)
	/* u.Username = html.EscapeString(u.Username)
	// TODO validate email
	u.FirstName = html.EscapeString(u.FirstName)
	u.MiddleName = html.EscapeString(u.MiddleName)
	u.LastName = html.EscapeString(u.LastName)
	u.Bio = html.EscapeString(u.Bio) */

}

var emailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

var usernameRX = regexp.MustCompile("^[a-zA-Z]?(?:[_]?[a-zA-Z0-9])*$")

// postUser returns a handler for POST /users requests
func postUser(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK
		u := new(user.User)

		{ // checks if requests uses forms or JSON and parses then
			u.Username = r.FormValue("username")
			if u.Username != "" {
				u.Password = r.FormValue("password")
				u.Email = r.FormValue("email")
				u.FirstName = r.FormValue("firstName")
				u.MiddleName = r.FormValue("middleName")
				u.LastName = r.FormValue("lastName")
			} else {
				err := json.NewDecoder(r.Body).Decode(u)
				if err != nil {
					response.Data = jSendFailData{
						ErrorReason: "request format",
						ErrorMessage: `bad request, use format
				{"username":"username len 5-24 chars",
				"passHash":"passHash",
				"email":"email",
				"firstName":"firstName",
				"middleName":"middleName",
				"lastName":"lastName"}`,
					}
					s.Logger.Printf("bad update user request")
					statusCode = http.StatusBadRequest
				}
			}
		}

		sanitizeUser(u, s)

		if response.Data == nil {
			switch {
			// this block checks for required fields
			case u.Username == "":
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: "username is required",
				}
			case len(u.Username) > 24 || len(u.Username) < 5:
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: "username length shouldn't be shorter that 5 and longer than 24 chars",
				}
			case !usernameRX.MatchString(u.Username):
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("username given not valid given is not valid, use pattern %s", usernameRX.String()),
				}
			case u.Password == "":
				response.Data = jSendFailData{
					ErrorReason:  "password",
					ErrorMessage: "password is required",
				}
			case len(u.Password) < 8:
				response.Data = jSendFailData{
					ErrorReason:  "password",
					ErrorMessage: "password length shouldn't be shorter that 8 chars",
				}
			case u.Email == "":
				response.Data = jSendFailData{
					ErrorReason:  "email",
					ErrorMessage: "email is required",
				}
			case !emailRX.MatchString(u.Email):
				response.Data = jSendFailData{
					ErrorReason:  "email",
					ErrorMessage: "email given is not valid",
				}
			case u.FirstName == "":
				response.Data = jSendFailData{
					ErrorReason:  "firstName",
					ErrorMessage: "firstName is required",
				}
			}
			if response.Data == nil {
				s.Logger.Printf("trying to add user %+v", u)
				username := u.Username
				u, err := s.UserService.AddUser(u)
				switch err {
				case nil:
					response.Status = "success"
					response.Data = *u
					s.Logger.Printf("success adding user %+v", u)
				case user.ErrUserNameOccupied:
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username is occupied",
					}
					statusCode = http.StatusConflict
				case user.ErrEmailIsOccupied:
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "email",
						ErrorMessage: "email is occupied",
					}
					statusCode = http.StatusConflict
				case user.ErrSomeUserDataNotPersisted:
					fallthrough
				default:
					_ = s.UserService.DeleteUser(username)
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when adding user"
					statusCode = http.StatusInternalServerError
				}
			} else {
				// if required fields aren't present
				s.Logger.Printf("bad adding user request")
				statusCode = http.StatusBadRequest
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getUser returns a handler for GET /users/{username} requests
func getUser(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		s.Logger.Printf("trying to fetch user %s", username)

		u, err := s.UserService.GetUser(username)
		switch err {
		case nil:
			response.Status = "success"
			{ // this block sanitizes the returned User if it's not the user herself accessing the route
				if username != r.Header.Get("authorized_username") {
					s.Logger.Printf("user %s fetched user %s", r.Header.Get("authorized_username"), u.Username)
					u.Email = ""
					u.BookmarkedPosts = nil
				}
			}
			if u.PictureURL != "" {
				u.PictureURL = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
			}
			response.Data = *u
			s.Logger.Printf("success fetching user %s", username)
		case user.ErrUserNotFound:
			s.Logger.Printf("fetch attempt of non existing user %s", username)
			response.Data = jSendFailData{
				ErrorReason:  "username",
				ErrorMessage: fmt.Sprintf("user of username %s not found", username),
			}
			statusCode = http.StatusNotFound
		default:
			s.Logger.Printf("fetching of user failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when fetching user"
			statusCode = http.StatusInternalServerError
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getUsers returns a handler for GET /users?sort=new&limit=5&offset=0&pattern=Joe requests
func getUsers(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		pattern := ""
		limit := 25
		offset := 0
		sortBy := user.SortByCreationTime
		sortOrder := user.SortDescending

		{ // this block reads the query strings if any
			pattern = r.URL.Query().Get("pattern")

			if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
				limit, err = strconv.Atoi(limitPageRaw)
				if err != nil || limit < 0 {
					s.Logger.Printf("bad get users request, limit")
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

			sortOrder = user.SortAscending
			switch sortByQuery := sortSplit[0]; sortByQuery {
			case "username":
				sortBy = user.SortByUsername
			case "first-name":
				sortBy = user.SortByFirstName
			case "last-name":
				sortBy = user.SortByLastName
			default:
				sortBy = user.SortByCreationTime
				sortOrder = user.SortDescending
			}
			if len(sortSplit) > 1 {
				switch sortOrderQuery := sortSplit[1]; sortOrderQuery {
				case "dsc":
					sortOrder = user.SortDescending
				default:
					sortOrder = user.SortAscending
				}
			}

		}
		// if queries are clean
		if response.Data == nil {
			users, err := s.UserService.SearchUser(pattern, sortBy, sortOrder, limit, offset)
			if err != nil {
				s.Logger.Printf("fetching of users failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when getting users"
				statusCode = http.StatusInternalServerError
			} else {
				response.Status = "success"
				for _, u := range users {
					u.Email = ""
					u.BookmarkedPosts = nil
					if u.PictureURL != "" {
						u.PictureURL = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
					}
				}
				response.Data = users
				s.Logger.Printf("success fetching users")
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// postUser returns a handler for PUT /users/{username} requests
func putUser(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block blocks user updating of user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				if _, err := s.UserService.GetUser(username); err == nil {
					s.Logger.Printf("unauthorized update user attempt")
					addCors(w)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
		}
		u := new(user.User)
		err := json.NewDecoder(r.Body).Decode(u)
		if err != nil {
			response.Data = jSendFailData{
				ErrorReason: "request format",
				ErrorMessage: `bad request, use format
				{"username":"username len 5-24 chars",
				"passHash":"passHash",
				"email":"email",
				"firstName":"firstName",
				"middleName":"middleName",
				"lastName":"lastName"}`,
			}
			s.Logger.Printf("bad update user request")
			statusCode = http.StatusBadRequest
		}
		if response.Data == nil {
			// if JSON parsing doesn't fail

			sanitizeUser(u, s)

			switch {
			case u.FirstName == "" && u.Username == "" && u.Bio == "" && u.Email == "" &&
				u.LastName == "" && u.MiddleName == "" && u.Password == "":
				// no update able data
				u, err = s.UserService.GetUser(username)
				switch err {
				case nil:
					s.Logger.Printf("success put user at user %s data %v", username, u)
					response.Status = "success"
					response.Data = *u
				default:
					s.Logger.Printf("update of user failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when updating user"
					statusCode = http.StatusInternalServerError
				}
			case u.Username != "" || u.Email != "":
				if u.Username != "" {
					if len(u.Username) > 24 || len(u.Username) < 5 {
						response.Data = jSendFailData{
							ErrorReason:  "username",
							ErrorMessage: "username length shouldn't be shorter that 5 and longer than 24 chars",
						}
						break
					}
					if !usernameRX.MatchString(u.Username) {
						response.Data = jSendFailData{
							ErrorReason:  "username",
							ErrorMessage: fmt.Sprintf("username given not valid given is not valid, use pattern %s", usernameRX.String()),
						}
						break
					}
				}
				if u.Email != "" {
					if !emailRX.MatchString(u.Email) {
						response.Data = jSendFailData{
							ErrorReason:  "email",
							ErrorMessage: "email given is not valid",
						}
						break
					}
				}
				if u.Password != "" {
					if len(u.Password) < 8 {
						response.Data = jSendFailData{
							ErrorReason:  "password",
							ErrorMessage: "password length shouldn't be shorter that 8 chars",
						}
						break
					}
				}
				fallthrough
			default:
				u, err = s.UserService.UpdateUser(u, username)
				switch err {
				case nil:
					s.Logger.Printf("success put user at user %s data %v", username, u)
					response.Status = "success"
					response.Data = *u
				case user.ErrUserNotFound:
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username not found",
					}
					statusCode = http.StatusConflict
				case user.ErrUserNameOccupied:
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username is occupied by a channel",
					}
					statusCode = http.StatusConflict
				case user.ErrEmailIsOccupied:
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "email",
						ErrorMessage: "email is occupied",
					}
					statusCode = http.StatusConflict
				case user.ErrInvalidUserData:
					s.Logger.Printf("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "request",
						ErrorMessage: "user must have email & password to be created",
					}
					statusCode = http.StatusBadRequest
				case user.ErrSomeUserDataNotPersisted:
					fallthrough
				default:
					s.Logger.Printf("update of user failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when updating user"
					statusCode = http.StatusInternalServerError
				}
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// deleteUser returns a handler for DELETE /users/{username} requests
func deleteUser(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized delete user attempt")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		s.Logger.Printf("trying to delete user %s", username)
		err := s.UserService.DeleteUser(username)
		if err != nil {
			s.Logger.Printf("deletion of user failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when deleting user"
			statusCode = http.StatusInternalServerError
		} else {
			response.Status = "success"
			s.Logger.Printf("success deleting user %s", username)
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getUserBookmarks returns a handler for GET /users/{username}/bookmarks requests
func getUserBookmarks(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized get user bookmarks request")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		u, err := s.UserService.GetUser(username)
		switch err {
		case nil:
			response.Status = "success"
			bookmarks := make(map[time.Time]interface{})
			for t, id := range u.BookmarkedPosts {
				if temp, err := s.PostService.GetPost((uint(id))); err == nil {
					//tempPost := post.Post{
					//	ID:               temp.ID,
					//	PostedByUsername: temp.PostedByUsername,
					//	OriginChannel:    temp.OriginChannel,
					//	Title:            temp.Title,
					//	Description:      temp.Description,
					//	CreationTime:     time.Time{},
					//}
					bookmarks[t] = temp
				} else {
					bookmarks[t] = id
				}
			}
			response.Data = bookmarks
			s.Logger.Printf("success fetching user %s", username)
		case user.ErrUserNotFound:
			s.Logger.Printf("fetch attempt of non existing user %s", username)
			response.Data = jSendFailData{
				ErrorReason:  "username",
				ErrorMessage: fmt.Sprintf("user of username %s not found", username),
			}
			statusCode = http.StatusNotFound
		default:
			s.Logger.Printf("fetching of user bookmarks failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when fetching user bookmarks"
			statusCode = http.StatusInternalServerError
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// postUserBookmarks returns a handler for POST /users/{username}/bookmarks/ requests
func postUserBookmarks(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized post user bookmarks request")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		var post struct {
			PostID int `json:"postID"`
		}
		{ // this block extracts post ID from the request
			temp := r.FormValue("postID")
			if temp != "" {
				post.PostID, err = strconv.Atoi(temp)
				if err != nil {
					s.Logger.Printf("bad bookmark post request")
					response.Data = jSendFailData{
						ErrorReason:  "postID",
						ErrorMessage: "bad request, postID must be an integer",
					}
					statusCode = http.StatusBadRequest
				}
			} else {
				err := json.NewDecoder(r.Body).Decode(&post)
				if err != nil {
					s.Logger.Printf("bad bookmark post request")
					response.Data = jSendFailData{
						ErrorReason: "request format",
						ErrorMessage: `bad request, use format
										{"postID":"postID"}`,
					}
					statusCode = http.StatusBadRequest
				}
			}
		}
		// if queries are clean
		s.Logger.Printf("bookmarking post: %v", post)
		if response.Data == nil {
			err := s.UserService.BookmarkPost(username, post.PostID)
			switch err {
			case nil:
				s.Logger.Printf("success adding bookmark %d to user %s", post.PostID, username)
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			case user.ErrPostNotFound:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of id %d not found", post.PostID),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when bookmarking post"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// putUserBookmarks returns a handler for PUT /users/{username}/bookmarks/{postID} requests
func putUserBookmarks(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block secures the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized put user bookmarks request")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		postID, err := strconv.Atoi(vars["postID"])
		if err != nil {
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: "bad request, postID must be an integer",
			}
			s.Logger.Printf("bad put bookmark post request")
			statusCode = http.StatusBadRequest
		}
		// if queries are clean
		if response.Data == nil {
			err := s.UserService.BookmarkPost(username, postID)
			switch err {
			case nil:
				s.Logger.Printf("success adding bookmark %d to user %s", postID, username)
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			case user.ErrPostNotFound:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of id %d not found", postID),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when putting using bookmark"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// deleteUserBookmarks returns a handler for DELETE /users/{username}/bookmarks/{postID} requests
func deleteUserBookmarks(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block secures the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized delete bookmarks attempt")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		postID, err := strconv.Atoi(vars["postID"])
		if err != nil {
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: "bad request, postID must be an integer",
			}
			s.Logger.Printf("bad delete bookmark post request")
			statusCode = http.StatusBadRequest
		}
		// if queries are clean
		if response.Data == nil {
			err = s.UserService.DeleteBookmark(username, postID)
			switch err {
			case nil:
				s.Logger.Printf("success removing bookmark %d from user %s", postID, username)
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Printf("deletion of bookmark failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("deletion of bookmark failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when deleting user bookmark"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getUserPicture returns a handler for GET /users/{username}/picture requests
func getUserPicture(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		u, err := s.UserService.GetUser(username)
		switch err {
		case nil:
			response.Status = "success"
			response.Data = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
			s.Logger.Printf("success fetching user %s picture URL", username)
		case user.ErrUserNotFound:
			s.Logger.Printf("fetch picture URL attempt of non existing user %s", username)
			response.Data = jSendFailData{
				ErrorReason:  "username",
				ErrorMessage: fmt.Sprintf("user of username %s not found", username),
			}
			statusCode = http.StatusNotFound
		default:
			s.Logger.Printf("fetching of user picture URL failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when fetching user picture URL"
			statusCode = http.StatusInternalServerError
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// putUserPicture returns a handler for PUT /users/{username}/picture requests
func putUserPicture(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		/*{
			buf := new(bytes.Buffer)
			_, _= io.Copy(buf, r.Body)
			fmt.Printf("Body:\n %s", string(buf.Bytes()))
		}
		return
		*/

		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block secures the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized user picture setting request")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		var tmpFile *os.File
		var fileName string
		{ // this block extracts the image
			tmpFile, fileName, err = saveImageFromRequest(r, "image")
			switch err {
			case nil:
				s.Logger.Printf("image found on put user picture request")
				defer os.Remove(tmpFile.Name())
				defer tmpFile.Close()
				s.Logger.Printf("temp file saved: %s", tmpFile.Name())
				fileName = generateFileNameForStorage(fileName, "user")
			case errUnacceptedType:
				response.Data = jSendFailData{
					ErrorMessage: "image",
					ErrorReason:  "only types image/jpeg & image/png are accepted",
				}
				statusCode = http.StatusBadRequest
			case errReadingFromImage:
				s.Logger.Printf("image not found on put request")
				response.Data = jSendFailData{
					ErrorReason:  "image",
					ErrorMessage: "unable to read image file\nuse multipart-form for for posting user pictures. A form that contains the file under the key 'image', of image type JPG/PNG.",
				}
				statusCode = http.StatusBadRequest
			default:
				response.Status = "error"
				response.Message = "server error when adding user picture"
				statusCode = http.StatusInternalServerError
			}
		}
		// if queries are clean
		if response.Data == nil {
			err := s.UserService.AddPicture(username, fileName)
			switch err {
			case nil:
				err := saveTempFilePermanentlyToPath(tmpFile, s.ImageStoragePath+fileName)
				if err != nil {
					s.Logger.Printf("adding of release failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when setting user picture"
					statusCode = http.StatusInternalServerError
					_ = s.UserService.RemovePicture(username)
				} else {
					s.Logger.Printf("success adding picture %s to user %s", fileName, username)
					response.Status = "success"
					response.Data = s.HostAddress + s.ImageServingRoute + url.PathEscape(fileName)
				}
			case user.ErrUserNotFound:
				s.Logger.Printf("adding of user picture failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("bookmarking of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when setting user picture"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// deleteUserPicture returns a handler for DELETE /users/{username}/picture requests
func deleteUserPicture(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		username := vars["username"]

		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Printf("unauthorized delete user picture attempt")
				addCors(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		// if queries are clean
		if response.Data == nil {
			err = s.UserService.RemovePicture(username)
			switch err {
			case nil:
				// TODO delete picture from fs
				s.Logger.Printf("success removing piture from user %s", username)
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Printf("deletion of user pictre failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("deletion of user pictre failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when removing user picture"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}
