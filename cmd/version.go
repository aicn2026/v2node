package cmd

import (
	"fmt"

	C "github.com/sagernet/sing-box/constant"
	"github.com/spf13/cobra"
)

var (
	version  = "TempVersion" //use ldflags replace
	codename = "v2node"
	intro    = "A V2board backend based on embedded sing-box"
)

var versionCommand = cobra.Command{
	Use:   "version",
	Short: "Print version info",
	Run: func(_ *cobra.Command, _ []string) {
		showVersion()
	},
}

func init() {
	command.AddCommand(&versionCommand)
}

func showVersion() {
	fmt.Printf("%s %s (%s)\n", codename, version, intro)
	fmt.Printf("sing-box %s started\n", C.Version)
}
