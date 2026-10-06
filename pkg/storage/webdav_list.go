package storage

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
)

type webDavMultiStatus struct {
	XMLName   xml.Name         `xml:"multistatus"`
	Responses []webDavResponse `xml:"response"`
}

type webDavResponse struct {
	Href      string             `xml:"href"`
	PropStats []webDavPropStatus `xml:"propstat"`
}

type webDavPropStatus struct {
	Prop webDavProperty `xml:"prop"`
}

type webDavProperty struct {
	ResourceType  *webDavResourceType `xml:"resourcetype"`
	ContentLength int64               `xml:"getcontentlength"`
	LastModified  string              `xml:"getlastmodified"`
}

type webDavResourceType struct {
	Collection *struct{} `xml:"collection"`
}

// List returns metadata of all objects under the specified prefix
func (s *WebDAVObjectStorage) List(ctx core.Context, prefix string) ([]ObjectMeta, error) {
	listPrefix := s.getListPrefix(prefix)
	requestUrl := s.getUrlWithPath(listPrefix)
	rootListPrefix := s.getListPrefix("")
	absoluteRootPathPrefix := s.getAbsolutePathPrefixFromListUrl(s.getUrlWithPath(rootListPrefix))

	req, err := http.NewRequest("PROPFIND", requestUrl, bytes.NewReader([]byte(webDavListAllObjectsRequestBody)))

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(s.webDavConfig.Username, s.webDavConfig.Password)
	req.Header.Set("Depth", "infinity")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := s.httpClient.Do(req)

	if err != nil {
		log.Errorf(ctx, "[webdav_storage.List] cannot list objects, because %s", err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Errorf(ctx, "[webdav_storage.List] cannot read response body, because %s", err.Error())
		return nil, err
	}

	if resp.StatusCode != http.StatusMultiStatus {
		log.Errorf(ctx, "[webdav_storage.List] cannot list objects, http status code is %d, response is %s", resp.StatusCode, string(body))
		return nil, errs.ErrSystemError
	}

	multiStatus := &webDavMultiStatus{}

	if err := xml.Unmarshal(body, multiStatus); err != nil {
		log.Errorf(ctx, "[webdav_storage.List] cannot parse multistatus response, because %s", err.Error())
		return nil, err
	}

	objects := make([]ObjectMeta, 0)

	for i := 0; i < len(multiStatus.Responses); i++ {
		response := &multiStatus.Responses[i]
		relativePath := s.getRelativeObjectPath(response.Href, rootListPrefix, absoluteRootPathPrefix)

		if relativePath == "" || strings.HasSuffix(relativePath, "/") {
			continue
		}

		property := s.getResponseMainProperty(response)

		if property == nil {
			continue
		}

		if property.ResourceType != nil && property.ResourceType.Collection != nil {
			continue
		}

		var lastModifiedTime time.Time

		if property.LastModified != "" {
			lastModifiedTime, err = http.ParseTime(property.LastModified)

			if err != nil {
				lastModifiedTime = time.Time{}
			}
		}

		objects = append(objects, ObjectMeta{
			Path:         strings.TrimPrefix(relativePath, "/"),
			Size:         property.ContentLength,
			LastModified: lastModifiedTime,
		})
	}

	return objects, nil
}

func (s *WebDAVObjectStorage) getListPrefix(prefix string) string {
	listPrefix := s.getFinalPath(prefix)

	if listPrefix != "" && !strings.HasSuffix(listPrefix, "/") {
		listPrefix = listPrefix + "/"
	}

	return listPrefix
}

func (s *WebDAVObjectStorage) getUrlWithPath(path string) string {
	finalUrl := s.webDavConfig.Url

	if len(finalUrl) < 1 || finalUrl[len(finalUrl)-1] != '/' {
		finalUrl = finalUrl + "/"
	}

	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	return finalUrl + path
}

func (s *WebDAVObjectStorage) getAbsolutePathPrefixFromListUrl(requestUrl string) string {
	parsedUrl, err := url.Parse(requestUrl)

	if err != nil {
		return ""
	}

	return parsedUrl.Path
}

func (s *WebDAVObjectStorage) getRelativeObjectPath(href string, listPrefix string, absoluteListPathPrefix string) string {
	if href == "" {
		return ""
	}

	rawPath := href

	if parsedUrl, err := url.Parse(href); err == nil && parsedUrl.Path != "" {
		if parsedUrl.Host != "" || strings.HasPrefix(href, parsedUrl.Path) {
			rawPath = parsedUrl.Path
		}
	}

	unescapedPath, err := url.PathUnescape(rawPath)

	if err != nil {
		unescapedPath = rawPath
	}

	unescapedPath = strings.ReplaceAll(unescapedPath, "\\", "/")

	if absoluteListPathPrefix != "" {
		if trimmedPath, ok := strings.CutPrefix(unescapedPath, absoluteListPathPrefix); ok {
			return trimmedPath
		}
	}

	if trimmedPath, ok := strings.CutPrefix(unescapedPath, listPrefix); ok {
		return trimmedPath
	}

	return strings.TrimPrefix(unescapedPath, "/")
}

func (s *WebDAVObjectStorage) getResponseMainProperty(response *webDavResponse) *webDavProperty {
	var property *webDavProperty

	for i := 0; i < len(response.PropStats); i++ {
		propstat := &response.PropStats[i]

		if property == nil {
			property = &propstat.Prop
		} else {
			if property.ResourceType == nil && propstat.Prop.ResourceType != nil {
				property.ResourceType = propstat.Prop.ResourceType
			}

			if property.ContentLength == 0 && propstat.Prop.ContentLength != 0 {
				property.ContentLength = propstat.Prop.ContentLength
			}

			if property.LastModified == "" && propstat.Prop.LastModified != "" {
				property.LastModified = propstat.Prop.LastModified
			}
		}
	}

	return property
}

const webDavListAllObjectsRequestBody = `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
<D:prop>
<D:getcontentlength/>
<D:getlastmodified/>
<D:resourcetype/>
</D:prop>
</D:propfind>`
