package commands

import (
	"errors"
	"fmt"

	"github.com/getnahook/nahook-cli/internal/api"
	"github.com/getnahook/nahook-cli/internal/commands/session"
	"github.com/getnahook/nahook-cli/internal/config"
)

// ErrIngestionKeyMissing signals that neither NAHOOK_INGESTION_KEY nor
// the persisted ingestion_key is set. Surfaced as a non-zero exit so
// scripts can detect missing-credential vs. a real API failure.
var ErrIngestionKeyMissing = errors.New("ingestion API key not configured")

// requireIngestion loads the local session and resolves the ingestion
// key. The same `nhc_` CLI token from session.Require IS the one we
// authenticate the dashboard with; for trigger/send we need an extra
// `nhk_` API key from env or config because the ingestion service
// doesn't yet accept CLI tokens.
func requireIngestion() (*config.Config, *api.IngestionClient, error) {
	cfg, _, err := session.Require()
	if err != nil {
		return nil, nil, err
	}
	key := cfg.EffectiveIngestionKey()
	if key == "" {
		// Resolve the actual config path so the hint stays accurate when
		// NAHOOK_CONFIG_DIR points somewhere other than ~/.nahook. Fall
		// back to the literal default if path resolution itself fails —
		// the hint is still useful, just generic.
		configPath := "~/.nahook/config.toml"
		if p, perr := config.Path(); perr == nil {
			configPath = p
		}
		return nil, nil, fmt.Errorf(`%w

Set one of:
  export NAHOOK_INGESTION_KEY=nhk_us_xxx
or add this line to %s:
  ingestion_key = "nhk_us_xxx"

Get a key in the dashboard at:
  Settings → API Keys`, ErrIngestionKeyMissing, configPath)
	}
	return cfg, api.NewIngestionClient(key), nil
}
