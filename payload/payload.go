package payload

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Create the payload command
var PayloadCmd = &cobra.Command{
	Use:   "payload [version] [stream]",
	Short: "View payload of release-controller",
	Long:  `View payload of release-controller (add more detail)`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Your payload command logic here
		//version := args[0]
		//stream := args[1]
		fmt.Println("payload called")
		// Rest of your code...
	},
}
