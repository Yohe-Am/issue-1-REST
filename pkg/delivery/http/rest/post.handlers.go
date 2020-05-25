package rest

import (
	"gopkg.in/russross/blackfriday.v2"
	"fmt"

	// "html"
	"net/http"
	"strconv"
	"strings"

	"encoding/json"

	"github.com/slim-crown/issue-1-REST/pkg/services/domain/post"
)

func sanitizePost(p *post.Post, s *Setup) {
	p.PostedByUsername = s.StrictSanitizer.Sanitize(p.PostedByUsername)
	p.OriginChannel = s.StrictSanitizer.Sanitize(p.OriginChannel)
	p.Title = s.StrictSanitizer.Sanitize(p.Title)
	p.Description = string(s.MarkupSanitizer.SanitizeBytes(
		blackfriday.Run(
			[]byte(p.Description),
			blackfriday.WithExtensions(blackfriday.CommonExtensions),
		),
	))
	if p.Description == "<p></p>\n" {
		p.Description = ""
	}
	/* p.PostedByUsername = html.EscapeString(p.PostedByUsername)
	p.OriginChannel = html.EscapeString(p.OriginChannel)
	p.Title = html.EscapeString(p.Title)
	p.Description = html.EscapeString(p.Description) */
}

// GET: /posts/:id ...getpost(id)
// getPost returns a handler for GET /posts/{id} requests
func getPost(d *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]
		id, err := strconv.Atoi(idRaw)
		if err != nil {
			d.Logger.Printf("fetch attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		}

		if response.Data == nil {
			id := uint(id)
			d.Logger.Printf("trying to fetch Post %d", id)
			rel, err := d.PostService.GetPost(id)
			switch err {
			case nil:
				response.Status = "success"
				response.Data = *rel
				d.Logger.Printf("success fetching post %d", id)
			case post.ErrPostNotFound:
				d.Logger.Printf("fetching of post failed because:Post not found")
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of postID %d not found", id),
				}
				statusCode = http.StatusNotFound
			default:
				d.Logger.Printf("fetching of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching post"
				statusCode = http.StatusInternalServerError

			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// POST: /posts/.....addpost
// postPost returns a handler for POST: /posts/.....{addpost}
func postPost(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		newPost := new(post.Post)
		{ // checks if requests uses forms or JSON and parses then
			newPost.PostedByUsername = r.FormValue("username")
			if newPost.PostedByUsername != "" {
				newPost.Title = r.FormValue("title")
				newPost.Description = r.FormValue("description")
				newPost.OriginChannel = r.FormValue("channelName")

			} else {
				err := json.NewDecoder(r.Body).Decode(newPost)
				if err != nil {
					response.Data = jSendFailData{
						ErrorReason: "request format",
						ErrorMessage: `bad request, use format
				{"postedByUsername":"username len 5-22 chars",
				"originChannel":"channel",
				"title":"title",
				"description":"description"
				}`,
					}
					s.Logger.Printf("bad update post request")
					statusCode = http.StatusBadRequest
				}
			}
		}

		if response.Data == nil {

			if newPost.OriginChannel == "" {
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: "username is required",
				}
			}
			if newPost.Title == "" {
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: "username is required",
				}
			}
			if newPost.PostedByUsername == "" {
				response.Data = jSendFailData{
					ErrorReason:  "username",
					ErrorMessage: "username is required",
				}
			} else {
				if len(newPost.PostedByUsername) > 22 || len(newPost.PostedByUsername) < 5 {
					response.Data = jSendFailData{
						ErrorReason:  "username",
						ErrorMessage: "username length shouldn't be shorter that 5 and longer than 22 chars",
					}
				}
			}

			{ // this block secures the route
				if newPost.PostedByUsername != r.Header.Get("authorized_username") {
					s.Logger.Printf("unauthorized update post attempt")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				/*
					channel,err:=s.ChannelService.GetChannel(newPost.OriginChannel)
					posterFound:=false
					if err==nil{
						for _,val:= range channel.AdminUsernames{
							if val==newPost.PostedByUsername{
								posterFound=true
								break;
							}
						}
						if !posterFound{
							s.Logger.Printf("unauthorized update post attempt")
							w.WriteHeader(http.StatusUnauthorized)
							return
						}

					}else{
						s.Logger.Printf("bad update post request")
						w.WriteHeader(http.StatusBadRequest)
						return
					}*/

			}
			if response.Data == nil {
				sanitizePost(newPost, s)
				s.Logger.Printf("trying to add post %s %s %s %s", newPost.PostedByUsername, newPost.Title, newPost.OriginChannel, newPost.Description)
				pos, err := s.PostService.AddPost(newPost)
				switch err {
				case nil:
					response.Status = "success"
					response.Data = *pos
					s.Logger.Printf("success adding post %s %s %s %s", pos.PostedByUsername, pos.Title, pos.OriginChannel, pos.Description)
				default:
					s.Logger.Printf("adding of post failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when adding post"
					statusCode = http.StatusInternalServerError
				}
			} else {
				// if required fields aren't present
				s.Logger.Printf("bad adding post request")
				statusCode = http.StatusBadRequest
			}
		}
		writeResponseToWriter(response, w, statusCode)

	}

}

// PUT: /posts/:id ---
//putPost returns a handler for PUT /posts/{id} requests--{updatePost(p,id)}
func putPost(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]

		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("update attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		} else {
			id := uint(id)
			{ // this block blocks user updating of post if the poster didn't accessing the route

				x, err := s.PostService.GetPost(id)
				if err == nil {
					if x.PostedByUsername != r.Header.Get("authorized_username") {
						s.Logger.Printf("unauthorized update post attempt")
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					/*
						channel,err:=s.ChannelService.GetChannel(x.OriginChannel)
						posterFound:=false
						if err==nil{
							for _,val:= range channel.AdminUsernames{
								if val==x.PostedByUsername{
									posterFound=true
									break;
								}
							}
							if !posterFound{
								s.Logger.Printf("unauthorized update post attempt")
								w.WriteHeader(http.StatusUnauthorized)
								return
							}
						}else{
							s.Logger.Printf("bad update post request")
							w.WriteHeader(http.StatusBadRequest)
							return
						}
					*/
				} else {
					s.Logger.Printf("invalid update post request")
					w.WriteHeader(http.StatusNotFound)
					return
				}

			}

			newPost := new(post.Post)
			erro := json.NewDecoder(r.Body).Decode(newPost)
			if erro != nil {
				response.Data = jSendFailData{
					ErrorReason: "request format",
					ErrorMessage: `bad request, use format
			{"poster":"username len 5-22 chars",
			"originChannel":"channel",
			"title":"title",
			"description":"description"
			}`,
				}
				s.Logger.Printf("bad update post request")
				statusCode = http.StatusBadRequest
			}
			if response.Data == nil {
				// if JSON parsing doesn't fail

				if newPost.PostedByUsername == "" && newPost.OriginChannel == "" && newPost.Title == "" && newPost.Description == "" {
					response.Data = jSendFailData{
						ErrorReason:  "request",
						ErrorMessage: "request doesn't contain updatable data",
					}
					statusCode = http.StatusBadRequest
				} else {
					sanitizePost(newPost, s)
					pos, erron := s.PostService.UpdatePost(newPost, id)
					switch erron {
					case nil:
						s.Logger.Printf("success put post %s %s %s %s %s", idRaw, pos.PostedByUsername, pos.OriginChannel, pos.Title, pos.Description)
						response.Status = "success"
						response.Data = *pos

					case post.ErrPostNotFound:
						response.Status = "error"
						s.Logger.Printf("updation of Post failed because: %v", erron)
						response.Data = jSendFailData{
							ErrorReason:  "PostID",
							ErrorMessage: fmt.Sprintf("Post of id %d not found", id),
						}
						statusCode = http.StatusNotFound

					default:
						s.Logger.Printf("adding of post failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when adding post"
						statusCode = http.StatusInternalServerError
					}
				}
			}
			writeResponseToWriter(response, w, statusCode)
		}

	}
}

// DELETE: /posts/:id
// postPost returns a handler for DELETE: /posts/:id ---{DeletePost(id)}
func deletePost(d *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]

		id, err := strconv.Atoi(idRaw)
		if err != nil {
			d.Logger.Printf("delete attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		} else {
			id := uint(id)
			{ // this block blocks user deleting of post if the poster didn't accessing the route
				x, err := d.PostService.GetPost(id)
				if err == nil {
					if x.PostedByUsername != r.Header.Get("authorized_username") {
						d.Logger.Printf("unauthorized update post attempt")
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					/*
						channel,err:=d.ChannelService.GetChannel(x.OriginChannel)
						posterFound:=false
						if err==nil{
							for _,val:= range channel.AdminUsernames{
								if val==x.PostedByUsername{
									posterFound=true
									break;
								}
							}
							if !posterFound{
								d.Logger.Printf("unauthorized update post attempt")
								w.WriteHeader(http.StatusUnauthorized)
								return
							}
						}else{
							d.Logger.Printf("bad update post request")
							w.WriteHeader(http.StatusBadRequest)
							return
						}
					*/
				} else {
					d.Logger.Printf("invalid update post request")
					w.WriteHeader(http.StatusNotFound)
					return
				}

			}
			d.Logger.Printf("trying to delete Post %d", id)
			err := d.PostService.DeletePost(id)
			switch err {
			case nil:
				response.Status = "success"
				d.Logger.Printf("success deleting Post %d", id)
			case post.ErrPostNotFound:
				d.Logger.Printf("deletion of Post failed because: %v", err)
				response.Data = jSendFailData{
					ErrorReason:  "PostID",
					ErrorMessage: fmt.Sprintf("Post of id %d not found", id),
				}
				statusCode = http.StatusNotFound

			default:
				d.Logger.Printf("deletion of Post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when adding Post"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)

	}

}

// GET: /posts/:id/releases/
// getPostReleases returns a handler for GET: /posts/:id/releases requests ---{getPost}
func getPostReleases(d *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]
		id, err := strconv.Atoi(idRaw)
		if err != nil {
			d.Logger.Printf("fetch attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		}

		if response.Data == nil {
			id := uint(id)
			d.Logger.Printf("trying to fetch Post %d", id)
			pos, err := d.PostService.GetPost(id)
			switch err {
			case nil:
				response.Status = "success"
				pReleases := make([]interface{}, 0)
				for _, rID := range pos.ContentsID {
					if temp, err := d.ReleaseService.GetRelease(int(rID)); err == nil {
						pReleases = append(pReleases, temp)
					} else {
						pReleases = append(pReleases, rID)
					}
				}
				response.Data = pReleases

			case post.ErrPostNotFound:
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of postID %d not found", id),
				}
				statusCode = http.StatusNotFound
			default:
				d.Logger.Printf("fetching of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching post"
				statusCode = http.StatusInternalServerError

			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// GET: /posts/:id/comments/
// getPostComments returns a handler for GET: /posts/:id/comments requests ---{getPost}
func getPostComments(d *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]
		id, err := strconv.Atoi(idRaw)
		if err != nil {
			d.Logger.Printf("fetch attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		}

		if response.Data == nil {
			id := uint(id)
			d.Logger.Printf("trying to fetch Post %d", id)
			pos, err := d.PostService.GetPost(id)
			switch err {
			case nil:
				response.Status = "success"
				pComments := make([]interface{}, 0)
				for _, cID := range pos.CommentsID {
					if temp, err := d.CommentService.GetComment(cID); err == nil {
						pComments = append(pComments, temp)
					} else {
						pComments = append(pComments, cID)
					}
				}
				response.Data = pComments

			case post.ErrPostNotFound:
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of postID %d not found", id),
				}
				statusCode = http.StatusNotFound
			default:
				d.Logger.Printf("fetching of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching post"
				statusCode = http.StatusInternalServerError

			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// GET:/posts?sortBy=creation-time&sortOrder=asc&limit=25&offset=0&pattern=John
// getPosts returns a handler for GET /posts?sort=new&limit=5&offset=0&pattern=Joe requests---{searchPost}
func getPosts(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		pattern := ""
		limit := 25
		offset := 0
		var sortBy post.SortBy
		var sortOrder post.SortOrder

		{ // this block reads the query strings if any
			pattern = r.URL.Query().Get("pattern")

			if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
				limit, err = strconv.Atoi(limitPageRaw)
				if err != nil || limit < 0 {
					s.Logger.Printf("bad get posts request, limit")
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

			sortOrder = post.SortAscending
			switch sortByQuery := sortSplit[0]; sortByQuery {
			case "username":
				sortBy = post.SortByPoster
			case "channelname":
				sortBy = post.SortByChannel
			case "title":
				sortBy = post.SortByTitle
			default:
				sortBy = post.SortByCreationTime
				sortOrder = post.SortDescending
			}
			if len(sortSplit) > 1 {
				switch sortOrderQuery := sortSplit[1]; sortOrderQuery {
				case "dsc":
					sortOrder = post.SortDescending
				default:
					sortOrder = post.SortAscending
				}
			}

		}
		// if queries are clean
		if response.Data == nil {
			posts, err := s.PostService.SearchPost(pattern, sortBy, sortOrder, limit, offset)
			if err != nil {
				s.Logger.Printf("fetching of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when getting posts"
				statusCode = http.StatusInternalServerError
			} else {
				response.Status = "success"
				response.Data = posts
				s.Logger.Printf("success fetching posts")
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

//GET: /posts/:id/stars
// getPostStars returns a handler for GET: /posts/:id/stars requests
func getPostStars(d *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]
		id, err := strconv.Atoi(idRaw)
		if err != nil {
			d.Logger.Printf("fetch attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		}

		if response.Data == nil {
			id := uint(id)
			d.Logger.Printf("trying to fetch Post %d", id)
			pos, err := d.PostService.GetPost(id)
			switch err {
			case nil:

				response.Status = "success"
				pStars := make([]interface{}, 0)
				for username := range pos.Stars {
					if temp, err := d.PostService.GetPostStar(id, username); err == nil {
						pStars = append(pStars, temp)
					} else {
						pStars = append(pStars, username)
					}
				}
				response.Data = pStars
				d.Logger.Printf("success fetching stars of post %d", id)

			case post.ErrPostNotFound:
				d.Logger.Printf("fetching of post failed because: %v", post.ErrPostNotFound)
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("post of postID %d not found", id),
				}
				statusCode = http.StatusNotFound
			default:
				d.Logger.Printf("fetching of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching post"
				statusCode = http.StatusInternalServerError

			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

//GET: /posts/:id/stars/username
// getPostStar returns a handler for GET: /posts/:id/stars/username requests----{getPostStar}
func getPostStar(d *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]
		id, err := strconv.Atoi(idRaw)
		username := vars["username"]

		if err != nil {
			d.Logger.Printf("fetch attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		}

		if response.Data == nil {
			id := uint(id)
			d.Logger.Printf("trying to fetch Star of Post %d and username %s", id, username)
			st, err := d.PostService.GetPostStar(id, username)
			switch err {
			case nil:
				response.Status = "success"
				response.Data = *st
				d.Logger.Printf("success fetch Star of Post %d and username %s", id, username)

			case post.ErrPostNotFound:
				response.Data = jSendFailData{
					ErrorReason:  "postID",
					ErrorMessage: fmt.Sprintf("star of postID %d not found", id),
				}
				statusCode = http.StatusNotFound
				d.Logger.Printf("star of postID %d not found", id)
			case post.ErrStarNotFound:
				response.Data = jSendFailData{
					ErrorReason:  "star",
					ErrorMessage: fmt.Sprintf("star of username %s and post id %d not found", username, id),
				}
				statusCode = http.StatusNotFound
				d.Logger.Printf("star of username %s and post id %d not found", username, id)
			default:
				d.Logger.Printf("fetching of post failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching post"
				statusCode = http.StatusInternalServerError

			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

//PUT: /posts/:id/stars
// putPostStar returns a handler for PUT: /posts/:id/stars ...AddPostStar,UpdatePostStar(),DeletePostStar
func putPostStar(s *Setup) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		statusCode := http.StatusOK
		response.Status = "fail"

		vars := getParametersFromRequestAsMap(r)
		idRaw := vars["postID"]

		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("update attempt of non invalid post id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "postID",
				ErrorMessage: fmt.Sprintf("invalid postID %d", id),
			}
			statusCode = http.StatusBadRequest
		} else {
			id := uint(id)
			st := new(post.Star)
			erro := json.NewDecoder(r.Body).Decode(st)

			if erro != nil {
				response.Data = jSendFailData{
					ErrorReason: "request format",
					ErrorMessage: `bad request, use format
			{"username":"username len 5-22 chars",
			"stars":"number of stars 0-5"}`,
				}
				s.Logger.Printf("bad update star request")
				statusCode = http.StatusBadRequest
			} else {
				username := st.Username
				{ // this block secures the route
					if username != r.Header.Get("authorized_username") {
						s.Logger.Printf("unauthorized post Star request")
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
				}
				if st.NumOfStars == 0 {
					errs := s.PostService.DeletePostStar(id, username)
					switch errs {
					case nil:
						response.Status = "success"
						s.Logger.Printf("success deleting Star of Post %d and username %s", id, username)
						response.Data = *st
					case post.ErrPostNotFound:
						response.Data = jSendFailData{
							ErrorReason:  "postID",
							ErrorMessage: fmt.Sprintf("star of postID %d not found", id),
						}
						statusCode = http.StatusNotFound
					case post.ErrStarNotFound:
						response.Data = jSendFailData{
							ErrorReason:  "star",
							ErrorMessage: fmt.Sprintf("star of username %s and post id %d not found", username, id),
						}
						statusCode = http.StatusNotFound
						s.Logger.Printf("star of username %s and post id %d not found", username, id)
					default:
						s.Logger.Printf("deletion of Star failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when adding Star"
						statusCode = http.StatusInternalServerError
					}
				}
				if st.NumOfStars > 5 || st.NumOfStars < 0 {
					response.Data = jSendFailData{
						ErrorReason: "request format",
						ErrorMessage: `bad request, use format
				{"username":"username len 5-22 chars",
				"num_of_stars":"num_of_stars(0-5)"}`,
					}
					s.Logger.Printf("bad update star request")
					statusCode = http.StatusBadRequest
				}
				if response.Data == nil {
					_, errr := s.PostService.GetPostStar(id, username)
					switch errr {
					case nil:
						newStar, w := s.PostService.UpdatePostStar(id, st)
						switch w {
						case nil:
							response.Status = "success"
							response.Data = *newStar
							s.Logger.Printf("successful in updating Star of Post %d and username %s", id, username)
						case post.ErrPostNotFound:
							response.Data = jSendFailData{
								ErrorReason:  "postID",
								ErrorMessage: fmt.Sprintf("star of postID %d not found", id),
							}
							statusCode = http.StatusNotFound
						case post.ErrStarNotFound:
							response.Data = jSendFailData{
								ErrorReason:  "star",
								ErrorMessage: fmt.Sprintf("star of username %s and post id %d not found", username, id),
							}
							statusCode = http.StatusNotFound
							s.Logger.Printf("star of username %s and post id %d not found", username, id)
						default:
							s.Logger.Printf("updating of post star failed because: %v", w)
							response.Status = "error"
							response.Message = "server error when updating post star"
							statusCode = http.StatusInternalServerError
						}
					case post.ErrPostNotFound:
						response.Data = jSendFailData{
							ErrorReason:  "postID",
							ErrorMessage: fmt.Sprintf("star of postID %d not found", id),
						}
						statusCode = http.StatusNotFound

					case post.ErrStarNotFound:
						newStar, e := s.PostService.AddPostStar(id, st)
						switch e {
						case nil:
							response.Status = "success"
							response.Data = *newStar
							s.Logger.Printf("successful in adding Star of Post %d and username %s", id, username)
						case post.ErrPostNotFound:
							response.Data = jSendFailData{
								ErrorReason:  "postID",
								ErrorMessage: fmt.Sprintf("star of postID %d not found", id),
							}
							statusCode = http.StatusNotFound
						case post.ErrStarNotFound:
							response.Data = jSendFailData{
								ErrorReason:  "star",
								ErrorMessage: fmt.Sprintf("star of username %s and post id %d not found", username, id),
							}
							statusCode = http.StatusNotFound
							s.Logger.Printf("star of username %s and post id %d not found", username, id)
						default:
							s.Logger.Printf("adding of post star failed because: %v", w)
							response.Status = "error"
							response.Message = "server error when adding post star"
							statusCode = http.StatusInternalServerError
						}
					default:
						s.Logger.Printf("fetching of post star failed because: %v", errr)
						response.Status = "error"
						response.Message = "server error when fetching post star"
						statusCode = http.StatusInternalServerError

					}
				}
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}
