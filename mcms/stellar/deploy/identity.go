package stellardeploy

import (
	"context"
	"crypto/sha256"
	"fmt"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

const (
	deploymentSaltDomain             = "chainlink:stellar:mcms"
	maxDeploymentSaltAttempts uint32 = 1024
)

type ledgerEntryReader interface {
	GetLedgerEntries(ctx context.Context, request protocolrpc.GetLedgerEntriesRequest) (protocolrpc.GetLedgerEntriesResponse, error)
}

type deploymentIdentity struct {
	Salt       [32]byte
	ContractID string
	Attempt    uint32
	Existing   bool
}

type contractInstanceIdentity struct {
	Exists   bool
	IsWASM   bool
	WASMHash xdr.Hash
}

func deriveDeploymentSalt(contractType cldf.ContractType, qualifier string, attempt uint32) [32]byte {
	input := fmt.Sprintf(
		"%s:%s:%s:%s",
		deploymentSaltDomain,
		contractType,
		qualifier,
		semvers.V1_0_0.String(),
	)

	if attempt > 0 {
		input = fmt.Sprintf("%s:%d", input, attempt)
	}

	return sha256.Sum256([]byte(input))
}

func computeContractID(networkPassphrase string, deployerAddress string, salt [32]byte) (string, error) {
	if networkPassphrase == "" {
		return "", fmt.Errorf("compute Stellar contract ID: network passphrase is empty")
	}
	if deployerAddress == "" {
		return "", fmt.Errorf("compute Stellar contract ID: deployer address is empty")
	}

	accountID, err := xdr.AddressToAccountId(deployerAddress)
	if err != nil {
		return "", fmt.Errorf("compute Stellar contract ID: decode deployer address %q: %w", deployerAddress, err)
	}

	networkID := sha256.Sum256([]byte(networkPassphrase))
	preimage := xdr.HashIdPreimage{
		Type: xdr.EnvelopeTypeEnvelopeTypeContractId,
		ContractId: &xdr.HashIdPreimageContractId{
			NetworkId: xdr.Hash(networkID),
			ContractIdPreimage: xdr.ContractIdPreimage{
				Type: xdr.ContractIdPreimageTypeContractIdPreimageFromAddress,
				FromAddress: &xdr.ContractIdPreimageFromAddress{
					Address: xdr.ScAddress{
						Type:      xdr.ScAddressTypeScAddressTypeAccount,
						AccountId: &accountID,
					},
					Salt: xdr.Uint256(salt),
				},
			},
		},
	}

	encodedPreimage, err := preimage.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("compute Stellar contract ID: marshal contract ID preimage: %w", err)
	}

	contractHash := sha256.Sum256(encodedPreimage)
	contractID, err := strkey.Encode(strkey.VersionByteContract, contractHash[:])
	if err != nil {
		return "", fmt.Errorf("compute Stellar contract ID: encode contract strkey: %w", err)
	}

	return contractID, nil
}

func resolveDeploymentIdentity(
	ctx context.Context,
	reader ledgerEntryReader,
	networkPassphrase string,
	deployerAddress string,
	contractType cldf.ContractType,
	qualifier string,
	wasm []byte,
) (deploymentIdentity, error) {
	if reader == nil {
		return deploymentIdentity{}, fmt.Errorf("resolve Stellar deployment identity: ledger reader is nil")
	}
	if networkPassphrase == "" {
		return deploymentIdentity{}, fmt.Errorf("resolve Stellar deployment identity: network passphrase is empty")
	}
	if deployerAddress == "" {
		return deploymentIdentity{}, fmt.Errorf("resolve Stellar deployment identity: deployer address is empty")
	}
	if contractType == "" {
		return deploymentIdentity{}, fmt.Errorf("resolve Stellar deployment identity: contract type is empty")
	}
	if len(wasm) == 0 {
		return deploymentIdentity{}, fmt.Errorf("resolve Stellar deployment identity: WASM is empty")
	}

	expectedWASMHash := xdr.Hash(sha256.Sum256(wasm))

	for attempt := range maxDeploymentSaltAttempts {
		salt := deriveDeploymentSalt(contractType, qualifier, attempt)
		contractID, err := computeContractID(networkPassphrase, deployerAddress, salt)
		if err != nil {
			return deploymentIdentity{}, fmt.Errorf(
				"resolve Stellar deployment identity for %s attempt %d: %w",
				contractType,
				attempt,
				err,
			)
		}

		instance, err := readContractInstanceIdentity(ctx, reader, contractID)
		if err != nil {
			return deploymentIdentity{}, fmt.Errorf(
				"resolve Stellar deployment identity for %s at %s: %w",
				contractType,
				contractID,
				err,
			)
		}

		if !instance.Exists {
			return deploymentIdentity{
				Salt:       salt,
				ContractID: contractID,
				Attempt:    attempt,
			}, nil
		}

		if instance.IsWASM && instance.WASMHash == expectedWASMHash {
			return deploymentIdentity{
				Salt:       salt,
				ContractID: contractID,
				Attempt:    attempt,
				Existing:   true,
			}, nil
		}
	}

	return deploymentIdentity{}, fmt.Errorf(
		"resolve Stellar deployment identity for %s with qualifier %q: exhausted %d deterministic salt attempts",
		contractType,
		qualifier,
		maxDeploymentSaltAttempts,
	)
}

func readContractInstanceIdentity(
	ctx context.Context,
	reader ledgerEntryReader,
	contractID string,
) (contractInstanceIdentity, error) {
	contractAddress, err := contractScAddress(contractID)
	if err != nil {
		return contractInstanceIdentity{}, err
	}

	ledgerKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract: contractAddress,
			Key: xdr.ScVal{
				Type: xdr.ScValTypeScvLedgerKeyContractInstance,
			},
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}

	keyXDR, err := ledgerKey.MarshalBinaryBase64()
	if err != nil {
		return contractInstanceIdentity{}, fmt.Errorf(
			"marshal Stellar contract instance ledger key for %s: %w",
			contractID,
			err,
		)
	}

	response, err := reader.GetLedgerEntries(
		ctx,
		protocolrpc.GetLedgerEntriesRequest{Keys: []string{keyXDR}},
	)
	if err != nil {
		return contractInstanceIdentity{}, fmt.Errorf(
			"get Stellar contract instance ledger entry for %s: %w",
			contractID,
			err,
		)
	}

	if len(response.Entries) == 0 {
		return contractInstanceIdentity{}, nil
	}
	if len(response.Entries) != 1 {
		return contractInstanceIdentity{}, fmt.Errorf(
			"get Stellar contract instance ledger entry for %s: expected 1 entry, got %d",
			contractID,
			len(response.Entries),
		)
	}

	entryResult := response.Entries[0]
	if entryResult.DataXDR == "" {
		return contractInstanceIdentity{}, fmt.Errorf(
			"get Stellar contract instance ledger entry for %s: response has empty data XDR",
			contractID,
		)
	}

	var entry xdr.LedgerEntryData
	if err := xdr.SafeUnmarshalBase64(entryResult.DataXDR, &entry); err != nil {
		return contractInstanceIdentity{}, fmt.Errorf(
			"decode Stellar contract instance ledger entry for %s: %w",
			contractID,
			err,
		)
	}

	contractData, ok := entry.GetContractData()
	if !ok {
		return contractInstanceIdentity{}, fmt.Errorf("ledger entry for Stellar contract %s is not contract data", contractID)
	}

	returnedContractID, err := contractIDFromScAddress(contractData.Contract)
	if err != nil {
		return contractInstanceIdentity{}, fmt.Errorf("decode returned Stellar contract address for %s: %w", contractID, err)
	}
	if returnedContractID != contractID {
		return contractInstanceIdentity{}, fmt.Errorf("stellar contract instance query for %s returned data for %s", contractID, returnedContractID)
	}

	instance, ok := contractData.Val.GetInstance()
	if !ok {
		return contractInstanceIdentity{}, fmt.Errorf("contract data for Stellar contract %s is not a contract instance", contractID)
	}

	wasmHash, ok := instance.Executable.GetWasmHash()
	if !ok {
		return contractInstanceIdentity{
			Exists: true,
		}, nil
	}

	return contractInstanceIdentity{
		Exists:   true,
		IsWASM:   true,
		WASMHash: wasmHash,
	}, nil
}

func contractScAddress(contractID string) (xdr.ScAddress, error) {
	rawContractID, err := strkey.Decode(strkey.VersionByteContract, contractID)
	if err != nil {
		return xdr.ScAddress{}, fmt.Errorf("decode Stellar contract ID %q: %w", contractID, err)
	}
	if len(rawContractID) != 32 {
		return xdr.ScAddress{}, fmt.Errorf("decode Stellar contract ID %q: expected 32 bytes, got %d", contractID, len(rawContractID))
	}

	var id xdr.ContractId
	copy(id[:], rawContractID)

	return xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &id,
	}, nil
}

func contractIDFromScAddress(address xdr.ScAddress) (string, error) {
	if address.Type != xdr.ScAddressTypeScAddressTypeContract {
		return "", fmt.Errorf("Stellar address is not a contract address")
	}

	contractID, ok := address.GetContractId()
	if !ok {
		return "", fmt.Errorf("Stellar contract address has no contract ID")
	}

	encoded, err := strkey.Encode(strkey.VersionByteContract, contractID[:])
	if err != nil {
		return "", fmt.Errorf("encode Stellar contract ID: %w", err)
	}

	return encoded, nil
}
