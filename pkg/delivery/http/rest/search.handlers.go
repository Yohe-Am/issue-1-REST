package rest

import (
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/channel"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/post"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/release"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/user"
	"github.com/slim-crown/issue-1-REST/pkg/services/search"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// getSearch returns a handler for GET /search?pattern=Joe requests
func getSearch(s *Setup) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var response jSendResponse
		response.Status = "fail"
		statusCode := http.StatusOK

		pattern := ""
		limit := 25
		offset := 0
		sortBy := search.SortByCreationTime
		sortOrder := search.SortDescending

		{ // this block reads the query strings if any
			pattern = r.URL.Query().Get("pattern")

			if pattern == "" {
				s.Logger.Printf("bad search request, pattern")
				response.Data = jSendFailData{
					ErrorReason:  "pattern",
					ErrorMessage: "bad request, pattern can't be empty",
				}
				statusCode = http.StatusBadRequest
			}

			if limitPageRaw := r.URL.Query().Get("limit"); limitPageRaw != "" {
				limit, err = strconv.Atoi(limitPageRaw)
				if err != nil || limit < 0 {
					s.Logger.Printf("bad search request, limit")
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
					s.Logger.Printf("bad search request, offset")
					response.Data = jSendFailData{
						ErrorReason:  "offset",
						ErrorMessage: "bad request, offset can't be negative",
					}
					statusCode = http.StatusBadRequest
				}
			}

			sort := r.URL.Query().Get("sort")
			sortSplit := strings.Split(sort, "_")

			switch sortByQuery := sortSplit[0]; sortByQuery {
			case "rank":
				sortBy = search.SortByRank
			default:
				sortBy = search.SortByCreationTime
				sortOrder = search.SortDescending
			}
			if len(sortSplit) > 1 {
				switch sortOrderQuery := sortSplit[1]; sortOrderQuery {
				case "dsc":
					sortOrder = search.SortDescending
				default:
					sortOrder = search.SortAscending
				}
			}

		}

		// if queries are clean
		if response.Data == nil {
			var responseData struct {
				Posts    interface{} `json:"posts"`
				Releases interface{} `json:"releases"`
				Comments interface{} `json:"comments"`
				Channels interface{} `json:"channels"`
				Users    interface{} `json:"users"`
			}
			order := string(sortOrder)
			successCounter := 0
			{
				posts, err := s.PostService.SearchPost(pattern, "", post.SortOrder(order), limit, offset)
				if err != nil {
					s.Logger.Printf("searching of posts failed because: %v", err)
					responseData.Posts = jSendResponse{
						Status:  "error",
						Data:    nil,
						Message: "server error when searching posts",
					}
				} else {
					responseData.Posts = posts
					s.Logger.Printf("success search fetching posts")
					successCounter++
				}
			}
			{
				users, err := s.UserService.SearchUser(pattern, user.SortByUsername, user.SortOrder(order), limit, offset)
				if err != nil {
					s.Logger.Printf("searching of users failed because: %v", err)
					responseData.Users = jSendResponse{
						Status:  "error",
						Data:    nil,
						Message: "server error when searching users",
					}
				} else {
					for _, u := range users {
						u.Email = ""
						u.BookmarkedPosts = nil
						if u.PictureURL != "" {
							u.PictureURL = s.HostAddress + s.ImageServingRoute + url.PathEscape(u.PictureURL)
						}
					}
					responseData.Users = users
					s.Logger.Printf("success searching users")
					successCounter++
				}
			}
			{
				releases, err := s.ReleaseService.SearchRelease(pattern, "", release.SortOrder(order), limit, offset)
				if err != nil {
					s.Logger.Printf("searching of releases failed because: %v", err)
					responseData.Releases = jSendResponse{
						Status:  "error",
						Data:    nil,
						Message: "server error when searching releases",
					}
				} else {
					for _, rel := range releases {
						if rel.Type == release.Image {
							rel.Content = s.HostAddress + s.ImageServingRoute + url.PathEscape(rel.Content)
						}
					}
					responseData.Releases = releases
					s.Logger.Printf("success searching releases")
					successCounter++
				}
			}
			{
				channels, err := s.ChannelService.SearchChannels(pattern, "", channel.SortOrder(order), limit, offset)
				if err != nil {
					s.Logger.Printf("searching of channels failed because: %v", err)
					responseData.Channels = jSendResponse{
						Status:  "error",
						Data:    nil,
						Message: "server error when channels channels",
					}
				} else {
					for _, c := range channels {
						c.AdminUsernames = nil
						c.ReleaseIDs = nil
						c.OwnerUsername = ""
						if c.PictureURL != "" {
							c.PictureURL = s.HostAddress + s.ImageServingRoute + url.PathEscape(c.PictureURL)
						}
					}
					responseData.Channels = channels
					s.Logger.Printf("success searching channels")
					successCounter++
				}
			}
			{
				comments, err := s.SearchService.SearchComments(pattern, sortBy, sortOrder, limit, offset)
				if err != nil {
					s.Logger.Printf("searching of comments failed because: %v", err)
					responseData.Comments = jSendResponse{
						Status:  "error",
						Data:    nil,
						Message: "server error when comments users",
					}
				} else {
					responseData.Comments = comments
					s.Logger.Printf("success searching comments")
					successCounter++
				}
			}
			if successCounter == 5 {
				response.Status = "success"
			}
			response.Data = responseData
		}
		writeResponseToWriter(response, w, statusCode)
	}
}
