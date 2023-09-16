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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var quickCmd = &cobra.Command{
	Use:   "quick recipe [settings...]",
	Short: "Boot an OS from a list",
	Long: `This ends up working the same as the simple boot command, but saves
you having to find the kernels and ramdisks for popular OSes.

TODO: better help here
`,
}

func debianRecipe(parent *cobra.Command) {
	versions := []string{
		"oldstable",
		"stable",
		"testing",
		"unstable",

		"wheezy",
		"jessie",
		"stretch",
		"buster",
		"sid",
	}

	debCmd := &cobra.Command{
		Use:   "debian version",
		Short: "Boot a Debian installer",
		Long:  fmt.Sprintf("Boot a Debian installer for the given version (one of %s)", strings.Join(versions, ",")),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fatalf("you must specify a Debian version")
			}
			var version string
			for _, v := range versions {
				if args[0] == v {
					version = v
					break
				}
			}
			if version == "" {
				fatalf("Unknown Debian version %q", version)
			}

			arch, err := cmd.Flags().GetString("arch")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}
			mirror, err := cmd.Flags().GetString("mirror")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}

			kernel := fmt.Sprintf("%s/dists/%s/main/installer-%s/current/images/netboot/debian-installer/%s/linux", mirror, version, arch, arch)
			initrd := fmt.Sprintf("%s/dists/%s/main/installer-%s/current/images/netboot/debian-installer/%s/initrd.gz", mirror, version, arch, arch)

			fmt.Println(staticFromFlags(cmd, kernel, []string{initrd}, "").Serve())
		},
	}

	debCmd.Flags().String("arch", "amd64", "CPU architecture of the Debian installer files")
	debCmd.Flags().String("mirror", "https://mirrors.kernel.org/debian", "Root of the debian mirror to use")
	serverConfigFlags(debCmd)
	staticConfigFlags(debCmd)
	parent.AddCommand(debCmd)
}

func ubuntuRecipe(parent *cobra.Command) {
	versions := []string{
		"precise",
		"trusty",
		"xenial",
		"bionic",
		"cosmic",
		"disco",
		"eoan",
		"focal",
		"groovy",
	}

	ubuntuCmd := &cobra.Command{
		Use:   "ubuntu version",
		Short: "Boot an Ubuntu installer",
		Long:  fmt.Sprintf("Boot an Ubuntu installer for the given version (one of %s)", strings.Join(versions, ",")),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fatalf("you must specify an Ubuntu version")
			}
			var version string
			for _, v := range versions {
				if args[0] == v {
					version = v
					break
				}
			}
			if version == "" {
				fatalf("Unknown Ubuntu version %q", version)
			}

			arch, err := cmd.Flags().GetString("arch")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}
			mirror, err := cmd.Flags().GetString("mirror")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}

			imageDir := "images"
			if version[0] >= 'f' {
				imageDir = "legacy-images"
			}

			kernel := fmt.Sprintf("%s/dists/%s/main/installer-%s/current/%s/netboot/ubuntu-installer/%s/linux", mirror, version, arch, imageDir, arch)
			initrd := fmt.Sprintf("%s/dists/%s/main/installer-%s/current/%s/netboot/ubuntu-installer/%s/initrd.gz", mirror, version, arch, imageDir, arch)

			fmt.Println(staticFromFlags(cmd, kernel, []string{initrd}, "").Serve())
		},
	}

	ubuntuCmd.Flags().String("arch", "amd64", "CPU architecture of the Ubuntu installer files")
	ubuntuCmd.Flags().String("mirror", "https://mirrors.kernel.org/ubuntu", "Root of the ubuntu mirror to use")
	serverConfigFlags(ubuntuCmd)
	staticConfigFlags(ubuntuCmd)
	parent.AddCommand(ubuntuCmd)
}

func fedoraRecipe(parent *cobra.Command) {
	versions := []string{
		"29",
		"30",
		"31",
		"32",
		"33",
	}

	fedoraCmd := &cobra.Command{
		Use:   "fedora version",
		Short: "Boot a Fedora installer",
		Long:  fmt.Sprintf(`Boot a Fedora installer for the given version (one of %s)`, strings.Join(versions, ",")),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fatalf("you must specify a Fedora version")
			}
			var version string
			for _, v := range versions {
				if args[0] == v {
					version = v
					break
				}
			}
			if version == "" {
				fatalf("Unknown Fedora version %q", version)
			}

			arch, err := cmd.Flags().GetString("arch")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}
			mirror, err := cmd.Flags().GetString("mirror")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}

			kernel := fmt.Sprintf("%s/releases/%s/Server/%s/os/images/pxeboot/vmlinuz", mirror, version, arch)
			initrd := fmt.Sprintf("%s/releases/%s/Server/%s/os/images/pxeboot/initrd.img", mirror, version, arch)
			stage2 := fmt.Sprintf("inst.stage2=%s/releases/%s/Server/%s/os/", mirror, version, arch)

			fmt.Println(staticFromFlags(cmd, kernel, []string{initrd}, stage2).Serve())
		},
	}

	fedoraCmd.Flags().String("arch", "x86_64", "CPU architecture of the Fedora installer files")
	// TODO: workstation/server variant
	fedoraCmd.Flags().String("mirror", "https://mirrors.kernel.org/fedora", "Root of the fedora mirror to use")
	serverConfigFlags(fedoraCmd)
	staticConfigFlags(fedoraCmd)
	parent.AddCommand(fedoraCmd)
}

func centosRecipe(parent *cobra.Command) {
	versions := []string{
		"5",
		"6",
		"7",
		"8",
	}

	centosCmd := &cobra.Command{
		Use:   "centos version",
		Short: "Boot a Centos installer",
		Long:  fmt.Sprintf(`Boot a Centos installer for the given version (one of %s)`, strings.Join(versions, ",")),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fatalf("you must specify a Centos version")
			}
			var version string
			for _, v := range versions {
				if args[0] == v {
					version = v
					break
				}
			}
			if version == "" {
				fatalf("Unknown Centos version %q", version)
			}

			arch, err := cmd.Flags().GetString("arch")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}
			mirror, err := cmd.Flags().GetString("mirror")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}

			kernel := fmt.Sprintf("%s/%s/os/%s/images/pxeboot/vmlinuz", mirror, version, arch)
			initrd := fmt.Sprintf("%s/%s/os/%s/images/pxeboot/initrd.img", mirror, version, arch)
			stage2 := fmt.Sprintf("inst.stage2=%s/%s/os/%s/", mirror, version, arch)

			fmt.Println(staticFromFlags(cmd, kernel, []string{initrd}, stage2).Serve())
		},
	}

	centosCmd.Flags().String("arch", "x86_64", "CPU architecture of the Centos installer files")
	centosCmd.Flags().String("mirror", "https://mirrors.kernel.org/centos", "Root of the centos mirror to use")
	serverConfigFlags(centosCmd)
	staticConfigFlags(centosCmd)
	parent.AddCommand(centosCmd)
}

func coreosRecipe(parent *cobra.Command) {
	versions := []string{
		"stable",
		"beta",
		"alpha",
	}

	var coreosCmd = &cobra.Command{
		Use:   "coreos version",
		Short: "Boot a CoreOS installer",
		Long:  fmt.Sprintf(`Boot a CoreOS installer for the given version (one of %s)`, strings.Join(versions, ",")),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fatalf("you must specify a CoreOS version")
			}
			var version string
			for _, v := range versions {
				if args[0] == v {
					version = v
					break
				}
			}
			if version == "" {
				fatalf("Unknown CoreOS version %q", version)
			}

			arch, err := cmd.Flags().GetString("arch")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}

			kernel := fmt.Sprintf("https://%s.release.core-os.net/%s-usr/current/coreos_production_pxe.vmlinuz", version, arch)
			initrd := fmt.Sprintf("https://%s.release.core-os.net/%s-usr/current/coreos_production_pxe_image.cpio.gz", version, arch)

			fmt.Println(staticFromFlags(cmd, kernel, []string{initrd}, "").Serve())
		},
	}

	coreosCmd.Flags().String("arch", "amd64", "CPU architecture of the CoreOS installer files")
	serverConfigFlags(coreosCmd)
	staticConfigFlags(coreosCmd)
	parent.AddCommand(coreosCmd)
}

func netbootRecipe(parent *cobra.Command) {
	var netbootCmd = &cobra.Command{
		Use:   "xyz",
		Short: "Boot a netboot.xyz installer",
		Long: `https://network.xyz allows to boot multiple operating
	systems and useful system utilities.`,
		Run: func(cmd *cobra.Command, args []string) {
			kernel := "https://boot.netboot.xyz/ipxe/netboot.xyz.lkrn"
			fmt.Println(staticFromFlags(cmd, kernel, []string{}, "").Serve())
		},
	}
	serverConfigFlags(netbootCmd)
	staticConfigFlags(netbootCmd)
	parent.AddCommand(netbootCmd)
}

func archRecipe(parent *cobra.Command) {
	archCmd := &cobra.Command{
		Use:   "arch [version]",
		Short: "Boot Arch Linux live image",
		Long: `Boot Arch Linux live image for the given version
version defaults to latest, can also be a YYYY.MM.DD iso release version`,
		Run: func(cmd *cobra.Command, args []string) {
			version := "latest"
			if len(args) >= 1 {
				version = args[0]
			}

			arch := "x86_64"
			mirror, err := cmd.Flags().GetString("mirror")
			if err != nil {
				fatalf("Error reading flag: %s", err)
			}

			httpSrv := fmt.Sprintf("%s/iso/%s", mirror, version)
			kernel := fmt.Sprintf("%s/arch/boot/%s/vmlinuz-linux", httpSrv, arch)
			initrd := fmt.Sprintf("%s/arch/boot/%s/initramfs-linux.img", httpSrv, arch)
			cmdline := fmt.Sprintf("archisobasedir=arch archiso_http_srv=%s/ ip=dhcp cms_verify=y net.ifnames=0", httpSrv)

			fmt.Println(staticFromFlags(cmd, kernel, []string{initrd}, cmdline).Serve())
		},
	}
	archCmd.Flags().String("mirror", "https://mirrors.kernel.org/archlinux", "Root of the archlinux mirror to use")
	serverConfigFlags(archCmd)
	staticConfigFlags(archCmd)
	parent.AddCommand(archCmd)
}

func init() {
	rootCmd.AddCommand(quickCmd)
	debianRecipe(quickCmd)
	ubuntuRecipe(quickCmd)
	fedoraRecipe(quickCmd)
	centosRecipe(quickCmd)
	netbootRecipe(quickCmd)
	coreosRecipe(quickCmd)
	archRecipe(quickCmd)

	// TODO: some kind of caching support where quick OSes get
	// downloaded locally, so you don't have to fetch from a remote
	// server on every boot attempt.
}
