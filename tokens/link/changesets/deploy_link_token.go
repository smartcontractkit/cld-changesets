// Package changesets provides reusable LINK token changesets.
package changesets

import (
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	opsevm "github.com/smartcontractkit/cld-changesets/pkg/family/evm/operations"
	linkops "github.com/smartcontractkit/cld-changesets/tokens/link/operations"
)

var _ cldf.ChangeSetV2[DeployLinkTokenInput] = DeployLinkTokenChangeset{}

// EVMLinkVariant selects the EVM LINK token contract to deploy.
// The zero value deploys the burn/mint ERC677 variant (the default for all modern chains).
type EVMLinkVariant string

const (
	// EVMLinkBurnMint deploys a burn/mint ERC677 LINK token. This is the default.
	EVMLinkBurnMint EVMLinkVariant = ""
	// EVMLinkStatic deploys a non-burn/mint static LINK token for chains that do not
	// support the burn/mint interface.
	EVMLinkStatic EVMLinkVariant = "static"
)

// EVMLinkConfig holds per-chain configuration for EVM LINK token deployment.
type EVMLinkConfig struct {
	Variant   EVMLinkVariant `yaml:"variant,omitempty"   json:"variant,omitempty"`
	Qualifier string         `yaml:"qualifier,omitempty" json:"qualifier,omitempty"`
}

// SolanaLinkConfig holds per-chain configuration for Solana LINK token deployment.
type SolanaLinkConfig struct {
	TokenPrivKey  solana.PrivateKey `yaml:"tokenPrivKey"        json:"tokenPrivKey"`
	TokenDecimals uint8             `yaml:"tokenDecimals"       json:"tokenDecimals"`
	Qualifier     string            `yaml:"qualifier,omitempty" json:"qualifier,omitempty"`
}

// DeployLinkTokenInput specifies which chains to deploy LINK tokens to and how.
type DeployLinkTokenInput struct {
	EVM    map[uint64]EVMLinkConfig    `yaml:"evm,omitempty"    json:"evm,omitempty"`
	Solana map[uint64]SolanaLinkConfig `yaml:"solana,omitempty" json:"solana,omitempty"`
}

// DeployLinkTokenChangeset deploys LINK tokens across EVM and Solana chains.
type DeployLinkTokenChangeset struct{}

func (DeployLinkTokenChangeset) VerifyPreconditions(e cldf.Environment, input DeployLinkTokenInput) error {
	if len(input.EVM) == 0 && len(input.Solana) == 0 {
		return errors.New("no chains specified: at least one EVM or Solana chain is required")
	}

	for sel, cfg := range input.EVM {
		if !e.BlockChains.Exists(sel) {
			return fmt.Errorf("EVM chain %d not found in environment", sel)
		}

		if err := validateSelectorsFamily([]uint64{sel}, chainsel.FamilyEVM); err != nil {
			return err
		}

		if cfg.Variant != EVMLinkBurnMint && cfg.Variant != EVMLinkStatic {
			return fmt.Errorf("unknown EVM LINK variant %q for chain %d: must be %q or %q", cfg.Variant, sel, EVMLinkBurnMint, EVMLinkStatic)
		}

		tv := linkTokenTypeAndVersion()
		if cfg.Variant == EVMLinkStatic {
			tv = staticLinkTokenTypeAndVersion()
		}

		if err := validateNoExistingContract(e, []uint64{sel}, tv, cfg.Qualifier); err != nil {
			return err
		}
	}

	for sel, cfg := range input.Solana {
		if !e.BlockChains.Exists(sel) {
			return fmt.Errorf("solana chain %d not found in environment", sel)
		}

		if err := validateSelectorsFamily([]uint64{sel}, chainsel.FamilySolana); err != nil {
			return err
		}

		if len(cfg.TokenPrivKey) == 0 {
			return fmt.Errorf("solana chain %d: TokenPrivKey must be set", sel)
		}

		if err := validateNoExistingContract(e, []uint64{sel}, linkTokenTypeAndVersion(), cfg.Qualifier); err != nil {
			return err
		}
	}

	return nil
}

func (DeployLinkTokenChangeset) Apply(e cldf.Environment, input DeployLinkTokenInput) (cldf.ChangesetOutput, error) {
	ds := datastore.NewMemoryDataStore()
	allReports := make([]cldfops.Report[any, any], 0, len(input.EVM)+len(input.Solana))

	for sel, cfg := range input.EVM {
		chain, ok := e.BlockChains.EVMChains()[sel]
		if !ok {
			return cldf.ChangesetOutput{}, fmt.Errorf("EVM chain not found in environment: %d", sel)
		}

		op := linkops.OpEVMDeployLinkToken
		tv := linkTokenTypeAndVersion()
		if cfg.Variant == EVMLinkStatic {
			op = linkops.OpEVMDeployStaticLinkToken
			tv = staticLinkTokenTypeAndVersion()
		}

		qualifier := cfg.Qualifier
		report, err := cldfops.ExecuteOperation(
			e.OperationsBundle,
			op,
			chain,
			opsevm.EVMDeployInput[any]{ChainSelector: sel, Qualifier: &qualifier},
		)
		if err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("failed to deploy link token for chain %d: %w", sel, err)
		}

		addr := report.Output.Address.String()
		if err := saveAddressRef(ds, sel, addr, tv, cfg.Qualifier); err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("failed to save link token address for chain %d: %w", sel, err)
		}

		allReports = append(allReports, report.ToGenericReport())
		e.Logger.Infow("Deployed link token", "chain", sel, "addr", addr, "variant", tv.Type)
	}

	tv := linkTokenTypeAndVersion()
	for sel, cfg := range input.Solana {
		chain, ok := e.BlockChains.SolanaChains()[sel]
		if !ok {
			return cldf.ChangesetOutput{}, fmt.Errorf("solana chain not found in environment: %d", sel)
		}

		report, err := cldfops.ExecuteOperation(
			e.OperationsBundle,
			linkops.OpSolanaDeployLinkToken,
			chain,
			linkops.SolanaLinkDeployInput{
				MintKey:  cfg.TokenPrivKey,
				Decimals: cfg.TokenDecimals,
			},
		)
		if err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("failed to deploy Solana link token for chain %d: %w", sel, err)
		}

		addr := report.Output.Address
		if err := saveAddressRef(ds, sel, addr, tv, cfg.Qualifier); err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("failed to save Solana link token address for chain %d: %w", sel, err)
		}

		allReports = append(allReports, report.ToGenericReport())
		e.Logger.Infow("Deployed Solana link token", "chain", sel, "addr", addr)
	}

	return cldf.ChangesetOutput{DataStore: ds, Reports: allReports}, nil
}
