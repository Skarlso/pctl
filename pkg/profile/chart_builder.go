package profile

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	profilesv1 "github.com/weaveworks/profiles/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ChartBuilder struct {
	GitRepositoryNamespace string
	GitRepositoryName      string
	RootDir                string
}

func (c *ChartBuilder) Build(artifact profilesv1.Artifact, installation profilesv1.ProfileInstallation, definition profilesv1.ProfileDefinition) ([]Artifact, error) {
	a := Artifact{Name: artifact.Name}
	helmRelease := c.makeHelmRelease(artifact, installation, definition)
	a.Objects = append(a.Objects, helmRelease)
	if artifact.Chart.Path != "" {
		if c.GitRepositoryNamespace == "" && c.GitRepositoryName == "" {
			return nil, fmt.Errorf("in case of local resources, the flux gitrepository object's details must be provided")
		}
		helmRelease.Spec.Chart.Spec.Chart = filepath.Join(c.RootDir, "artifacts", artifact.Name, artifact.Chart.Path)
		branch := installation.Spec.Source.Branch
		if installation.Spec.Source.Tag != "" {
			branch = installation.Spec.Source.Tag
		}
		a.RepoURL = installation.Spec.Source.URL
		a.SparseFolder = definition.Name
		a.Branch = branch
		a.PathsToCopy = append(a.PathsToCopy, artifact.Chart.Path)
	}
	if artifact.Chart.URL != "" {
		helmRepository := c.makeHelmRepository(artifact.Chart.URL, artifact.Chart.Name, installation)
		a.Objects = append(a.Objects, helmRepository)
	}
	return []Artifact{a}, nil
}

func (c *ChartBuilder) makeHelmRelease(artifact profilesv1.Artifact, installation profilesv1.ProfileInstallation, definition profilesv1.ProfileDefinition) *helmv2.HelmRelease {
	var helmChartSpec helmv2.HelmChartTemplateSpec
	if artifact.Chart.Path != "" {
		helmChartSpec = c.makeGitChartSpec(path.Join(installation.Spec.Source.Path, artifact.Chart.Path))
	} else if artifact.Chart != nil {
		helmChartSpec = c.makeHelmChartSpec(artifact.Chart.Name, artifact.Chart.Version, installation)
	}
	helmRelease := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      makeArtifactName(artifact.Name, installation, definition),
			Namespace: installation.ObjectMeta.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       helmv2.HelmReleaseKind,
			APIVersion: helmv2.GroupVersion.String(),
		},
		Spec: helmv2.HelmReleaseSpec{
			Chart: helmv2.HelmChartTemplate{
				Spec: helmChartSpec,
			},
			Values:     installation.Spec.Values,
			ValuesFrom: installation.Spec.ValuesFrom,
		},
	}
	return helmRelease
}

func (c *ChartBuilder) makeHelmRepository(url string, name string, installation profilesv1.ProfileInstallation) *sourcev1.HelmRepository {
	return &sourcev1.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.makeHelmRepoName(name, installation),
			Namespace: installation.ObjectMeta.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       sourcev1.HelmRepositoryKind,
			APIVersion: sourcev1.GroupVersion.String(),
		},
		Spec: sourcev1.HelmRepositorySpec{
			URL: url,
		},
	}
}

func (c *ChartBuilder) makeHelmRepoName(name string, installation profilesv1.ProfileInstallation) string {
	repoParts := strings.Split(installation.Spec.Source.URL, "/")
	repoName := repoParts[len(repoParts)-1]
	return join(installation.Name, repoName, name)
}

func (c *ChartBuilder) makeGitChartSpec(path string) helmv2.HelmChartTemplateSpec {
	return helmv2.HelmChartTemplateSpec{
		Chart: path,
		SourceRef: helmv2.CrossNamespaceObjectReference{
			Kind:      sourcev1.GitRepositoryKind,
			Name:      c.GitRepositoryName,
			Namespace: c.GitRepositoryNamespace,
		},
	}
}

func (c *ChartBuilder) makeHelmChartSpec(chart string, version string, installation profilesv1.ProfileInstallation) helmv2.HelmChartTemplateSpec {
	return helmv2.HelmChartTemplateSpec{
		Chart: chart,
		SourceRef: helmv2.CrossNamespaceObjectReference{
			Kind:      sourcev1.HelmRepositoryKind,
			Name:      c.makeHelmRepoName(chart, installation),
			Namespace: installation.ObjectMeta.Namespace,
		},
		Version: version,
	}
}
