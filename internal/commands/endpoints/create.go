package endpoints

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/commands/session"
)

func newCreateCommand() *cobra.Command {
	var (
		url           string
		endpointType  string
		description   string
		environment   string
		authUsername  string
		authPassword  string
		metadataPairs []string
		jsonOut       bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new webhook endpoint",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if url == "" {
				return errors.New("--url is required")
			}

			_, apiClient, err := session.Require()
			if err != nil {
				return err
			}

			envID, err := resolveEnvironment(cmd, apiClient, environment)
			if err != nil {
				return err
			}

			metadata, err := parseMetadataPairs(metadataPairs)
			if err != nil {
				return err
			}

			in := api.CreateEndpointInput{
				Type:          api.EndpointType(endpointType),
				URL:           url,
				Description:   description,
				AuthUsername:  authUsername,
				AuthPassword:  authPassword,
				Metadata:      metadata,
				EnvironmentID: envID,
			}
			ep, err := apiClient.CreateEndpoint(cmd.Context(), in)
			if err != nil {
				return err
			}
			return renderEndpoint(cmd.OutOrStdout(), ep, jsonOut)
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "destination URL the endpoint will deliver to (required)")
	cmd.Flags().StringVar(&endpointType, "type", "webhook", "endpoint type: webhook | slack")
	cmd.Flags().StringVar(&description, "description", "", "human-readable description")
	cmd.Flags().StringVar(&environment, "environment", "", "environment slug or ID (defaults to the workspace's default environment)")
	cmd.Flags().StringVar(&authUsername, "auth-username", "", "basic-auth username Nahook should send to this endpoint")
	cmd.Flags().StringVar(&authPassword, "auth-password", "", "basic-auth password Nahook should send to this endpoint")
	cmd.Flags().StringSliceVar(&metadataPairs, "metadata", nil, "metadata key=value; pass multiple times for multiple keys")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON instead of the human-readable key/value view")

	return cmd
}

// resolveEnvironment maps the --environment flag to a concrete env ID.
// Empty flag → workspace default. Non-empty → exact match by slug or ID.
func resolveEnvironment(cmd *cobra.Command, c *api.Client, requested string) (string, error) {
	if requested == "" {
		env, err := c.DefaultEnvironment(cmd.Context())
		if err != nil {
			return "", fmt.Errorf("could not resolve default environment: %w", err)
		}
		return env.ID, nil
	}
	env, err := c.FindEnvironment(cmd.Context(), requested)
	if err != nil {
		return "", err
	}
	return env.ID, nil
}

// parseMetadataPairs turns a slice of "key=value" entries into a map.
// Duplicate keys keep the last value, matching shell-variable semantics.
func parseMetadataPairs(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		i := indexOfEqual(p)
		if i < 0 {
			return nil, fmt.Errorf("metadata entry %q is missing '=' (expected key=value)", p)
		}
		out[p[:i]] = p[i+1:]
	}
	return out, nil
}

// indexOfEqual returns the index of the first '=' in s, or -1.
func indexOfEqual(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return i
		}
	}
	return -1
}
