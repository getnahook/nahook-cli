package endpoints

import (
	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/commands/cliargs"
	"github.com/getnahook/nahook-cli/internal/commands/session"
)

// newUpdateCommand implements `nahook endpoints update <id>`. Only flags
// the user explicitly passes are sent to the backend — `cmd.Flags().
// Changed(name)` distinguishes "unspecified" from "set to the default",
// matching the PATCH semantics of the Dashboard API.
func newUpdateCommand() *cobra.Command {
	var (
		url           string
		description   string
		isActive      bool
		authUsername  string
		authPassword  string
		metadataPairs []string
		jsonOut       bool
	)

	cmd := &cobra.Command{
		Use:   "update <endpoint-id>",
		Short: "Update fields on an existing webhook endpoint",
		Args:  cliargs.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}

			in := api.UpdateEndpointInput{}
			flags := cmd.Flags()
			if flags.Changed("url") {
				in.URL = &url
			}
			if flags.Changed("description") {
				in.Description = &description
			}
			if flags.Changed("active") {
				in.IsActive = &isActive
			}
			if flags.Changed("auth-username") {
				in.AuthUsername = &authUsername
			}
			if flags.Changed("auth-password") {
				in.AuthPassword = &authPassword
			}
			if flags.Changed("metadata") {
				meta, err := parseMetadataPairs(metadataPairs)
				if err != nil {
					return err
				}
				if meta == nil {
					meta = map[string]string{}
				}
				in.Metadata = &meta
			}

			ep, err := apiClient.UpdateEndpoint(cmd.Context(), args[0], in)
			if err != nil {
				return err
			}
			return renderEndpoint(cmd.OutOrStdout(), ep, jsonOut)
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "new destination URL")
	cmd.Flags().StringVar(&description, "description", "", "new human-readable description (empty to clear)")
	cmd.Flags().BoolVar(&isActive, "active", true, "set the endpoint's active state (true/false)")
	cmd.Flags().StringVar(&authUsername, "auth-username", "", "new basic-auth username (empty to clear)")
	cmd.Flags().StringVar(&authPassword, "auth-password", "", "new basic-auth password (empty to clear)")
	cmd.Flags().StringSliceVar(&metadataPairs, "metadata", nil, "replace metadata; pass key=value, repeat for multiple keys")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of the human-readable key/value view")

	return cmd
}
