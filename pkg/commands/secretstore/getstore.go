package secretstore

import (
	"io"

	"github.com/fastly/cli/pkg/cmd"
	"github.com/fastly/cli/pkg/config"
	fsterr "github.com/fastly/cli/pkg/errors"
	"github.com/fastly/cli/pkg/manifest"
	"github.com/fastly/cli/pkg/text"
	"github.com/fastly/go-fastly/v7/fastly"
)

// NewGetStoreCommand returns a usable command registered under the parent.
func NewGetStoreCommand(parent cmd.Registerer, globals *config.Data, data manifest.Data) *GetStoreCommand {
	c := GetStoreCommand{
		Base: cmd.Base{
			Globals: globals,
		},
		manifest: data,
	}

	c.CmdClause = parent.Command("get", "Get secret store")

	// Required.
	c.RegisterFlag(storeIDFlag(&c.Input.ID)) // --store-id

	// Optional.
	c.RegisterFlagBool(c.jsonFlag()) // --json

	return &c
}

// GetStoreCommand calls the Fastly API to list the available secret stores.
type GetStoreCommand struct {
	cmd.Base
	jsonOutput

	Input    fastly.GetSecretStoreInput
	manifest manifest.Data
}

// Exec invokes the application logic for the command.
func (cmd *GetStoreCommand) Exec(_ io.Reader, out io.Writer) error {
	if cmd.Globals.Verbose() && cmd.jsonOutput.enabled {
		return fsterr.ErrInvalidVerboseJSONCombo
	}

	o, err := cmd.Globals.APIClient.GetSecretStore(&cmd.Input)
	if err != nil {
		cmd.Globals.ErrLog.Add(err)
		return err
	}

	if ok, err := cmd.WriteJSON(out, o); ok {
		return err
	}

	text.PrintSecretStore(out, "", o)

	return nil
}
