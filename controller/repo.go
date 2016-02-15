package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/drone/drone/shared/crypto"
	"github.com/gin-gonic/gin"
	"github.com/mikkeloscar/maze/remote"
	"github.com/mikkeloscar/maze/repo"
	"github.com/mikkeloscar/maze/router/middleware/session"
	"github.com/mikkeloscar/maze/store"
)

func splitRepoName(source string) (string, string, error) {
	split := strings.Split(source, "/")
	if len(split) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %s", source)
	}

	return split[0], split[1], nil
}

func PostRepo(c *gin.Context) {
	remote := remote.FromContext(c)
	user := session.User(c)
	owner := c.Param("owner")
	name := c.Param("name")

	if owner != user.Login {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	in := struct {
		SourceRepo string `json:"source_repo" binding:"required"`
	}{}
	err := c.BindJSON(&in)
	if err != nil {
		log.Errorf("failed to parse request body: %s", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	sourceOwner, sourceName, err := splitRepoName(in.SourceRepo)
	if err != nil {
		log.Error(err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// error if the repository already exists
	_r, err := store.GetRepoByOwnerName(c, owner, name)
	if _r != nil {
		log.Errorf("unable to add repo: %s/%s, already exists", owner, name)
		c.AbortWithStatus(http.StatusConflict)
		return
	}

	// Fetch source repo
	r, err := remote.Repo(user, sourceOwner, sourceName)
	if err != nil {
		log.Errorf("unable to get repo: %s/%s: %s", sourceOwner, sourceName, err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	p, err := remote.Perm(user, sourceOwner, sourceName)
	if err != nil {
		log.Errorf("unable to get repo permission for: %s/%s: %s", sourceOwner, sourceName, err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if !(p.Admin || (p.Pull && p.Push)) {
		log.Errorf("pull/push access required")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// TODO: make branches configurable
	err = remote.SetupBranch(user, sourceOwner, sourceName, "master", "build")
	if err != nil {
		log.Errorf("failed to setup build branch: %s", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	r.UserID = user.ID
	r.Owner = owner
	r.Name = name
	r.LastCheck = time.Now().UTC().Add(-1 * time.Hour)
	r.Hash = crypto.Rand()

	fsRepo := repo.NewRepo(r)

	err = fsRepo.InitDir()
	if err != nil {
		log.Errorf("failed to create repo storage path '%s' on disk: %s", fsRepo.Path(), err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	err = store.CreateRepo(c, r)
	if err != nil {
		log.Errorf("failed to add repo: %s", err)
		err = fsRepo.ClearPath()
		if err != nil {
			log.Errorf("failed to cleanup repo path '%s': %s", fsRepo.Path(), err)
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, r)
}

func GetRepo(c *gin.Context) {
	c.JSON(http.StatusOK, session.Repo(c))
}
