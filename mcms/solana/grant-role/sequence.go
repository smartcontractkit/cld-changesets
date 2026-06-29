package solgrantrole

import (
	"fmt"
	"strconv"

	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

var seqGrantRole = operations.NewSequence(
	"seq-solana-grant-role",
	&semvers.V1_0_0,
	"Grants RBACTimelock roles on Solana chains",
	runSolanaGrantRole,
)

func runSolanaGrantRole(
	b operations.Bundle,
	deps grantrole.Deps,
	in grantrole.SeqInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
	}

	env := grantrole.EnvFromDeps(deps)
	timelockAddress, err := timelockContractAddress(env, in)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	useMCMS := in.MCMS != nil
	var authorityAccount solanago.PublicKey
	if useMCMS {
		authorityAccount, err = timelockSignerPDA(env, in)
		if err != nil {
			return sequenceutils.OnChainOutput{}, err
		}
	}

	var batchOps []mcmstypes.BatchOperation
	if useMCMS {
		batchOps = make([]mcmstypes.BatchOperation, 0)
	}

	var grantees []solanago.PublicKey
	for _, grant := range in.Grants {
		grantees, err = AddressesNeedingGrant(
			b.GetContext(),
			mcmssolanasdk.NewTimelockInspector(chain.Client),
			timelockAddress,
			grant,
		)
		if err != nil {
			return sequenceutils.OnChainOutput{}, err
		}

		for _, grantee := range grantees {
			opInput := OpSolanaGrantRoleInput{
				Target: GrantRoleTarget{
					Timelock: timelockAddress,
					Role:     grant.Role,
					Address:  grantee.String(),
				},
				NoSend: useMCMS,
			}
			if useMCMS {
				opInput.AuthorityAccount = authorityAccount
			}

			var report operations.Report[OpSolanaGrantRoleInput, OpSolanaGrantRoleOutput]
			report, err = operations.ExecuteOperation(
				b,
				OpSolanaGrantRole,
				chain,
				opInput,
				operations.WithIdempotencyKey[OpSolanaGrantRoleInput, cldfsol.Chain](
					strconv.FormatUint(chain.Selector, 10)+":"+grant.Role.String()+":"+grantee.String(),
				),
			)
			if err != nil {
				return sequenceutils.OnChainOutput{}, err
			}

			if useMCMS {
				batchOps = append(batchOps, report.Output.BatchOperation)
			}
		}
	}

	return sequenceutils.OnChainOutput{BatchOps: batchOps}, nil
}

func timelockContractAddress(env cldf.Environment, in grantrole.SeqInput) (string, error) {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	if !ok {
		return "", fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
	}

	input := cldf.MCMSTimelockProposalInput{}
	if in.MCMS != nil {
		input = *in.MCMS
	}

	ref, err := reader.GetTimelockRef(env, in.ChainSelector, input)
	if err != nil {
		return "", fmt.Errorf("resolve timelock for chain %d: %w", in.ChainSelector, err)
	}

	return ref.Address, nil
}

func timelockSignerPDA(env cldf.Environment, in grantrole.SeqInput) (solanago.PublicKey, error) {
	timelockAddress, err := timelockContractAddress(env, in)
	if err != nil {
		return solanago.PublicKey{}, err
	}

	timelockProgram, timelockSeed, err := mcmssolanasdk.ParseContractAddress(timelockAddress)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("parse timelock ref address for chain %d: %w", in.ChainSelector, err)
	}

	var seed legacysolana.PDASeed
	copy(seed[:], timelockSeed[:])

	return familysolana.GetTimelockSignerPDA(timelockProgram, seed), nil
}

func accessControllerProgram(env cldf.Environment, chainSelector uint64) (solanago.PublicKey, error) {
	raw, err := programRef(env, chainSelector, mcmscontracts.AccessControllerProgram)
	if err != nil {
		return solanago.PublicKey{}, err
	}

	return solanago.PublicKeyFromBase58(raw)
}

func accessControllerAccount(env cldf.Environment, chainSelector uint64, role mcmssdk.TimelockRole) (solanago.PublicKey, error) {
	contractType, err := accessControllerContractType(role)
	if err != nil {
		return solanago.PublicKey{}, err
	}

	raw, err := accessControllerRef(env, chainSelector, contractType)
	if err != nil {
		return solanago.PublicKey{}, err
	}

	return solanago.PublicKeyFromBase58(raw)
}

func programRef(env cldf.Environment, chainSelector uint64, contractType cldf.ContractType) (string, error) {
	if env.DataStore == nil {
		return "", fmt.Errorf("datastore not available for chain %d", chainSelector)
	}

	ref, err := datastore.FindUniqueRef(env.DataStore.Addresses(), datastore.AddressRef{
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(contractType),
	})
	if err != nil {
		return "", fmt.Errorf("resolve %s for chain %d: %w", contractType, chainSelector, err)
	}

	return ref.Address, nil
}
