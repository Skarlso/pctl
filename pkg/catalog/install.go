package catalog

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/google/uuid"
	profilesv1 "github.com/weaveworks/profiles/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/weaveworks/pctl/pkg/git"
	"github.com/weaveworks/pctl/pkg/profile"
)

// InstallConfig defines parameters for the installation call.
type InstallConfig struct {
	CatalogClient CatalogClient
	GitClient     git.Git
	ProfileBranch string
	CatalogName   string
	ConfigMap     string
	Namespace     string
	ProfileName   string
	SubName       string
	Version       string
	Directory     string
	URL           string
	Path          string
	GitRepository string
}

// MakeArtifacts returns artifacts for a subscription
type MakeArtifacts func(sub profilesv1.ProfileSubscription, gitClient git.Git, repoRoot, gitRepoNamespace, gitRepoName string) ([]profile.Artifact, error)

var makeArtifacts = profile.MakeArtifacts

// Install using the catalog at catalogURL and a profile matching the provided profileName generates a profile subscription
// and its artifacts
func Install(cfg InstallConfig) error {
	pSpec, err := getProfileSpec(cfg)
	if err != nil {
		return err
	}
	subscription := profilesv1.ProfileSubscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProfileSubscription",
			APIVersion: "weave.works/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.SubName,
			Namespace: cfg.Namespace,
		},
		Spec: pSpec,
	}
	if cfg.ConfigMap != "" {
		subscription.Spec.ValuesFrom = []helmv2.ValuesReference{
			{
				Kind:      "ConfigMap",
				Name:      cfg.SubName + "-values",
				ValuesKey: cfg.ConfigMap,
			},
		}
	}
	var (
		gitRepoNamespace string
		gitRepoName      string
	)
	if cfg.GitRepository != "" {
		split := strings.Split(cfg.GitRepository, "/")
		if len(split) != 2 {
			return fmt.Errorf("git-repository must in format <namespace>/<name>; was: %s", cfg.GitRepository)
		}
		gitRepoNamespace = split[0]
		gitRepoName = split[1]
	}
	profileRootdir := filepath.Join(cfg.Directory, cfg.ProfileName)
	artifacts, err := makeArtifacts(subscription, cfg.GitClient, profileRootdir, gitRepoNamespace, gitRepoName)
	if err != nil {
		return fmt.Errorf("failed to generate artifacts: %w", err)
	}
	artifactsRootDir := filepath.Join(profileRootdir, "artifacts")

	for _, artifact := range artifacts {
		artifactDir := filepath.Join(artifactsRootDir, artifact.Name)
		if err = os.MkdirAll(artifactDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory")
		}
		for _, obj := range artifact.Objects {
			if obj.GetObjectKind().GroupVersionKind().Kind == sourcev1.GitRepositoryKind {
				// clone and copy
				if g, ok := obj.(*sourcev1.GitRepository); ok {
					// TODO: Extract this into a function and defer clean the tmp folder.
					u := uuid.NewString()[:6]
					tmp, err := ioutil.TempDir("", "sparse_clone_git_repo_"+u)
					if err != nil {
						return fmt.Errorf("failed to create temp folder for artifact %s: %w", artifact.Name, err)
					}
					// artifactsRootDir + artifact.Name + g.Spec.Reference.Tag // helmRelease and the Kustomize -- both who consume a GitRepository object.
					if g.Spec.Reference == nil {
						return fmt.Errorf("git ref is empty for gitRepositroy resource")
					}
					ref := g.Spec.Reference.Branch
					if ref == "" {
						ref = g.Spec.Reference.Tag
					}
					split := strings.Split(ref, ":")
					if len(split) != 3 {
						return fmt.Errorf("ref should be of format <profile-name>:<path>:<branch>. was: %s", ref)
					}
					profilePath := split[0]
					path := split[1]
					branch := split[2]
					// I need the subscription path to be in the gitrepository resource...
					if err := cfg.GitClient.SparseClone(g.Spec.URL, branch, tmp, profilePath); err != nil {
						return fmt.Errorf("failed to sparse clone folder with url: %s; branch: %s; path: %s; with error: %w",
							g.Spec.URL,
							branch,
							profilePath,
							err)
					}
					// nginx/chart/...
					if strings.Contains(path, string(os.PathSeparator)) {
						path = filepath.Dir(path)
					}
					fullPath := filepath.Join(tmp, profilePath, path)
					if err := os.Rename(fullPath, filepath.Join(artifactDir, path)); err != nil {
						return fmt.Errorf("failed to move folder: %w", err)
					}
					if err := os.RemoveAll(tmp); err != nil {
						fmt.Println("Failed to remove tmp folder: ", tmp)
					}
					continue
				}
			}
			filename := filepath.Join(artifactDir, fmt.Sprintf("%s.%s", obj.GetObjectKind().GroupVersionKind().Kind, "yaml"))
			if err := generateOutput(filename, obj); err != nil {
				return err
			}
		}
	}

	return generateOutput(filepath.Join(profileRootdir, "profile.yaml"), &subscription)
}

// getProfileSpec generates a spec based on configured properties.
func getProfileSpec(cfg InstallConfig) (profilesv1.ProfileSubscriptionSpec, error) {
	if cfg.URL != "" {
		return profilesv1.ProfileSubscriptionSpec{
			ProfileURL: cfg.URL,
			Branch:     cfg.ProfileBranch,
			Path:       cfg.Path,
		}, nil
	}
	p, err := Show(cfg.CatalogClient, cfg.CatalogName, cfg.ProfileName, cfg.Version)
	if err != nil {
		return profilesv1.ProfileSubscriptionSpec{}, fmt.Errorf("failed to get profile %q in catalog %q: %w", cfg.ProfileName, cfg.CatalogName, err)
	}

	return profilesv1.ProfileSubscriptionSpec{
		ProfileURL: p.URL,
		Version:    filepath.Join(p.Name, p.Version),
		ProfileCatalogDescription: &profilesv1.ProfileCatalogDescription{
			Catalog: cfg.CatalogName,
			Version: p.Version,
			Profile: p.Name,
		},
	}, nil
}

func generateOutput(filename string, o runtime.Object) error {
	e := kjson.NewSerializerWithOptions(kjson.DefaultMetaFactory, nil, nil, kjson.SerializerOptions{Yaml: true, Strict: true})
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		if err := f.Close(); err != nil {
			fmt.Printf("Failed to properly close file %s\n", f.Name())
		}
	}(f)
	if err := e.Encode(o, f); err != nil {
		return err
	}
	return nil

}

// CreatePullRequest creates a pull request from the current changes.
func CreatePullRequest(scm git.SCMClient, g git.Git) error {
	if err := g.IsRepository(); err != nil {
		return fmt.Errorf("directory is not a git repository: %w", err)
	}

	if err := g.CreateBranch(); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	if err := g.Add(); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	if err := g.Commit(); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	if err := g.Push(); err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	if err := scm.CreatePullRequest(); err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	return nil
}
