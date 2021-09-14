package main

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/urfave/cli/v2"

	"github.com/weaveworks/pctl/api"
	"github.com/weaveworks/pctl/pkg/catalog"
	"github.com/weaveworks/pctl/pkg/client"
	"github.com/weaveworks/pctl/pkg/git"
	"github.com/weaveworks/pctl/pkg/log"
	"github.com/weaveworks/pctl/pkg/runner"
)

var createPRFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:  "create-pr",
		Value: false,
		Usage: "If given, upgrade will create a PR for the modifications it outputs.",
	},
	&cli.StringFlag{
		Name:        "pr-message",
		Value:       "Push changes to remote",
		DefaultText: "Push changes to remote",
		Usage:       "The message to use for committing.",
		Aliases:     []string{"m"},
	},
	&cli.StringFlag{
		Name:        "pr-remote",
		Value:       "origin",
		DefaultText: "origin",
		Usage:       "The remote to push the branch to.",
	},
	&cli.StringFlag{
		Name:        "pr-base",
		Value:       "main",
		DefaultText: "main",
		Usage:       "The base branch to open a PR against.",
	},
	&cli.StringFlag{
		Name:  "pr-branch",
		Usage: "The branch to create the PR from. Generated if not set.",
	},
	&cli.StringFlag{
		Name:  "pr-repo",
		Value: "",
		Usage: "The repository to open a pr against. Format is: org/repo-name.",
	},
}

func addCmd() *cli.Command {
	return &cli.Command{
		Name:    "add",
		Aliases: []string{"apply"},
		Usage:   "generate a profile installation",
		UsageText: "To add from a profile catalog entry: pctl --catalog-url <URL> add --name pctl-profile --namespace default --profile-branch main --config-map configmap-name <CATALOG>/<PROFILE>[/<VERSION>]\n   " +
			"To add directly from a profile repository: pctl add --name pctl-profile --namespace default --profile-branch development --profile-repo-url https://github.com/weaveworks/profiles-examples --profile-path bitnami-nginx",
		Flags: append(createPRFlags,
			&cli.StringFlag{
				Name:     "name",
				Usage:    "The name of the installation.",
				Required: true,
			},
			&cli.StringFlag{
				Name:        "namespace",
				DefaultText: "default",
				Value:       "default",
				Usage:       "The namespace to use for generating resources.",
			},
			&cli.StringFlag{
				Name:        "profile-branch",
				Value:       "main",
				DefaultText: "main",
				Usage:       "The branch to use on the repository in which the profile is.",
			},
			&cli.StringFlag{
				Name:  "config-map",
				Value: "",
				Usage: "The name of the ConfigMap which contains values for this profile.",
			},
			&cli.StringFlag{
				Name:        "out",
				DefaultText: "current",
				Value:       ".",
				Usage:       "Optional location to create the profile installation folder in. This should be relative to the current working directory.",
			},
			&cli.StringFlag{
				Name:  "profile-repo-url",
				Value: "",
				Usage: "Optional value defining the URL of the repository that contains the profile to be added.",
			},
			&cli.StringFlag{
				Name:        "profile-path",
				Value:       ".",
				DefaultText: "<root>",
				Usage:       "Value defining the path to a profile when url is provided.",
			},
			&cli.StringFlag{
				Name:  "git-repository",
				Value: "",
				Usage: "The namespace and name of the GitRepository object governing the flux repo.",
			}),
		Action: func(c *cli.Context) error {
			// Run installation main
			url := c.String("profile-repo-url")
			if url != "" && c.Args().Len() > 0 {
				return errors.New("it looks like you provided a url with a catalog entry; please choose either format: url/branch/path or <CATALOG>/<PROFILE>[/<VERSION>]")
			}

			var (
				profilePath   string
				catalogClient *client.Client
				err           error
			)
			if url == "" {
				profilePath, catalogClient, err = parseArgs(c)
				if err != nil {
					_ = cli.ShowCommandHelp(c, "add")
					return err
				}
			}

			branch := c.String("profile-branch")
			subName := c.String("name")
			namespace := c.String("namespace")
			configMap := c.String("config-map")
			dir := c.String("out")
			path := c.String("profile-path")
			message := c.String("pr-message")
			gitRepository := c.String("git-repository")
			r := &runner.CLIRunner{}
			g := git.NewCLIGit(git.CLIGitConfig{
				Message: message,
			}, r)
			installationDirectory, err := api.AddProfile(api.AddProfileOpts{
				URL:           url,
				Branch:        branch,
				SubName:       subName,
				Namespace:     namespace,
				ConfigMap:     configMap,
				Dir:           dir,
				Path:          path,
				GitRepository: gitRepository,
				ProfilePath:   profilePath,
				CatalogClient: catalogClient,
				GitClient:     g,
			})
			if err != nil {
				return err
			}
			log.Successf("installation completed successfully")
			// Create a pull request if desired
			if c.Bool("create-pr") {
				if err := createPullRequest(c, installationDirectory); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// createPullRequest runs the pull request creation part of the `add` command.
func createPullRequest(c *cli.Context, installationDirectory string) error {
	branch := c.String("pr-branch")
	repo := c.String("pr-repo")
	base := c.String("pr-base")
	remote := c.String("pr-remote")
	directory := c.String("out")
	message := c.String("pr-message")
	if repo == "" {
		return errors.New("repo must be defined if create-pr is true")
	}
	if branch == "" {
		branch = c.String("name") + "-" + uuid.NewString()[:6]
	}
	log.Actionf("creating a PR to repo %s with base %s and branch %s", repo, base, branch)
	r := &runner.CLIRunner{}
	g := git.NewCLIGit(git.CLIGitConfig{
		Directory: directory,
		Branch:    branch,
		Remote:    remote,
		Base:      base,
		Message:   message,
	}, r)
	scmClient, err := git.NewClient(git.SCMConfig{
		Branch: branch,
		Base:   base,
		Repo:   repo,
	})
	if err != nil {
		return fmt.Errorf("failed to create scm client: %w", err)
	}
	return catalog.CreatePullRequest(scmClient, g, branch, installationDirectory)
}
