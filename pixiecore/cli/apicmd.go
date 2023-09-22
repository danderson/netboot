// Copyright © 2016 David Anderson <dave@natulte.net>
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
	"encoding/base64"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.universe.tf/netboot/pixiecore"
)

var apiCmd = &cobra.Command{
	Use:   "api server",
	Short: "Boot machines using instructions from one or more API servers",
	Long: `API mode is a "PXE to HTTP" translator. Whenever Pixiecore sees a
machine trying to PXE boot, it will ask a remote HTTP(S) API server
what to do. The API server can tell Pixiecore to ignore the machine,
or tell it what to boot.

It is your responsibility to implement or run a server that implements
the Pixiecore boot API. The specification can be found at <TODO>.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fatalf("you must specify an API URL")
		}
		server := args[0]
		timeout, err := cmd.Flags().GetDuration("api-request-timeout")
		if err != nil {
			fatalf("Error reading flag: %s", err)
		}

		var key [32]byte

		signingKey, err := cmd.Flags().GetString("signing-key")
		if err != nil {
			fatalf("Error reading flag: %s", err)
		} else {
			if signingKey != "" {
				keydata, err := base64.StdEncoding.DecodeString(signingKey)
				if err != nil {
					fmt.Println("Error decoding signing key (it should be base64 encoded):", err)
				} else {
					if len(keydata) > 32 {
						fatalf("Too long signin key")
					} else {
						copy(key[:], keydata)
					}
				}
			}
		}

		booter, err := pixiecore.APIBooter(server, timeout, key)
		if err != nil {
			fatalf("Failed to create API booter: %s", err)
		}

		s := serverFromFlags(cmd)
		s.Booter = booter

		fmt.Println(s.Serve())
	}}

func init() {
	rootCmd.AddCommand(apiCmd)
	serverConfigFlags(apiCmd)
	apiCmd.Flags().Duration("api-request-timeout", 5*time.Second, "Timeout for request to the API server")
	apiCmd.Flags().String("signing-key", "", "Signing key")
	// TODO: SSL cert flags for both client and server auth.
}
