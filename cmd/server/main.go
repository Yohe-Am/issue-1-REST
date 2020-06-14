package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"github.com/Yohe-Am/issue-1-REST/pkg/delivery/http/rest"
	"github.com/Yohe-Am/issue-1-REST/pkg/repositories/memory"
	"github.com/Yohe-Am/issue-1-REST/pkg/repositories/postgres"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/auth"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/channel"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/comment"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/feed"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/post"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/release"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/domain/user"
	"github.com/Yohe-Am/issue-1-REST/pkg/services/search"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/microcosm-cc/bluemonday"

	_ "github.com/lib/pq"
)

/*
type postgresHandler struct{
	db *sql.DB
}

func (dbHandler *postgresHandler) Query(query string) {

}

func NewPostgresHandler() postgres.DBHandler {
	db, err := sql.Open(
		"postgres",
		"user='issue #1 dev' " +
		"password='password1234!@#$' " +
		"dbname='issue #1' " +
		"sslmode=disable"
	)
	if err != nil {
		panic(err)
	}
	defer db.Close()
}
*/
/*
type logger struct{}

// Log ...
func (logger *logger) Log(format string, a ...interface{}) {
	if a != nil {
		fmt.Printf(fmt.Sprintf("[%s] %s\n", time.Now().Format(time.StampMilli), format), a)
	} else {
		fmt.Printf("[%s] %s\n", time.Now().Format(time.StampMilli), format)
	}
}
*/

func main() {
	setup := rest.Setup{}
	setup.Logger = log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile)
	var db *sql.DB
	{
		var err error
		const (
			host     = "localhost"
			port     = "5432"
			dbname   = "issue#1_db"
			role     = "issue#1_dev"
			password = "password1234!@#$"
		)
		dataSourceName := fmt.Sprintf(
			`host=%s port=%s dbname='%s' user='%s' password='%s' sslmode=disable`,
			host, port, dbname, role, password)

		db, err = sql.Open("postgres", dataSourceName)
		if err != nil {
			setup.Logger.Fatalf("database connection failed because: %s", err.Error())
		}
		defer db.Close()

		if err = db.Ping(); err != nil {
			setup.Logger.Fatalf("database ping failed because: %s", err.Error())
		}
	}

	services := make(map[string]interface{})
	cacheRepos := make(map[string]interface{})
	dbRepos := make(map[string]interface{})

	{
		{
			var channelDBRepo = postgres.NewChannelRepository(db, &dbRepos)
			dbRepos["Channel"] = &channelDBRepo
			var channelCacheRepo = memory.NewChannelRepository(&channelDBRepo, &cacheRepos)
			cacheRepos["Channel"] = &channelCacheRepo
			setup.ChannelService = channel.NewService(&channelCacheRepo, &services)
			services["Channel"] = &setup.ChannelService
		}
		{
			var usrDBRepo = postgres.NewUserRepository(db, &dbRepos)
			dbRepos["User"] = &usrDBRepo
			var usrCacheRepo = memory.NewUserRepository(&usrDBRepo, &cacheRepos)
			cacheRepos["User"] = &usrCacheRepo
			setup.UserService = user.NewService(&usrCacheRepo, &services)
			services["User"] = &setup.UserService
		}
		{
			var feedDBRepo = postgres.NewFeedRepository(db, &dbRepos)
			dbRepos["Feed"] = &feedDBRepo
			var feedCacheRepo = memory.NewFeedRepository(&feedDBRepo, &cacheRepos)
			cacheRepos["Feed"] = &feedCacheRepo
			setup.FeedService = feed.NewService(&feedCacheRepo, &services)
			services["Feed"] = &setup.FeedService
		}
		{
			var releaseDBRepo = postgres.NewReleaseRepository(db, &dbRepos)
			dbRepos["Release"] = &releaseDBRepo
			var releaseCacheRepo = memory.NewReleaseRepository(&releaseDBRepo)
			cacheRepos["Release"] = &releaseCacheRepo
			setup.ReleaseService = release.NewService(&releaseCacheRepo)
			services["Release"] = &setup.ReleaseService
		}
		{
			var postDBRepo = postgres.NewPostRepository(db, &dbRepos)
			dbRepos["Post"] = &postDBRepo
			var postCacheRepo = memory.NewPostRepository(&postDBRepo)
			cacheRepos["Post"] = &postCacheRepo
			setup.PostService = post.NewService(&postCacheRepo)
			services["Post"] = &setup.PostService
		}
		{
			var commentDBRepo = postgres.NewCommentRepository(db, &dbRepos)
			dbRepos["Comment"] = &commentDBRepo
			var commentCacheRepo = memory.NewCommentRepository(&commentDBRepo)
			cacheRepos["Comment"] = &commentCacheRepo
			setup.CommentService = comment.NewService(&commentCacheRepo)
			services["Comment"] = &setup.CommentService
		}
		{
			var searchDBRepo = postgres.NewSearchRepository(db, &dbRepos)
			dbRepos["Search"] = &searchDBRepo
			setup.SearchService = search.NewService(&searchDBRepo)
			services["Search"] = &setup.SearchService
		}
	}

	setup.ImageServingRoute = "/images/"
	setup.ImageStoragePath = "data/images/"
	setup.HostAddress = "localhost"
	setup.Port = "8080"

	setup.HostAddress += ":" + setup.Port

	setup.StrictSanitizer = bluemonday.StrictPolicy()
	setup.MarkupSanitizer = bluemonday.UGCPolicy()
	setup.MarkupSanitizer.AllowAttrs("class").Matching(regexp.MustCompile("^language-[a-zA-Z0-9]+$")).OnElements("code")

	setup.TokenSigningSecret = []byte("secret")
	setup.TokenAccessLifetime = 15 * time.Minute
	setup.TokenRefreshLifetime = 7 * 24 * time.Hour

	{
		var authDBRepo = postgres.NewAuthRepository(db, &dbRepos)
		dbRepos["Auth"] = &authDBRepo
		var authCacheRepo = memory.NewAuthRepository(&authDBRepo)
		cacheRepos["Auth"] = &authCacheRepo
		setup.AuthService = auth.NewAuthService(&authCacheRepo,
			setup.TokenAccessLifetime,
			setup.TokenRefreshLifetime,
			setup.TokenSigningSecret)
		services["Auth"] = &setup.AuthService
	}

	mux := rest.NewMux(&setup)
	// command line ui
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			switch scanner.Text() {
			case "k":
				log.Fatalln("shutting server down...")
			default:
				fmt.Println("unknown command")
			}
		}
	}()

	setup.HTTPS = false

	if setup.HTTPS {
		setup.HostAddress = "https://" + setup.HostAddress
		setup.Logger.Printf("server running on %s", setup.HostAddress)
		log.Fatal(http.ListenAndServeTLS(":"+setup.Port, "cmd/server/cert.pem", "cmd/server/key.pem", mux))
	} else {
		setup.HostAddress = "http://" + setup.HostAddress
		setup.Logger.Printf("server running on %s", setup.HostAddress)
		log.Fatal(http.ListenAndServe(":"+setup.Port, mux))
	}

}
