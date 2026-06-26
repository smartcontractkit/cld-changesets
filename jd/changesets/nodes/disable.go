package nodes

import (
	"errors"
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

var _ cldf.ChangeSetV2[DisableNodesInput] = DisableNodesChangeset{}

// DisableNodesInput is the serializable input of DisableNodesChangeset.
type DisableNodesInput = jdops.DisableNodesInput

// DisableNodesChangeset disables nodes by CSA key.
type DisableNodesChangeset struct{}

func (DisableNodesChangeset) VerifyPreconditions(_ cldf.Environment, cfg DisableNodesInput) error {
	if len(cfg.CSAKeys) == 0 {
		return errors.New("no csa_keys provided")
	}
	seen := make(map[string]struct{}, len(cfg.CSAKeys))
	for _, k := range cfg.CSAKeys {
		if k == "" {
			return errors.New("csa_key cannot be empty")
		}
		if _, ok := seen[k]; ok {
			return fmt.Errorf("duplicate csa_key %q", k)
		}
		seen[k] = struct{}{}
	}

	return nil
}

func (DisableNodesChangeset) Apply(e cldf.Environment, input DisableNodesInput) (cldf.ChangesetOutput, error) {
	deps := jdops.JDOpDeps{Offchain: e.Offchain, EnvName: e.Name}
	_, err := fwops.ExecuteSequence(e.OperationsBundle, jdops.SeqJDDisableNodes, deps, input)

	return cldf.ChangesetOutput{}, err
}
