package profile_test

import (
	"fmt"
	"reflect"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/weaveworks/pctl/pkg/profile"
	profilesv1 "github.com/weaveworks/profiles/api/v1alpha1"
)

const (
	subscriptionName     = "mySub"
	namespace            = "default"
	branch               = "main"
	profileName1         = "profileName"
	profileName2         = "profileName2"
	chartName1           = "chartOneArtifactName"
	chartPath1           = "chart/artifact/path-one"
	chartName2           = "chartTwoArtifactName"
	chartPath2           = "chart/artifact/path-two"
	helmChartName1       = "helmChartArtifactName1"
	helmChartChart1      = "helmChartChartName1"
	helmChartURL1        = "https://org.github.io/chart"
	helmChartVersion1    = "8.8.1"
	kustomizeName1       = "kustomizeOneArtifactName"
	kustomizePath1       = "kustomize/artifact/path-one"
	profileSubKind       = "ProfileSubscription"
	profileSubAPIVersion = "weave.works/v1alpha1"
	profileURL           = "https://github.com/org/repo-name"
)

var (
	profileTypeMeta = metav1.TypeMeta{
		Kind:       profileSubKind,
		APIVersion: profileSubAPIVersion,
	}

	kustomizeKind       = kustomizev1.KustomizationKind
	kustomizeAPIVersion = kustomizev1.GroupVersion.String()

	gitRepoKind      = sourcev1.GitRepositoryKind
	sourceAPIVersion = sourcev1.GroupVersion.String()

	helmReleaseKind = helmv2.HelmReleaseKind
	helmRepoKind    = sourcev1.HelmRepositoryKind
	helmAPIVersion  = helmv2.GroupVersion.String()
)

var _ = Describe("Profile", func() {
	var (
		p             *profile.Profile
		pSub          profilesv1.ProfileSubscription
		pDef          profilesv1.ProfileDefinition
		pNestedDef    profilesv1.ProfileDefinition
		pNestedDefURL = "https://github.com/org/repo-name-nested"
	)

	BeforeEach(func() {
		pSub = profilesv1.ProfileSubscription{
			TypeMeta: profileTypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:      subscriptionName,
				Namespace: namespace,
			},
			Spec: profilesv1.ProfileSubscriptionSpec{
				ProfileURL: profileURL,
				Branch:     branch,
				Values: &apiextensionsv1.JSON{
					Raw: []byte(`{"replicaCount": 3,"service":{"port":8081}}`),
				},
				ValuesFrom: []helmv2.ValuesReference{
					{
						Name:     "nginx-values",
						Kind:     "Secret",
						Optional: true,
					},
				},
			},
		}

		pNestedDef = profilesv1.ProfileDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: profileName2,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Profile",
				APIVersion: "profiles.fluxcd.io/profilesv1",
			},
			Spec: profilesv1.ProfileDefinitionSpec{
				Description: "foo",
				Artifacts: []profilesv1.Artifact{
					{
						Name: chartName1,
						Path: chartPath1,
						Kind: profilesv1.HelmChartKind,
					},
				},
			},
		}

		pDef = profilesv1.ProfileDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: profileName1,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Profile",
				APIVersion: "profiles.fluxcd.io/profilesv1",
			},
			Spec: profilesv1.ProfileDefinitionSpec{
				Description: "foo",
				Artifacts: []profilesv1.Artifact{
					{
						Name: profileName2,
						Kind: profilesv1.ProfileKind,
						Profile: &profilesv1.Profile{
							URL:    pNestedDefURL,
							Branch: "main",
						},
					},
					{
						Name: chartName2,
						Path: chartPath2,
						Kind: profilesv1.HelmChartKind,
					},
					{
						Name: kustomizeName1,
						Path: kustomizePath1,
						Kind: profilesv1.KustomizeKind,
					},
					{
						Name: helmChartName1,
						Chart: &profilesv1.Chart{
							URL:     helmChartURL1,
							Name:    helmChartChart1,
							Version: helmChartVersion1,
						},
						Kind: profilesv1.HelmChartKind,
					},
				},
			},
		}

		p.SetProfileGetter(func(repoURL, branch, path string) (profilesv1.ProfileDefinition, error) {
			if profileURL == repoURL {
				return pDef, nil
			}
			return pNestedDef, nil
		})
	})

	Describe("MakeArtifacts", func() {
		It("generates the artifacts", func() {
			o, err := profile.MakeArtifacts(pSub)
			Expect(err).NotTo(HaveOccurred())

			Expect(o).To(HaveLen(7))
			Expect(o[0]).To(HaveTypeMeta(metav1.TypeMeta{Kind: gitRepoKind, APIVersion: sourceAPIVersion}))
			Expect(o[1]).To(HaveTypeMeta(metav1.TypeMeta{Kind: gitRepoKind, APIVersion: sourceAPIVersion}))
			Expect(o[2]).To(HaveTypeMeta(metav1.TypeMeta{Kind: helmReleaseKind, APIVersion: helmAPIVersion}))
			Expect(o[3]).To(HaveTypeMeta(metav1.TypeMeta{Kind: helmReleaseKind, APIVersion: helmAPIVersion}))
			Expect(o[4]).To(HaveTypeMeta(metav1.TypeMeta{Kind: kustomizeKind, APIVersion: kustomizeAPIVersion}))
			Expect(o[5]).To(HaveTypeMeta(metav1.TypeMeta{Kind: helmReleaseKind, APIVersion: helmAPIVersion}))
			Expect(o[6]).To(HaveTypeMeta(metav1.TypeMeta{Kind: helmRepoKind, APIVersion: sourceAPIVersion}))

			gitRefName := fmt.Sprintf("%s-%s-%s", subscriptionName, "repo-name-nested", branch)
			gitRepo := o[1].(*sourcev1.GitRepository)
			Expect(gitRepo.Name).To(Equal(gitRefName))
			Expect(gitRepo.Spec.URL).To(Equal("https://github.com/org/repo-name-nested"))
			Expect(gitRepo.Spec.Reference.Branch).To(Equal(branch))

			helmReleaseName := fmt.Sprintf("%s-%s-%s", subscriptionName, profileName2, chartName1)
			helmRelease := o[2].(*helmv2.HelmRelease)

			Expect(helmRelease.Name).To(Equal(helmReleaseName))
			Expect(helmRelease.Spec.Chart.Spec.Chart).To(Equal(chartPath1))
			Expect(helmRelease.Spec.Chart.Spec.SourceRef).To(Equal(
				helmv2.CrossNamespaceObjectReference{
					Kind:      gitRepoKind,
					Name:      gitRefName,
					Namespace: namespace,
				},
			))
			Expect(helmRelease.GetValues()).To(Equal(map[string]interface{}{
				"replicaCount": float64(3),
				"service": map[string]interface{}{
					"port": float64(8081),
				},
			}))
			Expect(helmRelease.Spec.ValuesFrom).To(Equal([]helmv2.ValuesReference{
				{
					Name:     "nginx-values",
					Kind:     "Secret",
					Optional: true,
				},
			}))

			gitRefName = fmt.Sprintf("%s-%s-%s", subscriptionName, "repo-name", branch)
			gitRepo = o[0].(*sourcev1.GitRepository)
			Expect(gitRepo.Name).To(Equal(gitRefName))
			Expect(gitRepo.Spec.URL).To(Equal("https://github.com/org/repo-name"))
			Expect(gitRepo.Spec.Reference.Branch).To(Equal(branch))

			helmReleaseName = fmt.Sprintf("%s-%s-%s", subscriptionName, profileName1, chartName2)
			helmRelease = o[3].(*helmv2.HelmRelease)
			Expect(helmRelease.Name).To(Equal(helmReleaseName))
			Expect(err).NotTo(HaveOccurred())
			Expect(helmRelease.Spec.Chart.Spec.Chart).To(Equal(chartPath2))
			Expect(helmRelease.Spec.Chart.Spec.SourceRef).To(Equal(
				helmv2.CrossNamespaceObjectReference{
					Kind:      gitRepoKind,
					Name:      gitRefName,
					Namespace: namespace,
				},
			))
			Expect(helmRelease.GetValues()).To(Equal(map[string]interface{}{
				"replicaCount": float64(3),
				"service": map[string]interface{}{
					"port": float64(8081),
				},
			}))
			Expect(helmRelease.Spec.ValuesFrom).To(Equal([]helmv2.ValuesReference{
				{
					Name:     "nginx-values",
					Kind:     "Secret",
					Optional: true,
				},
			}))

			kustomizeName := fmt.Sprintf("%s-%s-%s", subscriptionName, profileName1, kustomizeName1)
			kustomize := o[4].(*kustomizev1.Kustomization)
			Expect(kustomize.Name).To(Equal(kustomizeName))
			Expect(kustomize.Spec.Path).To(Equal(kustomizePath1))
			Expect(kustomize.Spec.TargetNamespace).To(Equal(namespace))
			Expect(kustomize.Spec.Prune).To(BeTrue())
			Expect(kustomize.Spec.Interval).To(Equal(metav1.Duration{Duration: time.Minute * 5}))
			Expect(kustomize.Spec.SourceRef).To(Equal(
				kustomizev1.CrossNamespaceSourceReference{
					Kind:      gitRepoKind,
					Name:      gitRefName,
					Namespace: namespace,
				},
			))

			helmRefName := fmt.Sprintf("%s-%s-%s", subscriptionName, "repo-name", helmChartChart1)
			helmRepo := o[6].(*sourcev1.HelmRepository)
			Expect(helmRepo.Name).To(Equal(helmRefName))
			Expect(helmRepo.Spec.URL).To(Equal(helmChartURL1))

			helmReleaseName = fmt.Sprintf("%s-%s-%s", subscriptionName, profileName1, helmChartName1)
			helmRelease = o[5].(*helmv2.HelmRelease)
			Expect(helmRelease.Name).To(Equal(helmReleaseName))
			Expect(helmRelease.Spec.Chart.Spec.Chart).To(Equal(helmChartChart1))
			Expect(helmRelease.Spec.Chart.Spec.Version).To(Equal(helmChartVersion1))
			Expect(helmRelease.Spec.Chart.Spec.SourceRef).To(Equal(
				helmv2.CrossNamespaceObjectReference{
					Kind:      helmRepoKind,
					Name:      helmRefName,
					Namespace: namespace,
				},
			))
			Expect(helmRelease.GetValues()).To(Equal(map[string]interface{}{
				"replicaCount": float64(3),
				"service": map[string]interface{}{
					"port": float64(8081),
				},
			}))
			Expect(helmRelease.Spec.ValuesFrom).To(Equal([]helmv2.ValuesReference{
				{
					Name:     "nginx-values",
					Kind:     "Secret",
					Optional: true,
				},
			}))
		})

		When("fetching the nested profile definition fails", func() {
			BeforeEach(func() {
				p.SetProfileGetter(func(repoURL, branch, path string) (profilesv1.ProfileDefinition, error) {
					if repoURL == profileURL {
						return pDef, nil
					}
					return pNestedDef, fmt.Errorf("foo")
				})
			})

			It("returns an error", func() {
				_, err := profile.MakeArtifacts(pSub)
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("failed to get profile definition %s on branch %s: foo", pNestedDefURL, branch))))
			})
		})
		When("configured with an invalid artifact", func() {
			When("the Kind of artifact is unknown", func() {
				BeforeEach(func() {
					pDef.Spec.Artifacts[0].Kind = "SomeUnknownKind"
				})

				It("errors", func() {
					_, err := profile.MakeArtifacts(pSub)
					Expect(err).To(MatchError(ContainSubstring("artifact kind \"SomeUnknownKind\" not recognized")))
				})
			})

			When("the nested profile is invalid", func() {
				BeforeEach(func() {
					pNestedDef.Spec.Artifacts[0].Kind = "SomeUnknownKind"
				})

				It("errors", func() {
					_, err := profile.MakeArtifacts(pSub)
					Expect(err).To(MatchError(ContainSubstring("failed to generate resources for nested profile \"profileName2\":")))
				})
			})
			When("helmRepository and path", func() {
				BeforeEach(func() {
					pDef = profilesv1.ProfileDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: profileName1,
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "Profile",
							APIVersion: "profiles.fluxcd.io/profilesv1",
						},
						Spec: profilesv1.ProfileDefinitionSpec{
							Description: "foo",
							Artifacts: []profilesv1.Artifact{
								{
									Name: helmChartName1,
									Chart: &profilesv1.Chart{
										URL:     helmChartURL1,
										Name:    helmChartChart1,
										Version: helmChartVersion1,
									},
									Path: "https://not.empty",
									Kind: profilesv1.HelmChartKind,
								},
							},
						},
					}
				})

				It("errors", func() {
					_, err := profile.MakeArtifacts(pSub)
					Expect(err).To(MatchError(ContainSubstring("validation failed for artifact helmChartArtifactName1: expected exactly one, got both: chart, path")))
				})
			})

			When("profile and path", func() {
				BeforeEach(func() {
					pDef = profilesv1.ProfileDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: profileName1,
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "Profile",
							APIVersion: "profiles.fluxcd.io/profilesv1",
						},
						Spec: profilesv1.ProfileDefinitionSpec{
							Description: "foo",
							Artifacts: []profilesv1.Artifact{
								{
									Name: helmChartName1,
									Profile: &profilesv1.Profile{
										URL:    "example.com",
										Branch: "branch",
									},
									Path: "https://not.empty",
									Kind: profilesv1.HelmChartKind,
								},
							},
						},
					}
				})

				It("errors", func() {
					_, err := profile.MakeArtifacts(pSub)
					Expect(err).To(MatchError(ContainSubstring("validation failed for artifact helmChartArtifactName1: expected exactly one, got both: path, profile")))
				})
			})

			When("helmRepository and profile", func() {
				BeforeEach(func() {
					pDef = profilesv1.ProfileDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: profileName1,
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "Profile",
							APIVersion: "profiles.fluxcd.io/profilesv1",
						},
						Spec: profilesv1.ProfileDefinitionSpec{
							Description: "foo",
							Artifacts: []profilesv1.Artifact{
								{
									Name: helmChartName1,
									Chart: &profilesv1.Chart{
										URL:     helmChartURL1,
										Name:    helmChartChart1,
										Version: helmChartVersion1,
									},
									Profile: &profilesv1.Profile{
										URL:    "example.com",
										Branch: "branch",
									},
									Kind: profilesv1.HelmChartKind,
								},
							},
						},
					}
				})

				It("errors", func() {
					_, err := profile.MakeArtifacts(pSub)
					Expect(err).To(MatchError(ContainSubstring("validation failed for artifact helmChartArtifactName1: expected exactly one, got both: chart, profile")))
				})
			})

			When("profile artifact pointing to itself", func() {
				var (
					pNestedDef2    profilesv1.ProfileDefinition
					pNestedDef2URL = "example.com/nested"
				)
				BeforeEach(func() {
					pNestedDef2 = profilesv1.ProfileDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: profileName2,
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "Profile",
							APIVersion: "profiles.fluxcd.io/profilesv1",
						},
						Spec: profilesv1.ProfileDefinitionSpec{
							Description: "foo",
							Artifacts: []profilesv1.Artifact{
								{
									Name: "recursive",
									Profile: &profilesv1.Profile{
										URL:    profileURL,
										Branch: branch,
									},
									Kind: profilesv1.ProfileKind,
								},
							},
						},
					}
					pNestedDef.Spec.Artifacts = []profilesv1.Artifact{
						{
							Name: "recursive",
							Profile: &profilesv1.Profile{
								URL:    pNestedDef2URL,
								Branch: branch,
							},
							Kind: profilesv1.ProfileKind,
						},
					}

					p.SetProfileGetter(func(repoURL, branch, path string) (profilesv1.ProfileDefinition, error) {
						if repoURL == profileURL {
							return pDef, nil
						}
						if repoURL == pNestedDef2URL {
							return pNestedDef2, nil
						}
						return pNestedDef, nil
					})
				})

				It("errors", func() {
					_, err := profile.MakeArtifacts(pSub)
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("recursive artifact detected: profile %s on branch %s contains an artifact that points recursively back at itself", profileURL, branch))))
				})
			})
		})
	})

})

func HaveTypeMeta(expected metav1.TypeMeta) types.GomegaMatcher {
	return &typeMetaMatcher{
		expected: expected,
	}
}

type typeMetaMatcher struct {
	expected metav1.TypeMeta
}

func (m *typeMetaMatcher) Match(actual interface{}) (bool, error) {
	ro, ok := actual.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("HaveTypeMeta expects a runtime.Object")
	}

	tm, err := m.typeMetaFromObject(ro)
	if err != nil {
		return false, fmt.Errorf("failed to get the type meta for object %#v: %w", ro, err)
	}

	return reflect.DeepEqual(tm, m.expected), nil
}

func (m *typeMetaMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nto have TypeMeta\n\t%#v", actual, m.expected)
}

func (m *typeMetaMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nnot to have TypeMeta\n\t%#v", actual, m.expected)
}

func (m *typeMetaMatcher) typeMetaFromObject(ro runtime.Object) (metav1.TypeMeta, error) {
	ta, err := meta.TypeAccessor(ro)
	if err != nil {
		return metav1.TypeMeta{}, fmt.Errorf("failed to get the type meta for object %#v: %w", ro, err)
	}
	return metav1.TypeMeta{APIVersion: ta.GetAPIVersion(), Kind: ta.GetKind()}, nil
}
