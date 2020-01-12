package rest

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/comment"
	"gopkg.in/russross/blackfriday.v2"
	"net/http"
	"strconv"
)

func sanitizeComment(c *comment.Comment, s *Setup) {
	c.Content = string(s.MarkupSanitizer.SanitizeBytes(
		blackfriday.Run(
			[]byte(c.Content),
			blackfriday.WithExtensions(blackfriday.CommonExtensions),
		),
	))
	if c.Content == "<p></p>\n" {
		c.Content = ""
	}

}

// postComment returns a handler for POST /posts/{postID}/comments requests
// it also handles /posts/{postID}/comments/{rootCommentID}/replies requests
func postComment(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK
		c := new(comment.Comment)

		vars := mux.Vars(r)

		postIDRaw := vars["postID"]
		c.OriginPost, err = strconv.Atoi(postIDRaw)
		if err != nil {
			s.Logger.Printf("post comment attempt on non invalid post id %s", postIDRaw)
			response.Data = jSendFailData{
				ErrorReason:  "releaseID",
				ErrorMessage: fmt.Sprintf("invalid post id %s", postIDRaw),
			}
			statusCode = http.StatusBadRequest
		}
		rootCommentIDRaw := vars["rootCommentID"]
		c.ReplyTo, err = strconv.Atoi(rootCommentIDRaw)
		if err != nil {
			c.ReplyTo = -1
		}
		if response.Data == nil {
			{ // checks if requests uses forms or JSON and parses then
				c.Commenter = r.FormValue("commenter")
				if c.Commenter != "" {
					c.Content = r.FormValue("content")
					c.ReplyTo, err = strconv.Atoi(r.FormValue("replyTo"))
					if err != nil {
						c.ReplyTo = -1
					}
				} else {
					err = json.NewDecoder(r.Body).Decode(c)
					if err != nil {
						// TODO format
						response.Data = jSendFailData{
							ErrorReason:  "request format",
							ErrorMessage: `bad request, use format`,
						}
						s.Logger.Printf("bad post comment request")
						statusCode = http.StatusBadRequest
					}
				}
			}
			if response.Data == nil {
				{ // this block secures the route
					if c.Commenter != r.Header.Get("authorized_username") {
						s.Logger.Printf("unauthorized post Comment request")
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
				}
				sanitizeComment(c, s)

				// this block checks for required fields
				if c.Commenter == "" {
					response.Data = jSendFailData{
						ErrorReason:  "commenter",
						ErrorMessage: "commenter is required",
					}
				}
				if c.Content == "" {
					response.Data = jSendFailData{
						ErrorReason:  "content",
						ErrorMessage: "content is required",
					}
				}
				if response.Data == nil {
					s.Logger.Printf("trying to add comment %v", c)
					c, err = s.CommentService.AddComment(c)
					switch err {
					case nil:
						response.Status = "success"
						response.Data = *c
						s.Logger.Printf("success adding comment %v", c)
					case comment.ErrPostNotFound:
						s.Logger.Printf("adding of comment failed because: %v", err)
						response.Data = jSendFailData{
							ErrorReason:  "postID",
							ErrorMessage: "post not found",
						}
						statusCode = http.StatusNotFound
					case comment.ErrUserNotFound:
						s.Logger.Printf("adding of comment failed because: %v", err)
						response.Data = jSendFailData{
							ErrorReason:  "username",
							ErrorMessage: "user not found",
						}
						statusCode = http.StatusNotFound
					default:
						//_ = s.UserService.DeleteUser(c.Username)
						s.Logger.Printf("adding of comment failed because: %v", err)
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
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getComment returns a handler for GET /posts/{postID}/comments/{id} requests
// it also handles /posts/{postID}/comments/{rootCommentID}/replies/{id} requests
func getComment(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK
		vars := mux.Vars(r)

		idRaw := vars["id"]
		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("fetch attempt of non invalid comment id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "commentID",
				ErrorMessage: fmt.Sprintf("invalid commentID %s", idRaw),
			}
			statusCode = http.StatusBadRequest
		}
		if response.Data == nil {
			c, err := s.CommentService.GetComment(id)
			switch err {
			case nil:
				response.Status = "success"
				response.Data = *c
				s.Logger.Printf("success fetching comment %d", id)
			case comment.ErrCommentNotFound:
				s.Logger.Printf("fetch attempt of non existing comment %d", id)
				response.Data = jSendFailData{
					ErrorReason:  "commentID",
					ErrorMessage: fmt.Sprintf("comment of commentID %d not found", id),
				}
				statusCode = http.StatusNotFound
			default:
				s.Logger.Printf("fetching of comment failed because: %v", err)
				response.Status = "error"
				response.Message = "server error when fetching comment"
				statusCode = http.StatusInternalServerError
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getComments returns a handler for GET /posts/{postID}/comments?sort=new&limit=5&offset=0&pattern=Joe requests
func getComments(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK
		vars := mux.Vars(r)

		postIDRaw := vars["postID"]
		postID, err := strconv.Atoi(postIDRaw)
		if err != nil {
			s.Logger.Printf("post comment attempt on non invalid post id %s", postIDRaw)
			response.Data = jSendFailData{
				ErrorReason:  "releaseID",
				ErrorMessage: fmt.Sprintf("invalid post id %s", postIDRaw),
			}
			statusCode = http.StatusBadRequest
		}
		if response.Data == nil {
			limit := 25
			offset := 0
			{ // this block reads the query strings if any
				if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
					limit, err = strconv.Atoi(limitPageRaw)
					if err != nil || limit < 0 {
						s.Logger.Printf("bad get feed request, limit")
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
			}

			if response.Data == nil {
				c, err := s.CommentService.GetComments(postID, comment.SortByCreationTime, comment.SortDescending, limit, offset)
				switch err {
				case nil:
					response.Status = "success"
					response.Data = c
					s.Logger.Printf("success fetching comments for post %d", postID)
				case comment.ErrPostNotFound:
					s.Logger.Printf("fetching of comment failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "postID",
						ErrorMessage: "post not found",
					}
					statusCode = http.StatusNotFound
				default:
					s.Logger.Printf("fetching of comment failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when fetching comment"
					statusCode = http.StatusInternalServerError
				}
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// getCommentReplies returns a handler for GET /posts/{postID}/comments/{rootCommentID}/replies?sort=new&limit=5&offset=0&pattern=Joe requests
func getCommentReplies(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK
		vars := mux.Vars(r)

		rootCommentIDRaw := vars["rootCommentID"]
		commentID, err := strconv.Atoi(rootCommentIDRaw)
		if err != nil {
			s.Logger.Printf("post comment attempt on non invalid comment id %s", rootCommentIDRaw)
			response.Data = jSendFailData{
				ErrorReason:  "commentID",
				ErrorMessage: fmt.Sprintf("invalid commentID %s", rootCommentIDRaw),
			}
			statusCode = http.StatusBadRequest
		}
		if response.Data == nil {
			limit := 25
			offset := 0
			{ // this block reads the query strings if any
				if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
					limit, err = strconv.Atoi(limitPageRaw)
					if err != nil || limit < 0 {
						s.Logger.Printf("bad get feed request, limit")
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
			}
			if response.Data == nil {
				c, err := s.CommentService.GetReplies(commentID, comment.SortByCreationTime, comment.SortDescending, limit, offset)
				switch err {
				case nil:
					response.Status = "success"
					response.Data = c
					s.Logger.Printf("success fetching replies for post %d", commentID)
				case comment.ErrCommentNotFound:
					s.Logger.Printf("fetching of replies failed because: %v", err)
					response.Data = jSendFailData{
						ErrorReason:  "commentID",
						ErrorMessage: "comment not found",
					}
					statusCode = http.StatusNotFound
				default:
					s.Logger.Printf("fetching of replies failed because: %v", err)
					response.Status = "error"
					response.Message = "server error when fetching comment"
					statusCode = http.StatusInternalServerError
				}
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// patchComment returns a handler for PATCH /posts/{postID}/comments/{id} requests
// it also handles /posts/{postID}/comments/{rootCommentID}/replies/{id} requests
func patchComment(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK
		c := new(comment.Comment)

		vars := mux.Vars(r)

		idRaw := vars["id"]
		id, err := strconv.Atoi(idRaw)
		if err != nil {
			s.Logger.Printf("fetch attempt of non invalid comment id %s", idRaw)
			response.Data = jSendFailData{
				ErrorReason:  "commentID",
				ErrorMessage: fmt.Sprintf("invalid commentID %s", idRaw),
			}
			statusCode = http.StatusBadRequest
		}
		c.ID = id
		{ // this block secures the route
			if temp, err := s.CommentService.GetComment(id); err == nil {
				if temp.Commenter != r.Header.Get("authorized_username") {
					s.Logger.Printf("unauthorized patch Comment request")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			} else {
				s.Logger.Printf("invalid delete comment request")
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
		if response.Data == nil {
			{ // checks if requests uses forms or JSON and parses then
				c.Commenter = r.FormValue("content")
				if c.Commenter != "" {

				} else {
					err = json.NewDecoder(r.Body).Decode(c)
					if err != nil {
						// TODO format
						response.Data = jSendFailData{
							ErrorReason:  "request format",
							ErrorMessage: `bad request, use format`,
						}
						s.Logger.Printf("bad post comment request")
						statusCode = http.StatusBadRequest
					}
				}
			}
			if response.Data == nil {

				sanitizeComment(c, s)

				// this block checks for required fields
				if c.Content == "" {
					// no update able data
					statusCode = http.StatusNoContent
					c, err = s.CommentService.GetComment(id)
					switch err {
					case nil:
						s.Logger.Printf("success patch comment at id %d", id)
						response.Status = "success"
						response.Data = *c
					default:
						s.Logger.Printf("patching of comment failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when adding user"
						statusCode = http.StatusInternalServerError
					}
				}
				if response.Data == nil {
					c, err = s.CommentService.UpdateComment(c)
					switch err {
					case nil:
						response.Status = "success"
						response.Data = *c
						s.Logger.Printf("success patching comment %v", c)
					case comment.ErrCommentNotFound:
						s.Logger.Printf("patch attempt of non existing comment %d", id)
						response.Data = jSendFailData{
							ErrorReason:  "commentID",
							ErrorMessage: fmt.Sprintf("comment of commentID %d not found", id),
						}
						statusCode = http.StatusNotFound
					default:
						//_ = s.UserService.DeleteUser(c.Username)
						s.Logger.Printf("adding of comment failed because: %v", err)
						response.Status = "error"
						response.Message = "server error when adding user"
						statusCode = http.StatusInternalServerError
					}
				}
			}
		}
		writeResponseToWriter(response, w, statusCode)
	}
}

// deleteComment returns a handler for DELETE /posts/{postID}/comments/{id} requests
// it also handles /posts/{postID}/comments/{rootCommentID}/replies/{id} requests
func deleteComment(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		vars := mux.Vars(r)

		idRaw := vars["id"]
		id, _ := strconv.Atoi(idRaw)

		{ // this block secures the route
			if temp, err := s.CommentService.GetComment(id); err == nil {
				if temp.Commenter != r.Header.Get("authorized_username") {
					s.Logger.Printf("unauthorized patch Comment request")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			} else {
				s.Logger.Printf("invalid delete comment request")
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
		err := s.CommentService.DeleteComment(id)
		if err != nil {
			s.Logger.Printf("deletion of comment failed because: %v", err)
			response.Status = "error"
			response.Message = "server error when deleting user"
			statusCode = http.StatusInternalServerError
		} else {
			response.Status = "success"
			s.Logger.Printf("success deleting comment %d", id)
		}
		writeResponseToWriter(response, w, statusCode)
	}
}
