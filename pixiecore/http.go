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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"text/template"
)

func (s *Server) httpError(w http.ResponseWriter, r *http.Request, status int, format string, args ...interface{}) {
	s.logHTTP(r, format, args...)
	http.Error(w, fmt.Sprintf(format, args...), status)
}

func (s *Server) handleIpxe(w http.ResponseWriter, r *http.Request) {
	args := r.URL.Query()
	mac, err := net.ParseMAC(args.Get("mac"))
	if err != nil {
		s.httpError(w, r, http.StatusBadRequest, "invalid MAC address %q: %s\n", args.Get("mac"), err)
		return
	}

	i, err := strconv.Atoi(args.Get("arch"))
	if err != nil {
		s.httpError(w, r, http.StatusBadRequest, "invalid architecture %q: %s\n", args.Get("arch"), err)
		return
	}
	arch := Architecture(i)
	switch arch {
	case ArchIA32, ArchX64:
	default:
		s.httpError(w, r, http.StatusBadRequest, "Unknown architecture %q\n", arch)
		return
	}

	mach := Machine{
		MAC:  mac,
		Arch: arch,
	}
	spec, err := s.Booter.BootSpec(mach)
	if err != nil {
		// TODO: maybe don't send this error over the network?
		s.logHTTP(r, "error getting bootspec for %#v: %s", mach, err)
		http.Error(w, "couldn't get a bootspec", http.StatusInternalServerError)
		return
	}
	if spec == nil {
		// TODO: make ipxe abort netbooting so it can fall through to
		// other boot options - unsure if that's possible.
		s.httpError(w, r, http.StatusNotFound, "no bootspec found for %q", mach.MAC)
		return
	}
	script, err := ipxeScript(spec, r.Host)
	if err != nil {
		s.logHTTP(r, "failed to assemble ipxe script: %s", err)
		http.Error(w, "couldn't get a bootspec", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(script)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	f, err := s.Booter.ReadBootFile(ID(name))
	if err != nil {
		s.logHTTP(r, "error getting requested file %q: %s", name, err)
		http.Error(w, "couldn't get file", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if _, err = io.Copy(w, f); err != nil {
		s.logHTTP(r, "copy of file %q failed: %s", name, err)
	}
}

func ipxeScript(spec *Spec, serverHost string) ([]byte, error) {
	if spec.Kernel == "" {
		return nil, errors.New("spec is missing Kernel")
	}

	urlPrefix := fmt.Sprintf("http://%s/_/file?name=", serverHost)
	var b bytes.Buffer
	b.WriteString("#!ipxe\n")
	fmt.Fprintf(&b, "kernel --name kernel %s%s\n", urlPrefix, url.QueryEscape(string(spec.Kernel)))
	for i, initrd := range spec.Initrd {
		fmt.Fprintf(&b, "initrd --name initrd%d %s%s\n", i, urlPrefix, url.QueryEscape(string(initrd)))
	}
	b.WriteString("boot kernel ")
	for i := range spec.Initrd {
		fmt.Fprintf(&b, "initrd=initrd%d ", i)
	}

	f := func(id string) string {
		return fmt.Sprintf("http://%s/_/file?name=%s", serverHost, url.QueryEscape(id))
	}
	cmdline, err := expandCmdline(spec.Cmdline, template.FuncMap{"ID": f})
	if err != nil {
		return nil, fmt.Errorf("expanding cmdline %q: %s", spec.Cmdline, err)
	}
	b.WriteString(cmdline)

	b.WriteByte('\n')
	return b.Bytes(), nil
}