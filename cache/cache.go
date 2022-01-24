package cache

import (
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
	"strings"
)

type Cache interface {
	Get(key interface{}) (interface{}, bool)
	Set(key, value interface{}, cost int64) bool
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Wrapper struct {
	cache      Cache
	httpClient HttpClient
}

//GetManifests checks using the etag of resource if the resource is modified on github using conditional requests
//( https://docs.github.com/en/rest/overview/resources-in-the-rest-api#conditional-requests)
// if is not modified return manifests stored on cache
func (w Wrapper) GetManifests(source string) ([]unstructured.Unstructured, error) {
	var unstructuredManifests []unstructured.Unstructured
	var etag string
	repo, owner := w.getRepoOwner(source)
	apiUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	etagItem, got := w.cache.Get(source)
	if !got {
		response, err := w.doRequest(apiUrl, map[string]string{})
		if err != nil {
			return nil, err
		}
		etag = response.Header.Get("ETag")
		set := w.cache.Set(source, etag, 1)
		if !set {
			return nil, err
		}
		return nil, errors.New("first request, not cached yet")
	}
	etag = etagItem.(string)
	headers := map[string]string{
		"If-None-Match": etag,
	}
	response, err := w.doRequest(apiUrl, headers)
	if err != nil {
		return unstructuredManifests, err
	}
	if response.StatusCode == http.StatusNotModified {
		item, got := w.cache.Get(etag)
		if !got {
			return nil, fmt.Errorf("failed to get value from key %s", item)
		}
		unstructuredManifests = item.([]unstructured.Unstructured)
		if err != nil {
			return nil, err
		}
		return unstructuredManifests, nil
	}
	return nil, errors.New("resource modified, should download it again")
}

func (w Wrapper) doRequest(url string, headers map[string]string) (*http.Response, error) {
	request, err := http.NewRequest("GET", url, nil)

	for key, value := range headers {
		request.Header.Add(key, value)
	}
	if err != nil {
		return nil, err
	}
	response, err := w.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (w Wrapper) getRepoOwner(source string) (string, string) {
	arrSource := strings.Split(source, "/")
	return arrSource[len(arrSource)-1], arrSource[len(arrSource)-2]
}

// Add store manifests on cache
func (w Wrapper) Add(source string, manifests []unstructured.Unstructured) error {
	itemETag, got := w.cache.Get(source)
	if !got {
		return errors.New("error getting etag on cache")
	}
	set := w.cache.Set(itemETag, manifests, 1)
	if !set {
		return errors.New("failed to set manifests to cache")
	}
	return nil
}

func New(client Cache, httpClient HttpClient) Wrapper {
	return Wrapper{
		cache:      client,
		httpClient: httpClient,
	}
}
