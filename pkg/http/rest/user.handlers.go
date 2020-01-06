package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slim-crown/issue-1-REST/pkg/domain/user"

	"github.com/gorilla/mux"
)

func sanitizeUser(u *user.User, s *Setup) {
	u.Username = s.StrictSanitizer.Sanitize(u.Username)
	// TODO validate email
	u.FirstName = s.StrictSanitizer.Sanitize(u.FirstName)
	u.MiddleName = s.StrictSanitizer.Sanitize(u.MiddleName)
	u.LastName = s.StrictSanitizer.Sanitize(u.LastName)
	u.Bio = s.StrictSanitizer.Sanitize(u.Bio)
}

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
				{"username":"username len 5-22 chars",
				"passHash":"passHash",
				"email":"email",
				"firstName":"firstName",
				"middleName":"middleName",
				"lastName":"lastName"}`,
					}
					s.Logger.Log("bad update user request")
					statusCode = http.StatusBadRequest
				}
			}
		}

		sanitizeUser(u, s)

		if response.Data == nil {
			// this block checks for required fields
			if u.FirstName == "" {
				response.Data = jSendFailData{
					ErrorReason:  "firstName",
					ErrorMessage: "firstName is required",
				}
			}
			if u.Password == "" {
				response.Data = jSendFailData{
					ErrorReason:  "password",
					ErrorMessage: "password is required",
				}
			}
			if u.Username == "" {
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: "username is required",
				}
			} else {
				if len(u.Username) > 22 || len(u.Username) < 5 {
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username length shouldn't be shorter that 5 and longer than 22 chars",
					}
				}
			}
			if response.Data == nil {
				s.Logger.Log("trying to add user %s", u.Username, u.Email, u.FirstName, u.MiddleName, u.LastName, u.Password)
				u, err := s.UserService.AddUser(u)
				switch err {
				case nil:
					response.Status = "success"
					response.Data = *u
					s.Logger.Log("success adding user %s", u.Username, u.Email, u.FirstName, u.MiddleName, u.LastName, u.Password)
				case user.ErrUserNameOccupied:
					s.Logger.Log("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username is occupied",
					}
					statusCode = http.StatusConflict
				case user.ErrEmailIsOccupied:
					s.Logger.Log("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "email",
						ErrorMessage: "email is occupied",
					}
					statusCode = http.StatusConflict
				case user.ErrSomeUserDataNotPersisted:
					fallthrough
				default:
					_ = s.UserService.DeleteUser(u.Username)
					s.Logger.Log("adding of user failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when adding user"
					statusCode = http.StatusInternalServerError
				}
			} else {
				// if required fields aren't present
				s.Logger.Log("bad adding user request")
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
		vars := mux.Vars(r)
		username := vars["username"]

		s.Logger.Log("trying to fetch user %s", username)

		u, err := s.UserService.GetUser(username)
		switch err {
		case nil:
			response.Status = "success"
			{ // this block sanitizes the returned User if it's not the user herself accessing the route
				if username != r.Header.Get("authorized_username") {
					s.Logger.Log(fmt.Sprintf("user %s fetched user %s", r.Header.Get("authorized_username"), u.Username))
					u.Email = ""
					u.BookmarkedPosts = make(map[int]time.Time)
				}
			}
			if u.PictureURL != "" {
				u.PictureURL = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
			}
			response.Data = *u
			s.Logger.Log("success fetching user %s", username)
		case user.ErrUserNotFound:
			s.Logger.Log("fetch attempt of non existing user %s", username)
			response.Data = jSendFailData{
				ErrorReason:  "username",
				ErrorMessage: fmt.Sprintf("user of username %s not found", username),
			}
			statusCode = http.StatusNotFound
		default:
			s.Logger.Log("fetching of user failed because: %v", err)
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
		var sortBy user.SortBy
		var sortOrder user.SortOrder

		{ // this block reads the query strings if any
			pattern = r.URL.Query().Get("pattern")

			if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
				limit, err = strconv.Atoi(limitPageRaw)
				if err != nil || limit < 0 {
					s.Logger.Log("bad get users request, limit")
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
					s.Logger.Log("bad request, offset")
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
			case "firstname":
				sortBy = user.SortByFirstName
			case "lastname":
				sortBy = user.SortByLastName
			default:
				sortBy = user.SortCreationTime
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
				s.Logger.Log("fetching of users failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when getting users"
				statusCode = http.StatusInternalServerError
			} else {
				response.Status = "success"
				for _, u := range users {
					u.Email = ""
					u.BookmarkedPosts = make(map[int]time.Time)
					if u.PictureURL != "" {
						u.PictureURL = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
					}
				}
				response.Data = users
				s.Logger.Log("success fetching users")
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

		vars := mux.Vars(r)
		username := vars["username"]

		{ // this block blocks user updating of user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				if _, err := s.UserService.GetUser(username); err == nil {
					s.Logger.Log("unauthorized update user attempt")
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
				{"username":"username len 5-22 chars",
				"passHash":"passHash",
				"email":"email",
				"firstName":"firstName",
				"middleName":"middleName",
				"lastName":"lastName"}`,
			}
			s.Logger.Log("bad update user request")
			statusCode = http.StatusBadRequest
		}
		if response.Data == nil {
			// if JSON parsing doesn't fail

			sanitizeUser(u, s)

			if u.FirstName == "" && u.Username == "" && u.Bio == "" && u.Email == "" && u.LastName == "" && u.MiddleName == "" && u.Password == "" {
				response.Data = jSendFailData{
					ErrorReason:  "request",
					ErrorMessage: "request doesn't contain updatable data",
				}
				statusCode = http.StatusBadRequest
			} else {
				u, err = s.UserService.UpdateUser(u, username)
				switch err {
				case nil:
					s.Logger.Log("success put user %s", username, u.Username, u.Password, u.Email, u.FirstName, u.MiddleName, u.LastName)
					response.Status = "success"
					response.Data = *u
				case user.ErrUserNameOccupied:
					s.Logger.Log("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username is occupied by a channel",
					}
					statusCode = http.StatusConflict
				case user.ErrEmailIsOccupied:
					s.Logger.Log("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "email",
						ErrorMessage: "email is occupied",
					}
					statusCode = http.StatusConflict
				case user.ErrInvalidUserData:
					s.Logger.Log("adding of user failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "request",
						ErrorMessage: "user must have email & password to be created",
					}
					statusCode = http.StatusBadRequest
				case user.ErrSomeUserDataNotPersisted:
					fallthrough
				default:
					_ = s.UserService.DeleteUser(u.Username)
					s.Logger.Log("update of user failed because: %v", err)
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

		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized update user attempt")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		s.Logger.Log("trying to delete user %s", username)
		err := s.UserService.DeleteUser(username)
		if err != nil {
			s.Logger.Log("deletion of user failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when deleting user"
			statusCode = http.StatusInternalServerError
		} else {
			response.Status = "success"
			s.Logger.Log("success deleting user %s", username)
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
		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized get user bookmarks request")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		u, err := s.UserService.GetUser(username)
		switch err {
		case nil:
			response.Status = "success"
			response.Data = u.BookmarkedPosts
			s.Logger.Log("success fetching user %s", username)
		case user.ErrUserNotFound:
			s.Logger.Log("fetch attempt of non existing user %s", username)
			response.Data = jSendFailData{
				ErrorReason:  "username",
				ErrorMessage: fmt.Sprintf("user of username %s not found", username),
			}
			statusCode = http.StatusNotFound
		default:
			s.Logger.Log("fetching of user bookmarks failed because: %v", err)
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
		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized post user bookmarks request")
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
					s.Logger.Log("bad bookmark post request")
					response.Data = jSendFailData{
						ErrorReason:  "postID",
						ErrorMessage: "bad request, postID must be an integer",
					}
					statusCode = http.StatusBadRequest
				}
			} else {
				err := json.NewDecoder(r.Body).Decode(&post)
				if err != nil {
					s.Logger.Log("bad bookmark post request")
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
		s.Logger.Log(fmt.Sprintf("bookmarking post: %v", post))
		if response.Data == nil {
			err := s.UserService.BookmarkPost(username, post.PostID)
			switch err {
			case nil:
				s.Logger.Log(fmt.Sprintf("success adding bookmark %d to user %s", post.PostID, username))
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			case user.ErrPostNotFound:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of id %d not found", post.PostID),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
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
		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block secures the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized put user bookmarks request")
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
			s.Logger.Log("bad put bookmark post request")
			statusCode = http.StatusBadRequest
		}
		// if queries are clean
		if response.Data == nil {
			err := s.UserService.BookmarkPost(username, postID)
			switch err {
			case nil:
				s.Logger.Log(fmt.Sprintf("success adding bookmark %d to user %s", postID, username))
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			case user.ErrPostNotFound:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of id %d not found", postID),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
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
		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block secures the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized delete bookmarks attempt")
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
			s.Logger.Log("bad delete bookmark post request")
			statusCode = http.StatusBadRequest
		}
		// if queries are clean
		if response.Data == nil {
			err = s.UserService.DeleteBookmark(username, postID)
			switch err {
			case nil:
				s.Logger.Log(fmt.Sprintf("success removing bookmark %d from user %s", postID, username))
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Log(fmt.Sprintf("deletion of bookmark failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Log(fmt.Sprintf("deletion of bookmark failed because: %v", err))
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

		vars := mux.Vars(r)
		username := vars["username"]

		u, err := s.UserService.GetUser(username)
		switch err {
		case nil:
			response.Status = "success"
			response.Data = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
			s.Logger.Log("success fetching user %s picture URL", username)
		case user.ErrUserNotFound:
			s.Logger.Log("fetch picture URL attempt of non existing user %s", username)
			response.Data = jSendFailData{
				ErrorReason:  "username",
				ErrorMessage: fmt.Sprintf("user of username %s not found", username),
			}
			statusCode = http.StatusNotFound
		default:
			s.Logger.Log("fetching of user picture URL failed because: %v", err)
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
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"
		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block secures the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized user picture setting request")
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
				s.Logger.Log("image found on put user picture request")
				defer os.Remove(tmpFile.Name())
				defer tmpFile.Close()
				s.Logger.Log(fmt.Sprintf("temp file saved: %s", tmpFile.Name()))
				fileName = generateFileNameForStorage(fileName, "user")
			case errUnacceptedType:
				response.Data = jSendFailData{
					ErrorMessage: "image",
					ErrorReason:  "only types image/jpeg & image/png are accepted",
				}
				statusCode = http.StatusBadRequest
			case errReadingFromImage:
				s.Logger.Log("image not found on put request")
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
					s.Logger.Log("adding of release failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when setting user picture"
					statusCode = http.StatusInternalServerError
					_ = s.UserService.RemovePicture(username)
				} else {
					s.Logger.Log(fmt.Sprintf("success adding picture %s to user %s", fileName, username))
					response.Status = "success"
					response.Data = s.HostAddress + s.ImageServingRoute + url.PathEscape(fileName)
				}
			case user.ErrUserNotFound:
				s.Logger.Log(fmt.Sprintf("adding of user picture failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Log(fmt.Sprintf("bookmarking of post failed because: %v", err))
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
		vars := mux.Vars(r)
		username := vars["username"]
		{ // this block blocks user deletion of a user if is not the user herself accessing the route
			if username != r.Header.Get("authorized_username") {
				s.Logger.Log("unauthorized delete user picture attempt")
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
				s.Logger.Log(fmt.Sprintf("success removing piture from user %s", username))
				response.Status = "success"
			case user.ErrUserNotFound:
				s.Logger.Log(fmt.Sprintf("deletion of user pictre failed because: %v", err))
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: fmt.Sprintf("user of username %s not found", username),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Log(fmt.Sprintf("deletion of user pictre failed because: %v", err))
				response.Status = "error"
				response.Message = "server error when removing user picture"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}
