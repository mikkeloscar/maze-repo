package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	"github.com/drone/drone/shared/envconfig"
	"github.com/drone/drone/shared/server"
	"github.com/gin-gonic/gin"
	"github.com/mikkeloscar/maze/checker"
	"github.com/mikkeloscar/maze/remote"
	"github.com/mikkeloscar/maze/repo"
	"github.com/mikkeloscar/maze/router"
	"github.com/mikkeloscar/maze/router/middleware/context"
	"github.com/mikkeloscar/maze/store/datastore"
)

var (
	envConf = flag.String("config", "env.conf", "")
	debug   = flag.Bool("d", false, "")
)

func main() {
	flag.Parse()

	if !*debug {
		// disbale gin debug mode
		gin.SetMode(gin.ReleaseMode)
	}

	env := envconfig.Load(*envConf)

	err := repo.LoadRepoStorage(env)
	if err != nil {
		log.Fatalf("repo storage error: %s", err)
	}

	log.Printf("using repo storage path: %s", repo.RepoStorage)

	ctxStore, err := datastore.Load(env)
	if err != nil {
		log.Fatalf("failed to load datastore: %s", err)
	}
	ctxRemote := remote.Load(env)

	chck := checker.Checker{
		Remote: ctxRemote,
		Store:  ctxStore,
	}
	go chck.Run()

	// setup the server and start listening
	httpServer := server.Load(env)
	httpServer.Run(
		router.Load(
			context.SetStore(ctxStore),
			context.SetRemote(ctxRemote),
		),
	)
}
