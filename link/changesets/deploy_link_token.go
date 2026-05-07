// Package changesets provides reusable LINK token changesets.
package changesets

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/gagliardetto/solana-go"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"golang.org/x/sync/errgroup"

	solCommonUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	solTokenUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/tokens"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/generated/link_token_interface"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

var _ cldf.ChangeSet[[]uint64] = DeployLinkToken
var _ cldf.ChangeSet[[]uint64] = DeployStaticLinkToken
var _ cldf.ChangeSet[DeploySolanaLinkTokenConfig] = DeploySolanaLinkToken

// DeployLinkToken deploys a link token contract to the chain identified by the ChainSelector.
func DeployLinkToken(e cldf.Environment, chains []uint64) (cldf.ChangesetOutput, error) {
	if err := validateSelectorsInEnvironment(e, chains); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if err := validateNoDuplicateSelectors(chains); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if err := validateSelectorsFamily(chains, chainsel.FamilyEVM); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if err := validateNoExistingContract(e, chains, linkTokenTypeAndVersion()); err != nil {
		return cldf.ChangesetOutput{}, err
	}

	out := newLinkTokenOutput()
	deployGrp := errgroup.Group{}
	for _, chain := range chains {
		deployGrp.Go(func() error {
			deploy, err := deployLinkTokenContractEVM(
				e.Logger, e.BlockChains.EVMChains()[chain], out.AddressBook, //nolint:staticcheck // SA1019: legacy changeset still supports AddressBook output.
			)
			if err != nil {
				e.Logger.Errorw("Failed to deploy link token", "chain", chain, "err", err)

				return fmt.Errorf("failed to deploy link token for chain %d: %w", chain, err)
			}

			return saveAddressRef(out.DataStore, chain, deploy.Address.String(), linkTokenTypeAndVersion(), "")
		})
	}

	return out, deployGrp.Wait()
}

// DeployStaticLinkToken deploys a static link token contract to the chain identified by the ChainSelector.
func DeployStaticLinkToken(e cldf.Environment, chains []uint64) (cldf.ChangesetOutput, error) {
	if err := validateSelectorsInEnvironment(e, chains); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if err := validateNoDuplicateSelectors(chains); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if err := validateSelectorsFamily(chains, chainsel.FamilyEVM); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if err := validateNoExistingContract(e, chains, staticLinkTokenTypeAndVersion()); err != nil {
		return cldf.ChangesetOutput{}, err
	}

	out := newLinkTokenOutput()
	for _, chainSel := range chains {
		chain, ok := e.BlockChains.EVMChains()[chainSel]
		if !ok {
			return cldf.ChangesetOutput{}, fmt.Errorf("chain not found in environment: %d", chainSel)
		}
		deploy, err := cldf.DeployContract[*link_token_interface.LinkToken](e.Logger, chain, out.AddressBook, //nolint:staticcheck // SA1019: legacy changeset still supports AddressBook output.
			func(chain cldf_evm.Chain) cldf.ContractDeploy[*link_token_interface.LinkToken] {
				linkTokenAddr, tx, linkToken, err2 := link_token_interface.DeployLinkToken(
					chain.DeployerKey,
					chain.Client,
				)

				return cldf.ContractDeploy[*link_token_interface.LinkToken]{
					Address:  linkTokenAddr,
					Contract: linkToken,
					Tx:       tx,
					Tv:       staticLinkTokenTypeAndVersion(),
					Err:      err2,
				}
			})
		if err != nil {
			e.Logger.Errorw("Failed to deploy static link token", "chain", chain.String(), "err", err)
			return cldf.ChangesetOutput{}, err
		}
		if err := saveAddressRef(out.DataStore, chainSel, deploy.Address.String(), staticLinkTokenTypeAndVersion(), ""); err != nil {
			return cldf.ChangesetOutput{}, err
		}
	}

	return out, nil
}

func deployLinkTokenContractEVM(
	lggr logger.Logger,
	chain cldf_evm.Chain,
	ab cldf.AddressBook,
) (*cldf.ContractDeploy[*link_token.LinkToken], error) {
	linkToken, err := cldf.DeployContract[*link_token.LinkToken](lggr, chain, ab,
		func(chain cldf_evm.Chain) cldf.ContractDeploy[*link_token.LinkToken] {
			var (
				linkTokenAddr common.Address
				tx            *eth_types.Transaction
				linkToken     *link_token.LinkToken
				err2          error
			)
			if !chain.IsZkSyncVM {
				linkTokenAddr, tx, linkToken, err2 = link_token.DeployLinkToken(
					chain.DeployerKey,
					chain.Client,
				)
			} else {
				linkTokenAddr, _, linkToken, err2 = link_token.DeployLinkTokenZk(
					nil,
					chain.ClientZkSyncVM,
					chain.DeployerKeyZkSyncVM,
					chain.Client,
				)
			}

			return cldf.ContractDeploy[*link_token.LinkToken]{
				Address:  linkTokenAddr,
				Contract: linkToken,
				Tx:       tx,
				Tv:       linkTokenTypeAndVersion(),
				Err:      err2,
			}
		})
	if err != nil {
		lggr.Errorw("Failed to deploy link token", "chain", chain.String(), "err", err)

		return linkToken, err
	}

	return linkToken, nil
}

type DeploySolanaLinkTokenConfig struct {
	ChainSelector uint64
	TokenPrivKey  solana.PrivateKey
	TokenDecimals uint8
}

func DeploySolanaLinkToken(e cldf.Environment, cfg DeploySolanaLinkTokenConfig) (cldf.ChangesetOutput, error) {
	chain, ok := e.BlockChains.SolanaChains()[cfg.ChainSelector]
	if !ok {
		return cldf.ChangesetOutput{}, fmt.Errorf("chain not found in environment: %d", cfg.ChainSelector)
	}
	if err := validateNoExistingContract(e, []uint64{cfg.ChainSelector}, linkTokenTypeAndVersion()); err != nil {
		return cldf.ChangesetOutput{}, err
	}

	mint := cfg.TokenPrivKey
	instructions, err := solTokenUtil.CreateToken(
		context.Background(),
		solana.TokenProgramID,
		mint.PublicKey(),
		chain.DeployerKey.PublicKey(),
		cfg.TokenDecimals,
		chain.Client,
		cldf_solana.SolDefaultCommitment,
	)
	if err != nil {
		e.Logger.Errorw("Failed to generate instructions for link token deployment", "chain", chain.String(), "err", err)
		return cldf.ChangesetOutput{}, err
	}
	err = chain.Confirm(instructions, solCommonUtil.AddSigners(mint))
	if err != nil {
		e.Logger.Errorw("Failed to confirm instructions for link token deployment", "chain", chain.String(), "err", err)
		return cldf.ChangesetOutput{}, err
	}

	tv := linkTokenTypeAndVersion()
	e.Logger.Infow("Deployed contract", "Contract", tv.String(), "addr", mint.PublicKey().String(), "chain", chain.String())

	out := newLinkTokenOutput()
	if err := out.AddressBook.Save(chain.Selector, mint.PublicKey().String(), tv); err != nil { //nolint:staticcheck // SA1019: legacy changeset still supports AddressBook output.
		e.Logger.Errorw("Failed to save link token", "chain", chain.String(), "err", err)
		return cldf.ChangesetOutput{}, err
	}
	if err := saveAddressRef(out.DataStore, chain.Selector, mint.PublicKey().String(), tv, ""); err != nil {
		e.Logger.Errorw("Failed to save link token in datastore", "chain", chain.String(), "err", err)
		return cldf.ChangesetOutput{}, err
	}

	return out, nil
}

func newLinkTokenOutput() cldf.ChangesetOutput {
	return cldf.ChangesetOutput{
		AddressBook: cldf.NewMemoryAddressBook(),
		DataStore:   datastore.NewMemoryDataStore(),
	}
}

func linkTokenTypeAndVersion() cldf.TypeAndVersion {
	return cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_0_0)
}

func staticLinkTokenTypeAndVersion() cldf.TypeAndVersion {
	return cldf.NewTypeAndVersion(linkcontracts.StaticLinkToken, cldchangesetscommon.Version1_0_0)
}

func saveAddressRef(ds datastore.MutableDataStore, chainSelector uint64, address string, tv cldf.TypeAndVersion, qualifier string) error {
	return ds.Addresses().Add(datastore.AddressRef{
		ChainSelector: chainSelector,
		Address:       address,
		Type:          datastore.ContractType(tv.Type.String()),
		Version:       &tv.Version,
		Qualifier:     qualifier,
		Labels:        datastore.NewLabelSet(),
	})
}

func validateSelectorsInEnvironment(e cldf.Environment, chains []uint64) error {
	for _, chain := range chains {
		if !e.BlockChains.Exists(chain) {
			return fmt.Errorf("chain %d not found in environment", chain)
		}
	}

	return nil
}

func validateNoDuplicateSelectors(chains []uint64) error {
	seen := make(map[uint64]struct{}, len(chains))
	for _, chain := range chains {
		if _, ok := seen[chain]; ok {
			return fmt.Errorf("duplicate chain selector found: %d", chain)
		}
		seen[chain] = struct{}{}
	}

	return nil
}

func validateSelectorsFamily(chains []uint64, family string) error {
	for _, chain := range chains {
		selectorFamily, err := chainsel.GetSelectorFamily(chain)
		if err != nil {
			return fmt.Errorf("failed to get family for chain selector %d: %w", chain, err)
		}
		if selectorFamily != family {
			return fmt.Errorf("chain selector %d is not in the %s family", chain, family)
		}
	}

	return nil
}

func validateNoExistingContract(e cldf.Environment, chains []uint64, tv cldf.TypeAndVersion) error {
	if e.ExistingAddresses != nil { //nolint:staticcheck // SA1019: legacy changeset still supports AddressBook state.
		for _, chain := range chains {
			addresses, err := e.ExistingAddresses.AddressesForChain(chain) //nolint:staticcheck // SA1019: legacy changeset still supports AddressBook state.
			if err != nil {
				continue
			}
			for _, existingTV := range addresses {
				if sameTypeAndVersion(existingTV, tv) {
					return fmt.Errorf("%s contract already exists for chain selector %d in address book", tv.Type, chain)
				}
			}
		}
	}

	if e.DataStore == nil {
		return nil
	}
	for _, chain := range chains {
		refs := e.DataStore.Addresses().Filter(
			datastore.AddressRefByChainSelector(chain),
			datastore.AddressRefByType(datastore.ContractType(tv.Type.String())),
		)
		for _, ref := range refs {
			if ref.Version == nil || ref.Version.Equal(&tv.Version) {
				return fmt.Errorf("%s contract already exists for chain selector %d in datastore", tv.Type, chain)
			}
		}
	}

	return nil
}

func sameTypeAndVersion(a, b cldf.TypeAndVersion) bool {
	return a.Type == b.Type && a.Version.Equal(&b.Version)
}
