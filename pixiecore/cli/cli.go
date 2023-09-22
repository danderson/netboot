// Copyright 2016 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cli implements the commandline interface for Pixiecore.
package cli // import "go.universe.tf/netboot/pixiecore/cli"

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.universe.tf/netboot/pixiecore"
)

// Ipxe is the set of ipxe binaries for supported firmwares.
//
// Can be set externally before calling CLI(), and set/extended by
// commandline processing in CLI().
var Ipxe = map[pixiecore.Firmware][]byte{}

// CLI runs the Pixiecore commandline.
//
// This function always exits back to the OS when finished.
func CLI() {
	if v1compatCLI() {
		return
	}

	cobra.OnInitialize(initConfig)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	os.Exit(0)
}

// This represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pixiecore",
	Short: "All-in-one network booting",
	Long:  `Pixiecore is a tool to make network booting easy.`,
}

func initConfig() {
	viper.SetEnvPrefix("pixiecore")
	viper.AutomaticEnv() // read in environment variables that match
}

func fatalf(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
	os.Exit(1)
}

func staticConfigFlags(cmd *cobra.Command) {
	cmd.Flags().String("cmdline", "", "Kernel commandline arguments")
	cmd.Flags().String("bootmsg", "", "Message to print on machines before booting")
}

func serverConfigFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("debug", "d", false, "Log more things that aren't directly related to booting a recognized client")
	cmd.Flags().BoolP("log-timestamps", "t", false, "Add a timestamp to each log line")
	cmd.Flags().StringP("listen-addr", "l", "0.0.0.0", "IPv4 address to listen on")
	cmd.Flags().IntP("port", "p", 80, "Port to listen on for HTTP")
	cmd.Flags().Int("status-port", 0, "HTTP port for status information (can be the same as --port)")
	cmd.Flags().Bool("dhcp-no-bind", false, "Handle DHCP traffic without binding to the DHCP server port")
	cmd.Flags().String("ipxe-bios", "", "Path to an iPXE binary for BIOS/UNDI")
	cmd.Flags().String("ipxe-ipxe", "", "Path to an iPXE binary for chainloading from another iPXE")
	cmd.Flags().String("ipxe-efi32", "", "Path to an iPXE binary for 32-bit UEFI")
	cmd.Flags().String("ipxe-efi64", "", "Path to an iPXE binary for 64-bit UEFI")

	// Development flags, hidden from normal use.
	cmd.Flags().String("ui-assets-dir", "", "UI assets directory (used for development)")
	err := cmd.Flags().MarkHidden("ui-assets-dir")
	if err != nil {
		panic(err)
	}
}

func mustFile(path string) []byte {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		fatalf("couldn't read file %q: %s", path, err)
	}

	return bs
}

func staticFromFlags(cmd *cobra.Command, kernel string, initrds []string, extraCmdline string) *pixiecore.Server {
	cmdline, err := cmd.Flags().GetString("cmdline")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	bootmsg, err := cmd.Flags().GetString("bootmsg")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}

	if extraCmdline != "" {
		cmdline = fmt.Sprintf("%s %s", extraCmdline, cmdline)
	}

	spec := &pixiecore.Spec{
		Kernel:  pixiecore.ID(kernel),
		Cmdline: cmdline,
		Message: bootmsg,
	}
	for _, initrd := range initrds {
		spec.Initrd = append(spec.Initrd, pixiecore.ID(initrd))
	}

	booter, err := pixiecore.StaticBooter(spec)
	if err != nil {
		fatalf("Couldn't make static booter: %s", err)
	}

	s := serverFromFlags(cmd)
	s.Booter = booter

	return s
}

func serverFromFlags(cmd *cobra.Command) *pixiecore.Server {
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	timestamps, err := cmd.Flags().GetBool("log-timestamps")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	addr, err := cmd.Flags().GetString("listen-addr")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	httpPort, err := cmd.Flags().GetInt("port")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	httpStatusPort, err := cmd.Flags().GetInt("status-port")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	dhcpNoBind, err := cmd.Flags().GetBool("dhcp-no-bind")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	ipxeBios, err := cmd.Flags().GetString("ipxe-bios")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	ipxeIpxe, err := cmd.Flags().GetString("ipxe-ipxe")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	ipxeEFI32, err := cmd.Flags().GetString("ipxe-efi32")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	ipxeEFI64, err := cmd.Flags().GetString("ipxe-efi64")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}
	uiAssetsDir, err := cmd.Flags().GetString("ui-assets-dir")
	if err != nil {
		fatalf("Error reading flag: %s", err)
	}

	if httpPort <= 0 {
		fatalf("HTTP port must be >0")
	}

	ret := &pixiecore.Server{
		Ipxe:           map[pixiecore.Firmware][]byte{},
		Log:            logWithStdFmt,
		HTTPPort:       httpPort,
		HTTPStatusPort: httpStatusPort,
		DHCPNoBind:     dhcpNoBind,
		UIAssetsDir:    uiAssetsDir,
	}
	for fwtype, bs := range Ipxe {
		ret.Ipxe[fwtype] = bs
	}
	if ipxeBios != "" {
		ret.Ipxe[pixiecore.FirmwareX86PC] = mustFile(ipxeBios)
	}
	if ipxeIpxe != "" {
		ret.Ipxe[pixiecore.FirmwareX86Ipxe] = mustFile(ipxeIpxe)
	}
	if ipxeEFI32 != "" {
		ret.Ipxe[pixiecore.FirmwareEFI32] = mustFile(ipxeEFI32)
	}
	if ipxeEFI64 != "" {
		ret.Ipxe[pixiecore.FirmwareEFI64] = mustFile(ipxeEFI64)
		ret.Ipxe[pixiecore.FirmwareEFIBC] = ret.Ipxe[pixiecore.FirmwareEFI64]
	}

	if timestamps {
		ret.Log = logWithStdLog
	}
	if debug {
		ret.Debug = ret.Log
	}
	if addr != "" {
		ret.Address = addr
	}

	return ret
}
