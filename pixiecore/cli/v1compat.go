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

package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.universe.tf/netboot/pixiecore"
)

func v1compatCLI() bool {
	fs := flag.NewFlagSet("main", flag.ContinueOnError)
	fs.Usage = func() {}

	portDHCP := fs.Int("port-dhcp", 67, "Port to listen on for DHCP requests")
	portPXE := fs.Int("port-pxe", 4011, "Port to listen on for PXE requests")
	portTFTP := fs.Int("port-tftp", 69, "Port to listen on for TFTP requests")
	portHTTP := fs.Int("port-http", 70, "Port to listen on for HTTP requests")
	listenAddr := fs.String("listen-addr", "", "Address to listen on (default all)")

	apiServer := fs.String("api", "", "Path to the boot API server")
	apiTimeout := fs.Duration("api-timeout", 5*time.Second, "Timeout on boot API server requests")

	kernelFile := fs.String("kernel", "", "Path to the linux kernel file to boot")
	initrdFile := fs.String("initrd", "", "Comma-separated list of initrds to pass to the kernel")
	kernelCmdline := fs.String("cmdline", "", "Additional arguments for the kernel commandline")

	debug := fs.Bool("debug", false, "Log more things that aren't directly related to booting a recognized client")

	if err := fs.Parse(os.Args[1:]); err != nil {
		// This error path includes passing -h or --help. We want the
		// modern CLI to respond to such cries for help, not the
		// compat CLI.
		return false
	}

	// Successful flag parsing might mean no flags were passed, so
	// heuristically decide if the parsed commandline looks
	// v1-compat-ish: no positional arguments, and one of kernelFile
	// or apiServer passed.
	if fs.NArg() > 0 || (*apiServer == "" && *kernelFile == "") {
		return false
	}

	// Run in compat mode.

	switch {
	case *apiServer != "":
		if *kernelFile != "" {
			fatalf("cannot provide -kernel with -api")
		}
		if *initrdFile != "" {
			fatalf("cannot provide -initrd with -api")
		}
		if *kernelCmdline != "" {
			fatalf("cannot provide -cmdline with -api")
		}

		var key [32]byte

		log.Printf("Starting Pixiecore in API mode, with server %s", *apiServer)
		booter, err := pixiecore.APIBooter(*apiServer, *apiTimeout, key)
		if err != nil {
			fatalf("Failed to create API booter: %s", err)
		}
		s := &pixiecore.Server{
			Booter:   booter,
			Ipxe:     Ipxe,
			Log:      logWithStdLog,
			Address:  *listenAddr,
			HTTPPort: *portHTTP,
			DHCPPort: *portDHCP,
			TFTPPort: *portTFTP,
			PXEPort:  *portPXE,
		}
		if *debug {
			s.Debug = logWithStdLog
		}
		fmt.Println(s.Serve())

	case *kernelFile != "":
		if *apiServer != "" {
			fatalf("cannot provide -api with -kernel")
		}

		log.Printf("Starting Pixiecore in static mode")
		var initrds []string
		if *initrdFile != "" {
			initrds = strings.Split(*initrdFile, ",")
		}
		spec := &pixiecore.Spec{
			Kernel:  pixiecore.ID(*kernelFile),
			Cmdline: *kernelCmdline,
		}
		for _, initrd := range initrds {
			spec.Initrd = append(spec.Initrd, pixiecore.ID(initrd))
		}

		booter, err := pixiecore.StaticBooter(spec)
		if err != nil {
			fatalf("Couldn't make static booter: %s", err)
		}

		s := &pixiecore.Server{
			Booter:   booter,
			Ipxe:     Ipxe,
			Log:      logWithStdLog,
			Address:  *listenAddr,
			HTTPPort: *portHTTP,
			DHCPPort: *portDHCP,
			TFTPPort: *portTFTP,
			PXEPort:  *portPXE,
		}
		if *debug {
			s.Debug = logWithStdLog
		}
		fmt.Println(s.Serve())

	default:
		fatalf("must specify either -api, or -kernel/-initrd")
	}

	return true
}
