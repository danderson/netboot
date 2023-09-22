// Copyright 2016 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pixiecore

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// StaticBooter boots all machines with the same Spec.
//
// IDs in spec should be either local file paths, or HTTP/HTTPS URLs.
func StaticBooter(spec *Spec) (Booter, error) {
	ret := &staticBooter{
		kernel: string(spec.Kernel),
		spec: &Spec{
			Kernel:  "kernel",
			Message: spec.Message,
		},
	}
	for i, initrd := range spec.Initrd {
		ret.initrd = append(ret.initrd, string(initrd))
		ret.spec.Initrd = append(ret.spec.Initrd, ID(fmt.Sprintf("initrd-%d", i)))
	}

	f := func(id string) string {
		ret.otherIDs = append(ret.otherIDs, id)
		return fmt.Sprintf("{{ ID \"other-%d\" }}", len(ret.otherIDs)-1)
	}
	cmdline, err := expandCmdline(spec.Cmdline, template.FuncMap{"ID": f})
	if err != nil {
		return nil, err
	}
	ret.spec.Cmdline = cmdline

	return ret, nil
}

type staticBooter struct {
	kernel   string
	initrd   []string
	otherIDs []string

	spec *Spec
}

func (s *staticBooter) BootSpec(m Machine) (*Spec, error) {
	return s.spec, nil
}

func (s *staticBooter) serveFile(path string) (io.ReadCloser, int64, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		resp, err := http.Get(path)
		if err != nil {
			return nil, -1, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, -1, fmt.Errorf("%s: %s", path, http.StatusText(resp.StatusCode))
		}
		return resp.Body, resp.ContentLength, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, -1, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, -1, err
	}
	return f, fi.Size(), nil
}

func (s *staticBooter) ReadBootFile(id ID) (io.ReadCloser, int64, error) {
	path := string(id)
	switch {
	case path == "kernel":
		return s.serveFile(s.kernel)

	case strings.HasPrefix(path, "initrd-"):
		i, err := strconv.Atoi(path[7:])
		if err != nil || i < 0 || i >= len(s.initrd) {
			return nil, -1, fmt.Errorf("no file with ID %q", id)
		}
		return s.serveFile(s.initrd[i])

	case strings.HasPrefix(path, "other-"):
		i, err := strconv.Atoi(path[6:])
		if err != nil || i < 0 || i >= len(s.otherIDs) {
			return nil, -1, fmt.Errorf("no file with ID %q", id)
		}
		return s.serveFile(s.otherIDs[i])
	}

	return nil, -1, fmt.Errorf("no file with ID %q", id)
}

func (s *staticBooter) WriteBootFile(ID, io.Reader) error {
	return nil
}

// APIBooter gets a BootSpec from a remote server over HTTP.
//
// The API is described in README.api.md
func APIBooter(url string, timeout time.Duration, signingKey [32]byte) (Booter, error) {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	ret := &apibooter{
		client:    &http.Client{Timeout: timeout},
		urlPrefix: url + "v1",
		key:       signingKey,
	}
	emptyByteVar := make([]byte, 32)
	if bytes.Equal(signingKey[:], emptyByteVar) {
		if _, err := io.ReadFull(rand.Reader, ret.key[:]); err != nil {
			return nil, fmt.Errorf("failed to get randomness for signing key: %s", err)
		}
	}

	return ret, nil
}

type apibooter struct {
	client    *http.Client
	urlPrefix string
	key       [32]byte
}

func (b *apibooter) getAPIResponse(hw net.HardwareAddr) (io.ReadCloser, error) {
	reqURL := fmt.Sprintf("%s/boot/%s", b.urlPrefix, hw)
	resp, err := b.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", reqURL, http.StatusText(resp.StatusCode))
	}

	return resp.Body, nil
}

func (b *apibooter) BootSpec(m Machine) (*Spec, error) {
	body, err := b.getAPIResponse(m.MAC)
	if body != nil {
		defer body.Close()
	}
	if err != nil {
		return nil, err
	}

	r := struct {
		Kernel     string      `json:"kernel"`
		Initrd     []string    `json:"initrd"`
		Cmdline    interface{} `json:"cmdline"`
		Message    string      `json:"message"`
		IpxeScript string      `json:"ipxe-script"`
	}{}
	if err = json.NewDecoder(body).Decode(&r); err != nil {
		return nil, err
	}

	if r.IpxeScript != "" {
		return &Spec{
			IpxeScript: r.IpxeScript,
		}, nil
	}

	r.Kernel, err = b.makeURLAbsolute(r.Kernel)
	if err != nil {
		return nil, err
	}
	for i, img := range r.Initrd {
		r.Initrd[i], err = b.makeURLAbsolute(img)
		if err != nil {
			return nil, err
		}
	}

	ret := Spec{
		Message: r.Message,
	}
	if ret.Kernel, err = signURL(r.Kernel, &b.key); err != nil {
		return nil, err
	}
	for _, img := range r.Initrd {
		initrd, err := signURL(img, &b.key)
		if err != nil {
			return nil, err
		}
		ret.Initrd = append(ret.Initrd, initrd)
	}

	if r.Cmdline != nil {
		switch c := r.Cmdline.(type) {
		case string:
			ret.Cmdline = c
		case map[string]interface{}:
			ret.Cmdline, err = b.constructCmdline(c)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("API server returned unknown type %T for kernel cmdline", r.Cmdline)
		}
	}

	f := func(u string) (string, error) {
		urlStr, err := b.makeURLAbsolute(u)
		if err != nil {
			return "", fmt.Errorf("invalid url %q for cmdline: %s", urlStr, err)
		}
		id, err := signURL(urlStr, &b.key)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("{{ ID %q }}", id), nil
	}
	ret.Cmdline, err = expandCmdline(ret.Cmdline, template.FuncMap{"URL": f})
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (b *apibooter) ReadBootFile(id ID) (io.ReadCloser, int64, error) {
	urlStr, err := getURL(id, &b.key)
	if err != nil {
		return nil, -1, err
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, -1, fmt.Errorf("%q is not an URL", urlStr)
	}
	var (
		ret io.ReadCloser
		sz  int64
	)
	if u.Scheme == "file" {
		// TODO serveFile
		f, err := os.Open(u.Path)
		if err != nil {
			return nil, -1, err
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, -1, err
		}
		ret, sz = f, fi.Size()
	} else {
		// urlStr will get reparsed by http.Get, which is mildly
		// wasteful, but the code looks nicer than constructing a
		// Request.
		resp, err := http.Get(urlStr)
		if err != nil {
			return nil, -1, err
		}
		if resp.StatusCode != 200 {
			return nil, -1, fmt.Errorf("GET %q failed: %s", urlStr, resp.Status)
		}

		ret, sz, err = resp.Body, resp.ContentLength, nil
		if err != nil {
			return nil, -1, err
		}
	}
	return ret, sz, nil
}

func (b *apibooter) WriteBootFile(id ID, body io.Reader) error {
	u, err := getURL(id, &b.key)
	if err != nil {
		return err
	}

	resp, err := http.Post(u, "application/octet-stream", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("POST %q failed: %s", u, resp.Status)
	}
	defer resp.Body.Close()
	return nil
}

func (b *apibooter) makeURLAbsolute(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("%q is not an URL", urlStr)
	}
	if !u.IsAbs() {
		base, err := url.Parse(b.urlPrefix)
		if err != nil {
			return "", err
		}
		u = base.ResolveReference(u)
	}
	return u.String(), nil
}

func (b *apibooter) constructCmdline(m map[string]interface{}) (string, error) {
	var c []string
	for k := range m {
		c = append(c, k)
	}
	sort.Strings(c)

	var ret []string
	for _, k := range c {
		switch v := m[k].(type) {
		case bool:
			ret = append(ret, k)
		case string:
			ret = append(ret, fmt.Sprintf("%s=%q", k, v))
		case map[string]interface{}:
			urlStr, ok := v["url"].(string)
			if !ok {
				return "", fmt.Errorf("cmdline key %q has object value with no 'url' attribute", k)
			}
			ret = append(ret, fmt.Sprintf("%s={{ URL %q }}", k, urlStr))
		default:
			return "", fmt.Errorf("unsupported value kind %T for cmdline key %q", m[k], k)
		}
	}
	return strings.Join(ret, " "), nil
}
