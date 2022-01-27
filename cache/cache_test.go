package cache_test

/*
 * Copyright 2022 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import (
	"errors"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zupIT/charlescd_k8s_config_cache/cache"
	"github.com/zupIT/charlescd_k8s_config_cache/cache/mocks"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
)

var _ = Describe("Cache", func() {
	var etag string
	var source string
	var mockCache *mocks.Cache
	var httpClient *mocks.HttpClient
	BeforeEach(func() {

		etag = "etag-example"
		source = "example.com/source"
		mockCache = new(mocks.Cache)
		httpClient = new(mocks.HttpClient)
	})
	Context("When is the first request for a repository", func() {

		It("should do the request and save the etag on cache", func() {

			mockCache.On("Get", source).Return(nil, false)
			httpClient.On("Do", mock.Anything).Return(GetHTTPResponseWithStatusNotModified(etag), nil)
			mockCache.On("Set", source, etag, int64(1)).Times(1).Return(true)
			manifestCache := cache.New(mockCache, httpClient)
			manifests, err := manifestCache.GetManifests(source)
			assert.Equal(GinkgoT(), err, errors.New("first request, not cached yet"))
			assert.Equal(GinkgoT(), len(manifests), 0)
		})
	})

	Context("When is the second request for a repository and the content of repository did not change", func() {
		It("should return the cached manifests", func() {

			mockCache.On("Get", source).Return(etag, true)
			httpClient.On("Do", mock.Anything).Return(GetHTTPResponseWithStatusNotModified(etag), nil)
			mockCache.On("Set", source, etag, int64(1)).Times(1).Return(true)
			mockCache.On("Get", etag).Return(getManifestsCached(), true)
			manifestCache := cache.New(mockCache, httpClient)
			manifests, err := manifestCache.GetManifests(source)
			assert.Equal(GinkgoT(), err, nil)
			assert.Equal(GinkgoT(), len(manifests), 1)
		})
	})

	Context("When is a invalid request ", func() {
		It("should return error", func() {
			errorRequest := errors.New("error sending request")
			mockCache.On("Get", source).Return(etag, true)
			httpClient.On("Do", mock.Anything).Return(GetHTTPResponseWithStatusBadRequest(), errorRequest)
			mockCache.On("Set", source, etag, int64(1)).Times(1).Return(true)
			mockCache.On("Get", etag).Return(getManifestsCached(), true)
			manifestCache := cache.New(mockCache, httpClient)
			manifests, err := manifestCache.GetManifests(source)
			assert.Equal(GinkgoT(), err, errorRequest)
			assert.Equal(GinkgoT(), len(manifests), 0)
		})
	})

	Context("When is the second request for a repository and the content of repository changed", func() {
		It("should not return cached manifests", func() {

			mockCache.On("Get", source).Return(etag, true)
			httpClient.On("Do", mock.Anything).Return(getHTTPResponse(etag), nil)
			mockCache.On("Set", source, etag, int64(1)).Times(1).Return(true)
			manifestCache := cache.New(mockCache, httpClient)
			manifests, err := manifestCache.GetManifests(source)
			assert.Equal(GinkgoT(), err, errors.New("resource modified, should download it again"))
			assert.Equal(GinkgoT(), len(manifests), 0)
		})
	})

	Context("when there is no error on cache operations", func() {
		It("should add manifests to cache successfully", func() {

			mockCache.On("Get", source).Return(etag, true)
			mockCache.On("Set", etag, getManifestsCached(), int64(1)).Times(1).Return(true)
			manifestCache := cache.New(mockCache, httpClient)
			err := manifestCache.Add(source, getManifestsCached())
			assert.Equal(GinkgoT(), err, nil)
		})
	})

	Context("when fails to get a key on cache", func() {
		It("should return error", func() {

			mockCache.On("Get", source).Return(nil, false)
			//mockCache.On("Set", etag, getManifestsCached(), int64(1)).Times(1).Return(true)
			manifestCache := cache.New(mockCache, httpClient)
			err := manifestCache.Add(source, getManifestsCached())
			assert.Equal(GinkgoT(), err, errors.New("error getting etag on cache"))
		})
	})

	Context("when fails to set a key on cache", func() {
		It("should return error", func() {

			mockCache.On("Get", source).Return(etag, true)
			mockCache.On("Set", etag, getManifestsCached(), int64(1)).Times(1).Return(false)
			manifestCache := cache.New(mockCache, httpClient)
			err := manifestCache.Add(source, getManifestsCached())
			assert.Equal(GinkgoT(), err, errors.New("failed to set manifests to cache"))
		})
	})
})

func getManifestsCached() []unstructured.Unstructured {
	manifests := make([]unstructured.Unstructured, 0)
	deployment := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "demo-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": 2,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "demo",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "demo",
						},
					},
					"spec": map[string]interface{}{
						"containers": []map[string]interface{}{
							{
								"name":  "web",
								"image": "nginx:1.12",
								"ports": []map[string]interface{}{
									{
										"name":          "http",
										"protocol":      "TCP",
										"containerPort": 80,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	manifests = append(manifests, deployment)
	return manifests
}

func GetHTTPResponseWithStatusNotModified(etag string) *http.Response {
	response := new(http.Response)
	response.Header = make(map[string][]string)
	response.Header.Set("etag", etag)
	response.StatusCode = http.StatusNotModified
	return response
}

func getHTTPResponse(etag string) *http.Response {
	response := new(http.Response)
	response.Header = make(map[string][]string)
	response.Header.Set("etag", etag)
	response.StatusCode = http.StatusOK
	return response
}

func GetHTTPResponseWithStatusBadRequest() *http.Response {
	response := new(http.Response)
	response.StatusCode = http.StatusBadRequest
	return response
}
