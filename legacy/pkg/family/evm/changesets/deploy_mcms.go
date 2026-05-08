package changesets

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/smartcontractkit/mcms/sdk"
	mcmsTypes "github.com/smartcontractkit/mcms/types"

	gethwrappers_zksync "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm/changesets/zksync"
	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

// TODO: Remove this function once the tests are implemented for the new sequence.
func DeployMCMSWithConfigEVM(
	contractType cldf.ContractType,
	lggr logger.Logger,
	chain cldf_evm.Chain,
	ab cldf.AddressBook,
	mcmConfig mcmsTypes.Config,
	options ...DeployMCMSOption,
) (*cldf.ContractDeploy[*bindings.ManyChainMultiSig], error) {
	groupQuorums, groupParents, signerAddresses, signerGroups, err := sdk.ExtractSetConfigInputs(&mcmConfig)
	if err != nil {
		lggr.Errorw("Failed to extract set config inputs", "chain", chain.String(), "err", err)
		return nil, err
	}
	mcm, err := cldf.DeployContract(lggr, chain, ab,
		func(chain cldf_evm.Chain) cldf.ContractDeploy[*bindings.ManyChainMultiSig] {
			var (
				mcmAddr common.Address
				tx      *types.Transaction
				mcm     *bindings.ManyChainMultiSig
				err2    error
			)
			if chain.IsZkSyncVM {
				mcmAddr, _, mcm, err2 = gethwrappers_zksync.DeployManyChainMultiSigZk(
					nil,
					chain.ClientZkSyncVM,
					chain.DeployerKeyZkSyncVM,
					chain.Client,
				)
			} else {
				mcmAddr, tx, mcm, err2 = bindings.DeployManyChainMultiSig(
					chain.DeployerKey,
					chain.Client,
				)
			}

			tv := cldf.NewTypeAndVersion(contractType, cldchangesetscommon.Version1_0_0)
			for _, option := range options {
				option(&tv)
			}

			return cldf.ContractDeploy[*bindings.ManyChainMultiSig]{
				Address: mcmAddr, Contract: mcm, Tx: tx, Tv: tv, Err: err2,
			}
		})
	if err != nil {
		lggr.Errorw("Failed to deploy mcm", "chain", chain.String(), "err", err)
		return mcm, err
	}
	mcmsTx, err := mcm.Contract.SetConfig(chain.DeployerKey,
		signerAddresses,
		// Signer 1 is int group 0 (root group) with quorum 1.
		signerGroups,
		groupQuorums,
		groupParents,
		false,
	)
	if _, confirmErr := cldf.ConfirmIfNoError(chain, mcmsTx, err); confirmErr != nil {
		lggr.Errorw("Failed to confirm mcm config", "chain", chain.String(), "err", confirmErr)
		return mcm, confirmErr
	}

	return mcm, nil
}
