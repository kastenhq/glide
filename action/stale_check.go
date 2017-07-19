package action

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/glide/cache"
	"github.com/Masterminds/glide/cfg"
	"github.com/Masterminds/glide/msg"
	gpath "github.com/Masterminds/glide/path"
	"github.com/Masterminds/vcs"
)

func StaleCheck(base string) {
	cache.SystemLock()
	_, err := gpath.Glide()
	glidefile := gpath.GlideFile
	if err != nil {
		msg.Info("Unable to find a glide.yaml file. Would you like to create one now? Yes (Y) or No (N)")
		bres := msg.PromptUntilYorN()
		if bres {
			// Guess deps
			conf := guessDeps(base, false)
			// Write YAML
			if err := conf.WriteFile(glidefile); err != nil {
				msg.Die("Could not save %s: %s", glidefile, err)
			}
		} else {
			msg.Err("Unable to find configuration file. Please create configuration information to continue.")
		}
	}

	conf := EnsureConfig()

	cache.Setup()

	msg.Info("Scanning for dependencies")
	var deps []*cfg.Dependency
	for _, dep := range conf.Imports {
		if dep.Reference == "" {
			msg.Warn("--> %s: version unspecified", dep.Name)
			continue
		}
		deps = append(deps, dep)
	}
	for _, dep := range conf.DevImports {
		if dep.Reference == "" {
			msg.Warn("--> %s: version unspecified", dep.Name)
			continue
		}
		deps = append(deps, dep)
	}

	for _, dep := range deps {
		wizardFindVersions(dep)
		msg.Info("%s is at %s", dep.Name, dep.Reference)
		if err := func(d *cfg.Dependency) error {
			remote := d.Remote()

			// If the repo uses semver tags, check if the latests equals the
			// local version.
			memlatest := cache.MemLatest(remote)
			if memlatest != "" {
				if dep.Reference != memlatest {
					msg.Warn("New version for %s: %s", dep.Name, memlatest)
				}
				return nil
			}

			// For packages without semver, we check the most recent hash.
			l := cache.Location()
			key, err := cache.Key(remote)
			if err != nil {
				return err
			}

			local := filepath.Join(l, "src", key)
			repo, err := vcs.NewRepo(remote, local)
			if err != nil {
				return err
			}
			if repo.Vcs() != vcs.Git {
				return fmt.Errorf("We require unversioned repos to use git")
			}
			ci, err := repo.CommitInfo("master")
			if err != nil {
				return err
			}
			if dep.Reference != ci.Commit {
				msg.Warn("--> Latest version is %s", ci.Commit)
			}

			return nil
		}(dep); err != nil {
			msg.Err("Could not get version info for %s: %s", dep.Name, err.Error())
		}
	}
}
