package setup

import (
	"errors"
	"fmt"
	"io"

	"github.com/fastly/cli/pkg/api"
	fsterrors "github.com/fastly/cli/pkg/errors"
	"github.com/fastly/cli/pkg/manifest"
	"github.com/fastly/cli/pkg/text"
	"github.com/fastly/go-fastly/v7/fastly"
)

// SecretStores represents the service state related to secret stores defined
// within the fastly.toml [setup] configuration.
//
// NOTE: It implements the setup.Interface interface.
type SecretStores struct {
	// Public
	APIClient      api.Interface
	AcceptDefaults bool
	NonInteractive bool
	Progress       text.Progress
	ServiceID      string
	ServiceVersion int
	Setup          map[string]*manifest.SetupSecretStore
	Stdin          io.Reader
	Stdout         io.Writer

	// Private
	required []SecretStore
}

// SecretStore represents the configuration parameters for creating a
// secret store via the API client.
type SecretStore struct {
	Name  string
	Items []SecretStoreItem
}

// SecretStoreItem represents the configuration parameters for creating
// secret store items via the API client.
type SecretStoreItem struct {
	Name   string
	Secret string
}

// Predefined indicates if the service resource has been specified within the
// fastly.toml file using a [setup] configuration block.
func (s *SecretStores) Predefined() bool {
	return len(s.Setup) > 0
}

// Configure prompts the user for specific values related to the service resource.
func (s *SecretStores) Configure() error {
	for name, settings := range s.Setup {
		if !s.AcceptDefaults && !s.NonInteractive {
			text.Break(s.Stdout)
			text.Output(s.Stdout, "Configuring secret store '%s'", name)
			if settings.Description != "" {
				text.Output(s.Stdout, settings.Description)
			}
		}

		store := SecretStore{
			Name:  name,
			Items: make([]SecretStoreItem, 0, len(settings.Items)),
		}

		for key, item := range settings.Items {
			var (
				value string
				err   error
			)

			if !s.AcceptDefaults && !s.NonInteractive {
				text.Break(s.Stdout)
				text.Output(s.Stdout, "Create a secret store entry called '%s'", key)
				if item.Description != "" {
					text.Output(s.Stdout, item.Description)
				}
				text.Break(s.Stdout)

				prompt := text.BoldYellow("Value: ")
				value, err = text.InputSecure(s.Stdout, prompt, s.Stdin)
				if err != nil {
					return fmt.Errorf("error reading prompt input: %w", err)
				}
			}

			if value == "" {
				return errors.New("value cannot be blank")
			}

			store.Items = append(store.Items, SecretStoreItem{
				Name:   key,
				Secret: value,
			})
		}

		s.required = append(s.required, store)
	}

	return nil
}

// Create calls the relevant API to create the service resource(s).
func (s *SecretStores) Create() error {
	if s.Progress == nil {
		return fsterrors.RemediationError{
			Inner:       fmt.Errorf("internal logic error: no text.Progress configured for setup.SecretStores"),
			Remediation: fsterrors.BugRemediation,
		}
	}

	for _, secretStore := range s.required {
		s.Progress.Step(fmt.Sprintf("Creating secret store '%s'...", secretStore.Name))

		store, err := s.APIClient.CreateSecretStore(&fastly.CreateSecretStoreInput{
			Name: secretStore.Name,
		})
		if err != nil {
			s.Progress.Fail()
			return fmt.Errorf("error creating secret store: %w", err)
		}

		for _, item := range secretStore.Items {
			s.Progress.Step(fmt.Sprintf("Creating secret store entry '%s'...", item.Name))

			_, err = s.APIClient.CreateSecret(&fastly.CreateSecretInput{
				ID:     store.ID,
				Name:   item.Name,
				Secret: []byte(item.Secret),
			})
			if err != nil {
				s.Progress.Fail()
				return fmt.Errorf("error creating secret store entry: %w", err)
			}
		}

		s.Progress.Step(fmt.Sprintf("Creating resource link between service and secret store '%s'...", secretStore.Name))

		// IMPORTANT: We need to link the secret store to the C@E Service.
		_, err = s.APIClient.CreateResource(&fastly.CreateResourceInput{
			ServiceID:      s.ServiceID,
			ServiceVersion: s.ServiceVersion,
			Name:           fastly.String(store.Name),
			ResourceID:     fastly.String(store.ID),
		})
		if err != nil {
			s.Progress.Fail()
			return fmt.Errorf("error creating resource link between the service '%s' and the secret store '%s': %w", s.ServiceID, store.Name, err)
		}
	}

	return nil
}
