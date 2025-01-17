package cluster_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/weaveworks/pctl/pkg/cluster"
	"github.com/weaveworks/pctl/pkg/cluster/fakes"
	runnerfake "github.com/weaveworks/pctl/pkg/runner/fakes"
)

type mockTransport struct {
	res *http.Response
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.res, nil
}

var _ = Describe("prepare", func() {
	var (
		waiter          *fakes.FakeWaiter
		applyRunner     *runnerfake.FakeRunner
		preflightRunner *runnerfake.FakeRunner
	)

	BeforeEach(func() {
		waiter = &fakes.FakeWaiter{}
		applyRunner = &runnerfake.FakeRunner{}
		preflightRunner = &runnerfake.FakeRunner{}
	})

	When("dry run is set", func() {
		It("can prepare the environment with everything that profiles needs without actually modifying the cluster", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_01")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					Location: tmp,
					DryRun:   true,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Waiter: waiter,
					Runner: applyRunner,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			Expect(preflightRunner.RunCallCount()).To(Equal(2))

			Expect(applyRunner.RunCallCount()).To(Equal(1))
			arg, args := applyRunner.RunArgsForCall(0)
			Expect(arg).To(Equal("kubectl"))
			Expect(args).To(Equal([]string{"apply", "-f", filepath.Join(tmp, "prepare.yaml"), "--dry-run=client", "--output=yaml"}))
			Expect(waiter.WaitCallCount()).To(Equal(0))
		})
	})
	When("dry-run is not set", func() {
		It("sets up the environment with everything that profiles needs", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_02")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:  "https://github.com/weaveworks/profiles/releases",
					Location: tmp,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Waiter: waiter,
					Runner: applyRunner,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			Expect(preflightRunner.RunCallCount()).To(Equal(2))
			Expect(applyRunner.RunCallCount()).To(Equal(1))
			arg, args := applyRunner.RunArgsForCall(0)
			Expect(arg).To(Equal("kubectl"))
			Expect(args).To(Equal([]string{"apply", "-f", filepath.Join(tmp, "prepare.yaml")}))
			Expect(waiter.WaitCallCount()).To(Equal(1))
			args = waiter.WaitArgsForCall(0)
			Expect(args).To(Equal([]string{"profiles-controller-manager"}))
		})
	})
	When("context and config is provided", func() {
		It("passes that over to kubernetes", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_02")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:     "https://github.com/weaveworks/profiles/releases",
					Location:    tmp,
					KubeContext: "context",
					KubeConfig:  "kubeconfig",
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Waiter: waiter,
					Runner: applyRunner,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			Expect(applyRunner.RunCallCount()).To(Equal(1))
			arg, args := applyRunner.RunArgsForCall(0)
			Expect(arg).To(Equal("kubectl"))
			Expect(args).To(Equal([]string{"apply", "-f", filepath.Join(tmp, "prepare.yaml"), "--context=context", "--kubeconfig=kubeconfig"}))
			Expect(waiter.WaitCallCount()).To(Equal(1))
			args = waiter.WaitArgsForCall(0)
			Expect(args).To(Equal([]string{"profiles-controller-manager"}))
		})
	})
	When("there is an error running kubectl apply", func() {
		It("will fail and show a proper error to the user", func() {
			applyRunner.RunReturns([]byte("nope"), errors.New("nope"))
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "specific_version_01")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:     "https://github.com/weaveworks/profiles/releases",
					KubeContext: "context",
					KubeConfig:  "kubeconfig",
					Location:    tmp,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).To(MatchError("install failed: nope"))
		})
	})
	When("a specific version is defined", func() {
		It("will respect the version and try to download that", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.String()).To(Equal("/download/v0.0.1/prepare.yaml"))
			}))
			tmp, err := ioutil.TempDir("", "specific_version_01")
			Expect(err).NotTo(HaveOccurred())
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:     server.URL,
					Version:     "v0.0.1",
					KubeContext: "context",
					KubeConfig:  "kubeconfig",
					Location:    tmp,
				},
				Fetcher: &cluster.Fetcher{
					Client: server.Client(),
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			// we deliberately ignore the error here. the important part is the called url.
			_ = p.Prepare()
		})
		It("the controller has the right version in the file", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_change_version_01")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					Location: tmp,
					DryRun:   true,
					Keep:     true,
					Version:  "v0.0.1",
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Waiter: waiter,
					Runner: applyRunner,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			content, err = ioutil.ReadFile(filepath.Join(tmp, "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("weaveworks/profiles-controller:v0.0.1"))
			Expect(waiter.WaitCallCount()).To(Equal(0))

		})
		It("will only try and fetch versions starting with (v)", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.String()).To(Equal("/latest/download/prepare.yaml"))
			}))
			tmp, err := ioutil.TempDir("", "specific_version_01")
			Expect(err).NotTo(HaveOccurred())
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:     server.URL,
					Version:     "0.0.1",
					KubeContext: "context",
					KubeConfig:  "kubeconfig",
					Location:    tmp,
				},
				Fetcher: &cluster.Fetcher{
					Client: server.Client(),
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			// we deliberately ignore the error here. the important part is the called url.
			_ = p.Prepare()
		})
	})
	When("there is an error accessing the given URL", func() {
		It("will provide a sensible failure", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			}))
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:     server.URL,
					KubeContext: "context",
					KubeConfig:  "kubeconfig",
				},
				Fetcher: &cluster.Fetcher{
					Client: server.Client(),
				},
				Runner: preflightRunner,
			}
			err := p.Prepare()
			msg := fmt.Sprintf("failed to download prepare.yaml from %s/latest/download/prepare.yaml, status: 502 Bad Gateway", server.URL)
			Expect(err).To(MatchError(msg))
		})
	})
	When("the base url is invalid", func() {
		It("will provide a sensible failure", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL: "invalid",
				},
				Fetcher: &cluster.Fetcher{
					Client: http.DefaultClient,
				},
				Runner: preflightRunner,
			}
			err := p.Prepare()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported protocol scheme"))
		})
	})
	When("the user decided to keep the downloaded file(s)", func() {
		It("will not delete the downloaded file(s)", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_should_keep_things")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:  "https://github.com/weaveworks/profiles/releases",
					Location: tmp,
					Keep:     true,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Waiter: waiter,
					Runner: applyRunner,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			downloadedContent, err := ioutil.ReadFile(filepath.Join(tmp, "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(downloadedContent).To(Equal(content))
		})
	})
	When("when all is done", func() {
		It("should remove any temporary folders", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_should_be_deleted_01")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:  "https://github.com/weaveworks/profiles/releases",
					Location: tmp,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Waiter: waiter,
					Runner: applyRunner,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Stat(tmp)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})
	When("the waiter fails to wait", func() {
		It("prepare should fail in a meaningful way", func() {
			waiter.WaitReturns(errors.New("nope"))
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			tmp, err := ioutil.TempDir("", "prepare_should_be_deleted_01")
			Expect(err).NotTo(HaveOccurred())
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:  "https://github.com/weaveworks/profiles/releases",
					Location: tmp,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to wait for resources to be ready: nope"))
		})
	})
	When("prepare is executed", func() {
		It("runs a preflight check which will determine if prepare can run", func() {
			preflightRunner.RunReturnsOnCall(1, []byte("bucket gitrepository helmchart helmrelease helmrepository kustomization"), nil)
			tmp, err := ioutil.TempDir("", "prepare_preflight_check_01")
			Expect(err).NotTo(HaveOccurred())
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:       "https://github.com/weaveworks/profiles/releases",
					Location:      tmp,
					FluxNamespace: "flux",
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			Expect(preflightRunner.RunCallCount()).To(Equal(2))
			arg, args := preflightRunner.RunArgsForCall(0)
			Expect(arg).To(Equal("kubectl"))
			Expect(args).To(Equal([]string{"get", "namespace", "flux", "--output", "name"}))

			arg, args = preflightRunner.RunArgsForCall(1)
			Expect(arg).To(Equal("kubectl"))
			Expect(args).To(Equal([]string{"get", "crds", "--output", "jsonpath='{.items[*].spec.names.singular}'"}))
		})
	})
	When("the flux namespace is not there", func() {
		It("will run nothing else until that is resolved", func() {
			preflightRunner.RunReturns(nil, errors.New("nope"))
			tmp, err := ioutil.TempDir("", "prepare_preflight_check_02")
			Expect(err).NotTo(HaveOccurred())
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:       "https://github.com/weaveworks/profiles/releases",
					Location:      tmp,
					FluxNamespace: "flux",
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).To(MatchError("failed to get flux namespace: nope\nTo ignore this error, please see the  --ignore-preflight-checks flag."))
			Expect(applyRunner.RunCallCount()).To(Equal(0))
		})
	})
	When("one of the flux crds is missing", func() {
		It("will run nothing else until that is resolved", func() {
			preflightRunner.RunReturnsOnCall(2, nil, errors.New("nope"))
			tmp, err := ioutil.TempDir("", "prepare_preflight_check_02")
			Expect(err).NotTo(HaveOccurred())
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:       "https://github.com/weaveworks/profiles/releases",
					Location:      tmp,
					FluxNamespace: "flux",
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).To(MatchError("failed to get crd helmrelease\nTo ignore this error, please see the  --ignore-preflight-checks flag."))
			Expect(applyRunner.RunCallCount()).To(Equal(0))
		})
	})
	When("the user decides to ignore preflight-check errors", func() {
		It("will output them as a warning but will not stop execution", func() {
			preflightRunner.RunReturns(nil, errors.New("nope"))
			tmp, err := ioutil.TempDir("", "prepare_preflight_check_02")
			Expect(err).NotTo(HaveOccurred())
			content, err := ioutil.ReadFile(filepath.Join("testdata", "prepare.yaml"))
			Expect(err).NotTo(HaveOccurred())
			client := &http.Client{
				Transport: &mockTransport{
					res: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewReader(content)),
					},
				},
			}
			p := &cluster.Preparer{
				PrepConfig: cluster.PrepConfig{
					BaseURL:               "https://github.com/weaveworks/profiles/releases",
					Location:              tmp,
					FluxNamespace:         "flux",
					IgnorePreflightErrors: true,
				},
				Fetcher: &cluster.Fetcher{
					Client: client,
				},
				Applier: &cluster.Applier{
					Runner: applyRunner,
					Waiter: waiter,
				},
				Runner: preflightRunner,
			}
			err = p.Prepare()
			Expect(err).NotTo(HaveOccurred())
			Expect(applyRunner.RunCallCount()).To(Equal(1))
		})
	})
})
