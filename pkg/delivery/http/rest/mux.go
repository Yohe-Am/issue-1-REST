package rest

import (
	"log"
	"net/http"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/julienschmidt/httprouter"

	"github.com/slim-crown/issue-1-REST/pkg/services/auth"
	"github.com/slim-crown/issue-1-REST/pkg/services/search"

	"github.com/slim-crown/issue-1-REST/pkg/services/domain/channel"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/comment"

	"github.com/slim-crown/issue-1-REST/pkg/services/domain/feed"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/post"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/release"
	"github.com/slim-crown/issue-1-REST/pkg/services/domain/user"
)

/*
// Logger ...
type Logger interface {
	Log(format string, a ...interface{})
}
*/

// Setup is used to inject dependencies and other required data used by the handlers.
type Setup struct {
	Config
	Dependencies
}

// Dependencies contains dependencies used by the handlers.
type Dependencies struct {
	StrictSanitizer *bluemonday.Policy
	MarkupSanitizer *bluemonday.Policy
	UserService    user.Service
	FeedService    feed.Service
	ChannelService channel.Service
	ReleaseService release.Service
	PostService    post.Service
	CommentService comment.Service
	SearchService  search.Service
	AuthService    auth.Service
	Logger         *log.Logger
}

// Config contains the different settings used to set up the handlers
type Config struct {
	ImageServingRoute, ImageStoragePath, HostAddress, Port string
	TokenAccessLifetime, TokenRefreshLifetime              time.Duration
	TokenSigningSecret                                     []byte
	HTTPS                                                  bool
}

// NewMux returns a new multiplexer with all the used setup.
func NewMux(s *Setup) *httprouter.Router {
	/*
		mainRouter := mux.NewRouter().StrictSlash(true)
		secureRouter := mainRouter.NewRoute().Subrouter()

		// setup static file server
		mainRouter.PathPrefix(s.ImageServingRoute).Handler(
			http.StripPrefix(s.ImageServingRoute, http.FileServer(http.Dir(s.ImageStoragePath))))

		// setup security
		mainRouter.Use(ParseAuthTokenMiddleware(s))
		secureRouter.Use(CheckForAuthMiddleware(s))

		// attach routes
		attachAuthRoutesToRouters(mainRouter, secureRouter, s)
		attachUserRoutesToRouters(mainRouter, secureRouter, s)
		attachReleaseRoutesToRouters(mainRouter, secureRouter, s)
		attachChannelRoutesToRouters(mainRouter, secureRouter, s)
		attachFeedRoutesToRouters(secureRouter, s)
		attachCommentRoutesToRouters(mainRouter, secureRouter, s)
		attachPostRoutesToRouters(mainRouter, secureRouter, s)

		mainRouter.HandleFunc("/search", getSearch(s)).Methods("GET")

		return mainRouter
	*/

	rootRouter := httprouter.New()
	rootRouter.HandleMethodNotAllowed = false

	mainRouter := httprouter.New()
	mainRouter.HandleMethodNotAllowed = false

	secureRouter := httprouter.New()
	secureRouter.HandleMethodNotAllowed = false

	parseAuthMiddleware := ParseAuthTokenMiddleware(s)(mainRouter)

	// CORS hack
	rootRouter.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: replace with rs/cors package

		if r.Method == "OPTIONS"{
			// s.Logger.Printf("OPTIONS request")
			w.Header().Add("Access-Control-Allow-Origin", "*")
			w.Header().Add("Vary", "Origin")	
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set(
				"Access-Control-Allow-Headers",
			 	"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			return;
		}
		parseAuthMiddleware.ServeHTTP(w,r);

	})
	mainRouter.NotFound = CheckForAuthMiddleware(s)(secureRouter)

	fs := http.FileServer(http.Dir(s.ImageStoragePath))
	rootRouter.Handler("GET", s.ImageServingRoute+"*filepath", http.StripPrefix(s.ImageServingRoute, fs))

	// attach routes
	attachAuthRoutesToRouters(mainRouter, secureRouter, s)
	attachUserRoutesToRouters(mainRouter, secureRouter, s)
	attachReleaseRoutesToRouters(mainRouter, secureRouter, s)
	attachFeedRoutesToRouters(secureRouter, s)
	attachCommentRoutesToRouters(mainRouter, secureRouter, s)
	attachChannelRoutesToRouters(mainRouter, secureRouter, s)
	attachPostRoutesToRouters(mainRouter, secureRouter, s)

	mainRouter.HandlerFunc("GET", "/search", getSearch(s))

	return rootRouter
}

func attachAuthRoutesToRouters(mainRouter, secureRouter *httprouter.Router, setup *Setup) {
	mainRouter.HandlerFunc("POST", "/token-auth", postTokenAuth(setup))
	mainRouter.HandlerFunc("GET", "/token-auth-refresh", getTokenAuthRefresh(setup))
	secureRouter.HandlerFunc("GET", "/logout", getLogout(setup))
}

func attachUserRoutesToRouters(mainRouter, secureRouter *httprouter.Router, setup *Setup) {
	mainRouter.HandlerFunc("GET", "/users", getUsers(setup))
	mainRouter.HandlerFunc("POST", "/users", postUser(setup))

	mainRouter.HandlerFunc("GET", "/users/:username", getUser(setup))
	secureRouter.HandlerFunc("PUT", "/users/:username", putUser(setup))
	secureRouter.HandlerFunc("DELETE", "/users/:username", deleteUser(setup))
	secureRouter.HandlerFunc("GET", "/users/:username/bookmarks", getUserBookmarks(setup))
	secureRouter.HandlerFunc("PUT", "/users/:username/bookmarks/:postID", putUserBookmarks(setup))
	secureRouter.HandlerFunc("DELETE", "/users/:username/bookmarks/:postID", deleteUserBookmarks(setup))
	secureRouter.HandlerFunc("POST", "/users/:username/bookmarks", postUserBookmarks(setup))
	secureRouter.HandlerFunc("GET", "/users/:username/picture", getUserPicture(setup))
	secureRouter.HandlerFunc("PUT", "/users/:username/picture", putUserPicture(setup))
	secureRouter.HandlerFunc("DELETE", "/users/:username/picture", deleteUserPicture(setup))
}

func attachReleaseRoutesToRouters(mainRouter, secureRouter *httprouter.Router, setup *Setup) {
	mainRouter.HandlerFunc("GET", "/releases", getReleases(setup))
	mainRouter.HandlerFunc("GET", "/releases/:id", getRelease(setup))
	secureRouter.HandlerFunc("POST", "/releases", postRelease(setup))
	secureRouter.HandlerFunc("PATCH", "/releases/:id", patchRelease(setup))
	secureRouter.HandlerFunc("DELETE", "/releases/:id", deleteRelease(setup))
}

func attachFeedRoutesToRouters(secureRouter *httprouter.Router, setup *Setup) {
	secureRouter.HandlerFunc("GET", "/users/:username/feed", getFeed(setup))
	secureRouter.HandlerFunc("GET", "/users/:username/feed/posts", getFeedPosts(setup))
	secureRouter.HandlerFunc("GET", "/users/:username/feed/channels", getFeedChannels(setup))
	secureRouter.HandlerFunc("POST", "/users/:username/feed/channels", postFeedChannel(setup))
	secureRouter.HandlerFunc("PUT", "/users/:username/feed", putFeed(setup))
	secureRouter.HandlerFunc("DELETE", "/users/:username/feed/channels/:channelname", deleteFeedChannel(setup))
}

func attachCommentRoutesToRouters(mainRouter, secureRouter *httprouter.Router, setup *Setup) {
	mainRouter.HandlerFunc(http.MethodGet, "/posts/:postID/comments/:commentID", getComment(setup))
	mainRouter.HandlerFunc(http.MethodGet, "/posts/:postID/comments", getComments(setup))
	secureRouter.HandlerFunc(http.MethodPost, "/posts/:postID/comments", postComment(setup))
	secureRouter.HandlerFunc(http.MethodPatch, "/posts/:postID/comments/:commentID", patchComment(setup))
	secureRouter.HandlerFunc(http.MethodDelete, "/posts/:postID/comments/:commentID", deleteComment(setup))

	mainRouter.HandlerFunc(http.MethodGet, "/posts/:postID/comments/:commentID/replies/:replyID", getComment(setup))
	mainRouter.HandlerFunc(http.MethodGet, "/posts/:postID/comments/:commentID/replies", getCommentReplies(setup))
	secureRouter.HandlerFunc(http.MethodPost, "/posts/:postID/comments/:commentID/replies", postComment(setup))
	secureRouter.HandlerFunc(http.MethodPatch, "/posts/:postID/comments/:commentID/replies/:replyID", patchComment(setup))
	secureRouter.HandlerFunc(http.MethodGet, "/posts/:postID/comments/:commentID/replies/:replyID", deleteComment(setup))
}
func attachChannelRoutesToRouters(mainRouter, secureRouter *httprouter.Router, setup *Setup) {
	mainRouter.HandlerFunc("POST", "/channels", postChannel(setup))
	mainRouter.HandlerFunc("GET", "/channels", getChannels(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername", getChannel(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername", putChannel(setup))
	secureRouter.HandlerFunc("DELETE", "/channels/:channelUsername", deleteChannel(setup))
	secureRouter.HandlerFunc("GET", "/channels/:channelUsername/admins", getAdmins(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername/admins/:adminUsername", putAdmin(setup))
	secureRouter.HandlerFunc("DELETE", "/channels/:channelUsername/admins/:adminUsername", deleteAdmin(setup))
	secureRouter.HandlerFunc("GET", "/channels/:channelUsername/owners", getOwner(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername/owners/:ownerUsername", putOwner(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername/Posts", getChannelPosts(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername/Posts/:postID", getChannelPost(setup))
	secureRouter.HandlerFunc("GET", "/channels/:channelUsername/catalog", getCatalog(setup))
	secureRouter.HandlerFunc("DELETE", "/channels/:channelUsername/catalogs/:catalogID", deleteReleaseFromCatalog(setup))
	secureRouter.HandlerFunc("GET", "/channels/:channelUsername/official/:catalogID", deleteReleaseFromOfficialCatalog(setup))
	secureRouter.HandlerFunc("GET", "/channels/:channelUsername/catalogs/:catalogID", getReleaseFromCatalog(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername/official/:catalogID", getReleaseFromOfficialCatalog(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername/official", getOfficialCatalog(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername/catalogs/:catalogID", putReleaseInCatalog(setup))
	secureRouter.HandlerFunc("POST", "/channels/:channelUsername/catalogs}", postReleaseInCatalog(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername/official/:releaseID", putReleaseInOfficialCatalog(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername/stickiedPosts", getStickiedPosts(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername/Posts/:postID", stickyPost(setup))
	secureRouter.HandlerFunc("DELETE", "/channels/:channelUsername/stickiedPosts/:stickiedPostID", deleteStickiedPost(setup))
	secureRouter.HandlerFunc("PUT", "/channels/:channelUsername/picture", putChannelPicture(setup))
	mainRouter.HandlerFunc("GET", "/channels/:channelUsername/picture", getChannelPicture(setup))
	secureRouter.HandlerFunc("DELETE", "/channels/:channelUsername/picture", deleteChannelPicture(setup))

}
func attachPostRoutesToRouters(mainRouter, secureRouter *httprouter.Router, setup *Setup) {
	mainRouter.HandlerFunc("GET", "/posts", getPosts(setup))
	secureRouter.HandlerFunc("POST", "/posts", postPost(setup))
	//TODO secure these routes
	mainRouter.HandlerFunc("GET", "/posts/:postID", getPost(setup))
	secureRouter.HandlerFunc("PUT", "/posts/:postID", putPost(setup))
	secureRouter.HandlerFunc("DELETE", "/posts/:postID", deletePost(setup))
	mainRouter.HandlerFunc("GET", "/posts/:postID/releases", getPostReleases(setup))
	//mainRouter.HandlerFunc("GET","/posts/:postID/comments", getPostComments(setup))
	mainRouter.HandlerFunc("GET", "/posts/:postID/stars", getPostStars(setup))
	mainRouter.HandlerFunc("GET", "/posts/:postID/stars/:username", getPostStar(setup))
	secureRouter.HandlerFunc("PUT", "/posts/:postID/stars", putPostStar(setup))
}

// Old gorilla trappings, just comment out

/*

func attachCommentRoutesToRouters(mainRouter, secureRouter *mux.Router, setup *Setup) {
	mainRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{id:[0-9]+}", getComment(setup)).Methods(http.MethodGet)
	mainRouter.HandleFunc("/posts/{postID:[0-9]+}/comments", getComments(setup)).Methods(http.MethodGet)
	secureRouter.HandleFunc("/posts/{postID:[0-9]+}/comments", postComment(setup)).Methods(http.MethodPost)
	secureRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{id:[0-9]+}", patchComment(setup)).Methods(http.MethodPatch)
	secureRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{id:[0-9]+}", deleteComment(setup)).Methods(http.MethodDelete)

	mainRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{rootCommentID:[0-9]+}/replies/{id:[0-9]+}", getComment(setup)).Methods(http.MethodGet)
	mainRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{rootCommentID:[0-9]+}/replies", getCommentReplies(setup)).Methods(http.MethodGet)
	secureRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{rootCommentID:[0-9]+}/replies", postComment(setup)).Methods(http.MethodPost)
	secureRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{rootCommentID:[0-9]+}/replies/{id:[0-9]+}", patchComment(setup)).Methods(http.MethodPatch)
	secureRouter.HandleFunc("/posts/{postID:[0-9]+}/comments/{rootCommentID:[0-9]+}/replies/{id:[0-9]+}", deleteComment(setup)).Methods(http.MethodDelete)
}

func attachAuthRoutesToRouters(mainRouter, secureRouter *mux.Router, setup *Setup) {
	mainRouter.HandleFunc("/token-auth", postTokenAuth(setup)).Methods("POST")
	mainRouter.HandleFunc("/token-auth-refresh", getTokenAuthRefresh(setup)).Methods("GET")
	secureRouter.HandleFunc("/logout", getLogout(setup)).Methods("GET")
}

func attachFeedRoutesToRouters(secureRouter *mux.Router, setup *Setup) {
	//TODO secure these routes
	secureRouter.HandleFunc("/users/{username}/feed", getFeed(setup)).Methods("GET")
	secureRouter.HandleFunc("/users/{username}/feed/posts", getFeedPosts(setup)).Methods("GET")
	secureRouter.HandleFunc("/users/{username}/feed/channels", getFeedChannels(setup)).Methods("GET")
	secureRouter.HandleFunc("/users/{username}/feed/channels", postFeedChannel(setup)).Methods("POST")
	secureRouter.HandleFunc("/users/{username}/feed", putFeed(setup)).Methods("PUT")
	secureRouter.HandleFunc("/users/{username}/feed/channels/{channelname}", deleteFeedChannel(setup)).Methods("DELETE")

}
func attachPostRoutesToRouters(mainRouter, secureRouter *mux.Router, setup *Setup) {
	mainRouter.HandleFunc("/posts", getPosts(setup)).Methods("GET")
	secureRouter.HandleFunc("/posts", postPost(setup)).Methods("POST")
	//TODO secure these routes
	mainRouter.HandleFunc("/posts/{id}", getPost(setup)).Methods("GET")
	secureRouter.HandleFunc("/posts/{id}", putPost(setup)).Methods("PUT")
	secureRouter.HandleFunc("/posts/{id}", deletePost(setup)).Methods("DELETE")
	mainRouter.HandleFunc("/posts/{id}/releases", getPostReleases(setup)).Methods("GET")
	mainRouter.HandleFunc("/posts/{id}/comments", getPostComments(setup)).Methods("GET")
	mainRouter.HandleFunc("/posts/{id}/stars", getPostStars(setup)).Methods("GET")
	mainRouter.HandleFunc("/posts/{id}/stars/{username}", getPostStar(setup)).Methods("GET")
	secureRouter.HandleFunc("/posts/{id}/stars", putPostStar(setup)).Methods("PUT")

}

func attachUserRoutesToRouters(mainRouter, secureRouter *mux.Router, setup *Setup) {
	mainRouter.HandleFunc("/users", getUsers(setup)).Methods("GET")
	mainRouter.HandleFunc("/users", postUser(setup)).Methods("POST")
	//TODO secure these routes
	mainRouter.HandleFunc("/users/{username}", getUser(setup)).Methods("GET")
	secureRouter.HandleFunc("/users/{username}", putUser(setup)).Methods("PUT")
	secureRouter.HandleFunc("/users/{username}", deleteUser(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/users/{username}/bookmarks", getUserBookmarks(setup)).Methods("GET")
	secureRouter.HandleFunc("/users/{username}/bookmarks/{postID}", putUserBookmarks(setup)).Methods("PUT")
	secureRouter.HandleFunc("/users/{username}/bookmarks/{postID}", deleteUserBookmarks(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/users/{username}/bookmarks", postUserBookmarks(setup)).Methods("POST")
	secureRouter.HandleFunc("/users/{username}/picture", getUserPicture(setup)).Methods("GET")
	secureRouter.HandleFunc("/users/{username}/picture", putUserPicture(setup)).Methods("PUT")
	secureRouter.HandleFunc("/users/{username}/picture", deleteUserPicture(setup)).Methods("DELETE")
}

func attachReleaseRoutesToRouters(mainRouter, secureRouter *mux.Router, setup *Setup) {
	mainRouter.HandleFunc("/releases", getReleases(setup)).Methods("GET")
	mainRouter.HandleFunc("/releases", postRelease(setup)).Methods("POST")
	//TODO secure these routes
	secureRouter.HandleFunc("/releases/{id}", getRelease(setup)).Methods("GET")
	secureRouter.HandleFunc("/releases/{id}", patchRelease(setup)).Methods("PUT")
	secureRouter.HandleFunc("/releases/{id}", deleteRelease(setup)).Methods("DELETE")
}
func attachChannelRoutesToRouters(mainRouter, secureRouter *mux.Router, setup *Setup) {
	mainRouter.HandleFunc("/channels", postChannel(setup)).Methods("POST")
	mainRouter.HandleFunc("/channels", getChannels(setup)).Methods("GET")
	mainRouter.HandleFunc("/channels/{channelUsername}", getChannel(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}", putChannel(setup)).Methods("PUT")
	secureRouter.HandleFunc("/channels/{channelUsername}", deleteChannel(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/channels/{channelUsername}/admins", getAdmins(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/admins/{adminUsername}", putAdmin(setup)).Methods("PUT")
	secureRouter.HandleFunc("/channels/{channelUsername}/admins/{adminUsername}", deleteAdmin(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/channels/{channelUsername}/owners", getOwner(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/owners/{ownerUsername}", putOwner(setup)).Methods("PUT")
	mainRouter.HandleFunc("/channels/{channelUsername}/Posts", getChannelPosts(setup)).Methods("GET")
	mainRouter.HandleFunc("/channels/{channelUsername}/Posts/{postID}", getChannelPost(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/catalog", getCatalog(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/catalogs/{catalogID}", deleteReleaseFromCatalog(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/channels/{channelUsername}/official/{catalogID}", deleteReleaseFromOfficialCatalog(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/channels/{channelUsername}/catalogs/{catalogID}", getReleaseFromCatalog(setup)).Methods("GET")
	mainRouter.HandleFunc("/channels/{channelUsername}/official/{catalogID}", getReleaseFromOfficialCatalog(setup)).Methods("GET")
	mainRouter.HandleFunc("/channels/{channelUsername}/official", getOfficialCatalog(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/catalogs/{catalogID}", putReleaseInCatalog(setup)).Methods("PUT")
	secureRouter.HandleFunc("/channels/{channelUsername}/catalogs}", postReleaseInCatalog(setup)).Methods("POST")
	secureRouter.HandleFunc("/channels/{channelUsername}/official/{releaseID}", putReleaseInOfficialCatalog(setup)).Methods("PUT")
	mainRouter.HandleFunc("/channels/{channelUsername}/stickiedPosts", getStickiedPosts(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/Posts/{postID}", stickyPost(setup)).Methods("PUT")
	secureRouter.HandleFunc("/channels/{channelUsername}/stickiedPosts/{stickiedPostID}", deleteStickiedPost(setup)).Methods("DELETE")
	secureRouter.HandleFunc("/channels/{channelUsername}/picture", putChannelPicture(setup)).Methods("PUT")
	mainRouter.HandleFunc("/channels/{channelUsername}/picture", getChannelPicture(setup)).Methods("GET")
	secureRouter.HandleFunc("/channels/{channelUsername}/picture", deleteChannelPicture(setup)).Methods("DELETE")

}

*/
