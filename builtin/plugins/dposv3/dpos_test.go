package dposv3

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	loom "github.com/loomnetwork/go-loom"
	common "github.com/loomnetwork/go-loom/common"
	"github.com/loomnetwork/go-loom/plugin"
	"github.com/loomnetwork/go-loom/plugin/contractpb"
	types "github.com/loomnetwork/go-loom/types"
	"github.com/loomnetwork/loomchain"
	"github.com/loomnetwork/loomchain/builtin/plugins/coin"

	dtypes "github.com/loomnetwork/go-loom/builtin/types/dposv3"
)

var (
	validatorPubKeyHex1 = "3866f776276246e4f9998aa90632931d89b0d3a5930e804e02299533f55b39e1"
	validatorPubKeyHex2 = "7796b813617b283f81ea1747fbddbe73fe4b5fce0eac0728e47de51d8e506701"
	validatorPubKeyHex3 = "e4008e26428a9bca87465e8de3a8d0e9c37a56ca619d3d6202b0567528786618"
	validatorPubKeyHex4 = "21908210428a9bca87465e8de3a8d0e9c37a56ca619d3d6202b0567528786618"

	delegatorAddress1       = loom.MustParseAddress("chain:0xb16a379ec18d4093666f8f38b11a3071c920207d")
	delegatorAddress2       = loom.MustParseAddress("chain:0xfa4c7920accfd66b86f5fd0e69682a79f762d49e")
	delegatorAddress3       = loom.MustParseAddress("chain:0x5cecd1f7261e1f4c684e297be3edf03b825e01c4")
	delegatorAddress4       = loom.MustParseAddress("chain:0x000000000000000000000000e3edf03b825e01e0")
	delegatorAddress5       = loom.MustParseAddress("chain:0x020000000000000000000000e3edf03b825e0288")
	delegatorAddress6       = loom.MustParseAddress("chain:0x000000000000000000040400e3edf03b825e0398")
	chainID                 = "default"
	startTime         int64 = 100000

	pubKey1, _ = hex.DecodeString(validatorPubKeyHex1)
	addr1      = loom.Address{
		ChainID: chainID,
		Local:   loom.LocalAddressFromPublicKey(pubKey1),
	}
	pubKey2, _ = hex.DecodeString(validatorPubKeyHex2)
	addr2      = loom.Address{
		ChainID: chainID,
		Local:   loom.LocalAddressFromPublicKey(pubKey2),
	}
	pubKey3, _ = hex.DecodeString(validatorPubKeyHex3)
	addr3      = loom.Address{
		ChainID: chainID,
		Local:   loom.LocalAddressFromPublicKey(pubKey3),
	}
	pubKey4, _ = hex.DecodeString(validatorPubKeyHex4)
	addr4      = loom.Address{
		ChainID: chainID,
		Local:   loom.LocalAddressFromPublicKey(pubKey4),
	}
)

func TestRegisterWhitelistedCandidate(t *testing.T) {
	oraclePubKey, _ := hex.DecodeString(validatorPubKeyHex2)
	oracleAddr := loom.Address{
		Local: loom.LocalAddressFromPublicKey(oraclePubKey),
	}

	pubKey, _ := hex.DecodeString(validatorPubKeyHex1)
	addr := loom.Address{
		Local: loom.LocalAddressFromPublicKey(pubKey),
	}
	pctx := plugin.CreateFakeContext(addr, addr)

	coinContract := &coin.Coin{}
	coinAddr := pctx.CreateContract(coin.Contract)
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(addr2, 2000000000000000000),
		},
	})

	dpos, err := deployDPOSContract(pctx, 21, nil, &coinAddr, nil, nil, nil, nil, &oracleAddr)
	require.Nil(t, err)
	dposAddr := dpos.Address

	whitelistAmount := big.NewInt(1000000000000)
	err = dpos.WhitelistCandidate(pctx.WithSender(oracleAddr), addr, whitelistAmount, 0)
	require.Nil(t, err)

	err = dpos.RegisterCandidate(pctx.WithSender(addr), pubKey, nil, nil, nil, nil)
	require.Nil(t, err)

	err = dpos.UnregisterCandidate(pctx.WithSender(addr))
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dposAddr))
	require.Nil(t, err)

	registrationFee := &types.BigUInt{Value: *scientificNotation(defaultRegistrationRequirement, tokenDecimals)}
	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dpos.RegisterCandidate(pctx.WithSender(addr2), pubKey2, nil, nil, nil, nil)
	require.Nil(t, err)

	err = dpos.RegisterCandidate(pctx.WithSender(addr), pubKey, nil, nil, nil, nil)
	require.Nil(t, err)

	err = dpos.RemoveWhitelistedCandidate(pctx.WithSender(oracleAddr), &addr)
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(pctx)
	require.Nil(t, err)
	assert.Equal(t, 2, len(candidates))

	err = dpos.UnregisterCandidate(pctx.WithSender(addr))
	require.Nil(t, err)

	candidates, err = dpos.ListCandidates(pctx)
	require.Nil(t, err)
	assert.Equal(t, 2, len(candidates))

	require.NoError(t, elect(pctx, dposAddr))

	candidates, err = dpos.ListCandidates(pctx)
	require.Nil(t, err)
	assert.Equal(t, 1, len(candidates))

	err = dpos.RegisterCandidate(pctx.WithSender(addr), pubKey, nil, nil, nil, nil)
	require.NotNil(t, err)
}

func TestChangeFee(t *testing.T) {
	oldFee := uint64(100)
	newFee := uint64(1000)
	oraclePubKey, _ := hex.DecodeString(validatorPubKeyHex2)
	oracleAddr := loom.Address{
		Local: loom.LocalAddressFromPublicKey(oraclePubKey),
	}

	pubKey, _ := hex.DecodeString(validatorPubKeyHex1)
	addr := loom.Address{
		Local: loom.LocalAddressFromPublicKey(pubKey),
	}
	pctx := plugin.CreateFakeContext(addr, addr)

	// Deploy the coin contract (DPOS Init() will attempt to resolve it)
	coinContract := &coin.Coin{}
	_ = pctx.CreateContract(contractpb.MakePluginContract(coinContract))

	dpos, err := deployDPOSContract(pctx, 21, nil, nil, nil, nil, nil, nil, &oracleAddr)
	require.Nil(t, err)

	amount := big.NewInt(1000000000000)
	err = dpos.WhitelistCandidate(pctx.WithSender(oracleAddr), addr, amount, 0)
	require.Nil(t, err)

	err = dpos.RegisterCandidate(pctx.WithSender(addr), pubKey, &oldFee, nil, nil, nil)
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(pctx)
	require.Nil(t, err)
	assert.Equal(t, oldFee, candidates[0].Candidate.Fee)
	assert.Equal(t, oldFee, candidates[0].Candidate.NewFee)

	require.NoError(t, elect(pctx, dpos.Address))

	require.NoError(t, elect(pctx, dpos.Address))

	candidates, err = dpos.ListCandidates(pctx)
	require.Nil(t, err)
	assert.Equal(t, oldFee, candidates[0].Candidate.Fee)
	assert.Equal(t, oldFee, candidates[0].Candidate.NewFee)

	err = dpos.ChangeFee(pctx.WithSender(addr), newFee)
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	candidates, err = dpos.ListCandidates(pctx)
	require.Nil(t, err)
	// Fee should not reset after only a single election
	assert.Equal(t, oldFee, candidates[0].Candidate.Fee)
	assert.Equal(t, newFee, candidates[0].Candidate.NewFee)

	require.NoError(t, elect(pctx, dpos.Address))

	candidates, err = dpos.ListCandidates(pctx)
	require.Nil(t, err)
	// Fee should reset after two elections
	assert.Equal(t, newFee, candidates[0].Candidate.Fee)
	assert.Equal(t, newFee, candidates[0].Candidate.NewFee)
}

func TestDelegate(t *testing.T) {
	pctx := plugin.CreateFakeContext(addr1, addr1)

	oraclePubKey, _ := hex.DecodeString(validatorPubKeyHex2)
	oracleAddr := loom.Address{
		Local: loom.LocalAddressFromPublicKey(oraclePubKey),
	}

	// Deploy the coin contract (DPOS Init() will attempt to resolve it)
	coinContract := &coin.Coin{}
	coinAddr := pctx.CreateContract(coin.Contract)
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 1000000000000000000),
			makeAccount(delegatorAddress2, 2000000000000000000),
			makeAccount(delegatorAddress3, 1000000000000000000),
			makeAccount(addr1, 1000000000000000000),
		},
	})

	dpos, err := deployDPOSContract(pctx, 21, nil, nil, nil, nil, nil, nil, &oracleAddr)
	require.Nil(t, err)

	whitelistAmount := big.NewInt(1000000000000)
	// should fail from non-oracle
	err = dpos.WhitelistCandidate(pctx.WithSender(addr1), addr1, whitelistAmount, 0)
	require.Error(t, err)

	err = dpos.WhitelistCandidate(pctx.WithSender(oracleAddr), addr1, whitelistAmount, 0)
	require.Nil(t, err)

	err = dpos.RegisterCandidate(pctx.WithSender(addr1), pubKey1, nil, nil, nil, nil)
	require.Nil(t, err)

	delegationAmount := big.NewInt(100)
	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
		Spender: dpos.Address.MarshalPB(),
		Amount:  &types.BigUInt{Value: *loom.NewBigUInt(delegationAmount)},
	})
	require.Nil(t, err)

	response, err := coinContract.Allowance(contractpb.WrapPluginContext(coinCtx.WithSender(oracleAddr)), &coin.AllowanceRequest{
		Owner:   addr1.MarshalPB(),
		Spender: dpos.Address.MarshalPB(),
	})
	require.Nil(t, err)
	require.True(t, delegationAmount.Cmp(response.Amount.Value.Int) == 0)

	candidates, err := dpos.ListCandidates(pctx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 1)

	err = dpos.Delegate(pctx.WithSender(addr1), &addr1, delegationAmount, nil, nil)
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
		Spender: dpos.Address.MarshalPB(),
		Amount:  &types.BigUInt{Value: *loom.NewBigUInt(delegationAmount)},
	})
	require.Nil(t, err)

	// total rewards distribution should equal 0 before elections run
	totalRewardDistribution, err := dpos.CheckRewards(pctx.WithSender(addr1))
	require.Nil(t, err)
	assert.True(t, totalRewardDistribution.Cmp(common.BigZero()) == 0)

	require.NoError(t, elect(pctx, dpos.Address))

	// total rewards distribution should equal still be zero after first election
	totalRewardDistribution, err = dpos.CheckRewards(pctx.WithSender(addr1))
	require.Nil(t, err)
	assert.True(t, totalRewardDistribution.Cmp(common.BigZero()) == 0)

	err = dpos.Delegate(pctx.WithSender(addr1), &addr1, delegationAmount, nil, nil)
	require.Nil(t, err)

	_, delegatedAmount, _, err := dpos.CheckDelegation(pctx, &addr1, &addr2)
	require.Nil(t, err)
	assert.True(t, delegatedAmount.Cmp(common.BigZero()) == 0)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.ApproveRequest{
		Spender: dpos.Address.MarshalPB(),
		Amount:  &types.BigUInt{Value: *loom.NewBigUInt(delegationAmount)},
	})
	require.Nil(t, err)

	err = dpos.Delegate(pctx.WithSender(delegatorAddress1), &addr1, delegationAmount, nil, nil)
	require.Nil(t, err)

	// checking a non-existent delegation should result in an empty (amount = 0)
	// delegaiton being returned
	_, delegatedAmount, _, err = dpos.CheckDelegation(pctx, &addr1, &addr2)
	require.Nil(t, err)
	assert.True(t, delegatedAmount.Cmp(common.BigZero()) == 0)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
		Spender: dpos.Address.MarshalPB(),
		Amount:  &types.BigUInt{Value: *loom.NewBigUInt(delegationAmount)},
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	// total rewards distribution should be greater than zero
	totalRewardDistribution, err = dpos.CheckRewards(pctx.WithSender(addr1))
	require.Nil(t, err)
	assert.True(t, common.IsPositive(*totalRewardDistribution))

	// advancing contract time beyond the delegator1-addr1 lock period
	now := uint64(pctx.Now().Unix())
	pctx.SetTime(pctx.Now().Add(time.Duration(now+TierLocktimeMap[0]) * time.Second))

	err = dpos.Unbond(pctx.WithSender(addr1), &addr1, delegationAmount, 1)
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	err = dpos.Unbond(pctx.WithSender(addr1), &addr1, delegationAmount, 2)
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	err = dpos.Unbond(pctx.WithSender(addr1), &addr1, big.NewInt(1), 3)
	require.Error(t, err)

	// testing delegations to limbo validator
	err = dpos.Redelegate(pctx.WithSender(delegatorAddress1), &addr1, &limboValidatorAddress, delegationAmount, 1, nil, nil)
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	_, delegatedAmount, _, err = dpos.CheckDelegation(pctx, &addr1, &delegatorAddress1)
	require.Nil(t, err)
	assert.True(t, delegatedAmount.Cmp(common.BigZero()) == 0)

	_, delegatedAmount, _, err = dpos.CheckDelegation(pctx, &limboValidatorAddress, &delegatorAddress1)
	require.Nil(t, err)
	assert.True(t, delegatedAmount.Int.Cmp(delegationAmount) == 0)
}

func TestRedelegate(t *testing.T) {
	pctx := plugin.CreateFakeContext(addr1, addr1)

	// Deploy the coin contract (DPOS Init() will attempt to resolve it)
	coinContract := &coin.Coin{}
	coinAddr := pctx.CreateContract(coin.Contract)
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 1000000000000000000),
			makeAccount(delegatorAddress2, 2000000000000000000),
			makeAccount(delegatorAddress3, 1000000000000000000),
			makeAccount(addr1, 1000000000000000000),
			makeAccount(addr2, 1000000000000000000),
			makeAccount(addr3, 1000000000000000000),
		},
	})

	registrationFee := loom.BigZeroPB()

	dpos, err := deployDPOSContract(pctx, 21, nil, nil, nil, &registrationFee.Value, nil, nil, nil)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	// Registering 3 candidates
	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &RegisterCandidateRequest{
		PubKey: pubKey2,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &RegisterCandidateRequest{
		PubKey: pubKey3,
	})
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 3)

	err = Elect(contractpb.WrapPluginContext(dposCtx))
	require.Nil(t, err)

	// Verifying that with registration fee = 0, none of the 3 registered candidates are elected validators
	validators, err := dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 0)

	delegationAmount := loom.NewBigUIntFromInt(10000000)
	smallDelegationAmount := loom.NewBigUIntFromInt(1000000)
	partialSplitAmount := loom.NewBigUIntFromInt(900000)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	// Verifying that addr1 was elected sole validator
	validators, err = dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 1)
	assert.True(t, validators[0].Address.Local.Compare(addr1.Local) == 0)

	// checking that redelegation fails with 0 amount
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &RedelegateRequest{
		FormerValidatorAddress: addr1.MarshalPB(),
		ValidatorAddress:       addr2.MarshalPB(),
		Amount:                 loom.BigZeroPB(),
		Index:                  1,
	})
	require.NotNil(t, err)

	// redelegating sole delegation to validator addr2
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &RedelegateRequest{
		FormerValidatorAddress: addr1.MarshalPB(),
		ValidatorAddress:       addr2.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *delegationAmount},
		Index:                  1,
	})
	require.Nil(t, err)

	// Redelegation takes effect within a single election period
	require.NoError(t, elect(pctx, dpos.Address))

	// Verifying that addr2 was elected sole validator
	validators, err = dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 1)
	assert.True(t, validators[0].Address.Local.Compare(addr2.Local) == 0)

	// redelegating sole delegation to validator addr3
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &RedelegateRequest{
		FormerValidatorAddress: addr2.MarshalPB(),
		ValidatorAddress:       addr3.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *delegationAmount},
		Index:                  1,
	})
	require.Nil(t, err)

	// Redelegation takes effect within a single election period
	require.NoError(t, elect(pctx, dpos.Address))

	// Verifying that addr3 was elected sole validator
	validators, err = dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 1)
	assert.True(t, validators[0].Address.Local.Compare(addr3.Local) == 0)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	// adding 2nd delegation from 2nd delegator in order to elect a second validator
	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	// checking that the 2nd validator (addr1) was elected in addition to add3
	validators, err = dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 2)

	// delegator 1 removes delegation to limbo
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &RedelegateRequest{
		FormerValidatorAddress: addr3.MarshalPB(),
		ValidatorAddress:       limboValidatorAddress.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *delegationAmount},
		Index:                  1,
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	// Verifying that addr1 was elected sole validator AFTER delegator1 redelegated to limbo validator
	validators, err = dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 1)
	assert.True(t, validators[0].Address.Local.Compare(addr1.Local) == 0)

	// Checking that redelegaiton of a negative amount is rejected
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &RedelegateRequest{
		FormerValidatorAddress: addr1.MarshalPB(),
		ValidatorAddress:       addr2.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *loom.NewBigUIntFromInt(-1000)},
	})
	require.NotNil(t, err)

	// Checking that redelegaiton of an amount greater than the total delegation is rejected
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &RedelegateRequest{
		FormerValidatorAddress: addr1.MarshalPB(),
		ValidatorAddress:       addr2.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *loom.NewBigUIntFromInt(100000000)},
	})
	require.NotNil(t, err)

	// splitting delegator2's delegation to 2nd validator
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &RedelegateRequest{
		FormerValidatorAddress: addr1.MarshalPB(),
		ValidatorAddress:       addr2.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *smallDelegationAmount},
		Index:                  1,
	})
	require.Nil(t, err)

	// partially splitting delegator2's delegation to 3rd validator
	// this also tests that redelegate is able to set a new tier
	err = dposContract.Redelegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &RedelegateRequest{
		FormerValidatorAddress: addr1.MarshalPB(),
		ValidatorAddress:       addr3.MarshalPB(),
		Amount:                 &types.BigUInt{Value: *partialSplitAmount},
		NewLocktimeTier:        3,
		Index:                  1,
	})
	require.Nil(t, err)

	balanceBefore, err := coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr1.MarshalPB(),
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	balanceAfter, err := coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr1.MarshalPB(),
	})
	require.Nil(t, err)

	require.True(t, balanceBefore.Balance.Value.Cmp(&balanceAfter.Balance.Value) == 0)

	require.NoError(t, elect(pctx, dpos.Address))

	delegationResponse, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: addr3.MarshalPB(),
		DelegatorAddress: delegatorAddress2.MarshalPB(),
	})
	require.Nil(t, err)
	// assert.True(t, delegationResponse.Amount.Value.Cmp(smallDelegationAmount) == 0)
	assert.Equal(t, delegationResponse.Delegations[len(delegationResponse.Delegations)-1].LocktimeTier, TIER_THREE)

	// checking that all 3 candidates have been elected validators
	validators, err = dpos.ListValidators(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(validators), 3)
}

func TestReward(t *testing.T) {
	// set elect time in params to one second for easy calculations
	delegationAmount := loom.BigUInt{big.NewInt(10000000000000)}
	cycleLengthSeconds := int64(100)
	params := Params{
		ElectionCycleLength: cycleLengthSeconds,
		MaxYearlyReward:     &types.BigUInt{Value: *scientificNotation(defaultMaxYearlyReward, tokenDecimals)},
	}
	statistic := ValidatorStatistic{
		DelegationTotal: &types.BigUInt{Value: delegationAmount},
	}

	rewardTotal := common.BigZero()
	for i := int64(0); i < yearSeconds; i = i + cycleLengthSeconds {
		cycleReward := calculateRewards(statistic.DelegationTotal.Value, &params, *common.BigZero())
		rewardTotal.Add(rewardTotal, &cycleReward)
	}

	// checking that distribution is roughtly equal to 5% of delegation after one year
	assert.Equal(t, rewardTotal.Cmp(&loom.BigUInt{big.NewInt(490000000000)}), 1)
	assert.Equal(t, rewardTotal.Cmp(&loom.BigUInt{big.NewInt(510000000000)}), -1)
}

func TestElectWhitelists(t *testing.T) {
	// Init the coin balances
	pctx := plugin.CreateFakeContext(delegatorAddress1, loom.Address{}).WithBlock(loom.BlockHeader{
		ChainID: chainID,
		Time:    startTime,
	})
	coinAddr := pctx.CreateContract(coin.Contract)

	coinContract := &coin.Coin{}
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 1e18),
			makeAccount(delegatorAddress2, 20),
			makeAccount(delegatorAddress3, 10),
		},
	})
	// Enable the feature flag and check that the whitelist rules get applied corectly
	cycleLengthSeconds := int64(100)
	maxYearlyReward := scientificNotation(defaultMaxYearlyReward, tokenDecimals)
	// Init the dpos contract
	dpos, err := deployDPOSContract(pctx, 5, &cycleLengthSeconds, &coinAddr, maxYearlyReward, nil, nil, nil, &addr1)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	dposCtx.SetFeature(loomchain.DPOSVersion2_1, true)
	require.True(t, dposCtx.FeatureEnabled(loomchain.DPOSVersion2_1, false))

	// transfer coins to reward fund
	amount := big.NewInt(10000000)
	amount.Mul(amount, big.NewInt(1e18))
	err = coinContract.Transfer(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.TransferRequest{
		To:     dposAddr.MarshalPB(),
		Amount: &types.BigUInt{Value: loom.BigUInt{amount}},
	})
	require.Nil(t, err)

	whitelistAmount := loom.BigUInt{big.NewInt(1000000000000)}

	// Whitelist with locktime tier 0, which should use 5% of rewards
	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr1.MarshalPB(),
					Amount:           &types.BigUInt{Value: whitelistAmount},
					LocktimeTier:     0,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 1,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	// Whitelist with locktime tier 1, which should use 7.5% of rewards
	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr2.MarshalPB(),
					Amount:           &types.BigUInt{Value: whitelistAmount},
					LocktimeTier:     1,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 2,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	// Whitelist with locktime tier 2, which should use 10% of rewards
	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr3.MarshalPB(),
					Amount:           &types.BigUInt{Value: whitelistAmount},
					LocktimeTier:     2,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 3,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	// Whitelist with locktime tier 3, which should use 20% of rewards
	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr4.MarshalPB(),
					Amount:           &types.BigUInt{Value: whitelistAmount},
					LocktimeTier:     3,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 4,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	// Register the 5 validators

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &RegisterCandidateRequest{
		PubKey: pubKey2,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &RegisterCandidateRequest{
		PubKey: pubKey3,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr4)), &RegisterCandidateRequest{
		PubKey: pubKey4,
	})
	require.Nil(t, err)

	// Check that they were registered properly
	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 4)

	listValidatorsResponse, err := dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)

	// Elect them
	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 4)

	// Do a bunch of elections that correspond to 1/100th of a year
	for i := int64(0); i < yearSeconds/100; i = i + cycleLengthSeconds {
		require.NoError(t, elect(pctx, dpos.Address))
		dposCtx.SetTime(dposCtx.Now().Add(time.Duration(cycleLengthSeconds) * time.Second))
	}

	checkResponse, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckRewardDelegationRequest{ValidatorAddress: addr1.MarshalPB()})
	require.Nil(t, err)
	// checking that rewards are roughtly equal to 0.5% of delegation after one year
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(490000000)}), 1)
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(510000000)}), -1)

	checkResponse, err = dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &CheckRewardDelegationRequest{ValidatorAddress: addr2.MarshalPB()})
	require.Nil(t, err)
	// checking that rewards are roughtly equal to 0.75% of delegation after one year
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(740000000)}), 1)
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(760000000)}), -1)

	checkResponse, err = dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &CheckRewardDelegationRequest{ValidatorAddress: addr3.MarshalPB()})
	require.Nil(t, err)
	// checking that rewards are roughtly equal to 1% of delegation after one year
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(990000000)}), 1)
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(1000000000)}), -1)

	checkResponse, err = dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr4)), &CheckRewardDelegationRequest{ValidatorAddress: addr4.MarshalPB()})
	require.Nil(t, err)
	// checking that rewards are roughtly equal to 2% of delegation after one year
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(1990000000)}), 1)
	assert.Equal(t, checkResponse.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(2000000000)}), -1)

	// Let's withdraw rewards and see how the balances change.

	err = dposContract.Unbond(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &UnbondRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           loom.BigZeroPB(),
		Index:            REWARD_DELEGATION_INDEX,
	})

	require.Nil(t, err)
	err = dposContract.Unbond(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &UnbondRequest{
		ValidatorAddress: addr2.MarshalPB(),
		Amount:           loom.BigZeroPB(),
		Index:            REWARD_DELEGATION_INDEX,
	})
	require.Nil(t, err)

	err = dposContract.Unbond(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &UnbondRequest{
		ValidatorAddress: addr3.MarshalPB(),
		Amount:           loom.BigZeroPB(),
		Index:            REWARD_DELEGATION_INDEX,
	})

	require.Nil(t, err)
	err = dposContract.Unbond(contractpb.WrapPluginContext(dposCtx.WithSender(addr4)), &UnbondRequest{
		ValidatorAddress: addr4.MarshalPB(),
		Amount:           loom.BigZeroPB(),
		Index:            REWARD_DELEGATION_INDEX,
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	balanceAfterClaim, err := coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(490000000)}), 1)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(510000000)}), -1)

	balanceAfterClaim, err = coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr2.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(740000000)}), 1)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(760000000)}), -1)

	balanceAfterClaim, err = coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr3.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(990000000)}), 1)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(1000000000)}), -1)

	balanceAfterClaim, err = coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr4.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(1990000000)}), 1)
	assert.Equal(t, balanceAfterClaim.Balance.Value.Cmp(&loom.BigUInt{big.NewInt(2000000000)}), -1)

}

func TestElect(t *testing.T) {
	pctx := plugin.CreateFakeContext(delegatorAddress1, loom.Address{}).WithBlock(loom.BlockHeader{
		ChainID: chainID,
		Time:    startTime,
	})
	coinAddr := pctx.CreateContract(coin.Contract)

	coinContract := &coin.Coin{}
	coinCtx := pctx.WithAddress(coinAddr)
	// Initialize the coin balances
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 130),
			makeAccount(delegatorAddress2, 20),
			makeAccount(delegatorAddress3, 10),
		},
	})

	// create dpos contract
	dpos, err := deployDPOSContract(pctx, 2, nil, &coinAddr, nil, nil, nil, nil, &addr1)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	// transfer coins to reward fund
	amount := big.NewInt(10)
	amount.Exp(amount, big.NewInt(19), nil)
	coinContract.Transfer(contractpb.WrapPluginContext(coinCtx), &coin.TransferRequest{
		To: dposAddr.MarshalPB(),
		Amount: &types.BigUInt{
			Value: common.BigUInt{amount},
		},
	})

	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr1.MarshalPB(),
					Amount:           &types.BigUInt{Value: loom.BigUInt{big.NewInt(1000000000000)}},
					LocktimeTier:     0,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 1,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	whitelistAmount := loom.BigUInt{big.NewInt(1000000000000)}

	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr2.MarshalPB(),
					Amount:           &types.BigUInt{Value: whitelistAmount},
					LocktimeTier:     0,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 2,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	err = dposContract.ProcessRequestBatch(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RequestBatch{
		Batch: []*dtypes.BatchRequest{
			&dtypes.BatchRequest{
				Payload: &dtypes.BatchRequest_WhitelistCandidate{&WhitelistCandidateRequest{
					CandidateAddress: addr3.MarshalPB(),
					Amount:           &types.BigUInt{Value: whitelistAmount},
					LocktimeTier:     0,
				}},
				Meta: &dtypes.BatchRequestMeta{
					BlockNumber: 3,
					TxIndex:     0,
					LogIndex:    0,
				},
			},
		},
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &RegisterCandidateRequest{
		PubKey: pubKey2,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &RegisterCandidateRequest{
		PubKey: pubKey3,
	})
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 3)

	listValidatorsResponse, err := dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)

	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 2)

	oldRewardsValue := *common.BigZero()
	for i := 0; i < 10; i++ {
		require.NoError(t, elect(pctx, dpos.Address))
		checkDelegation, _ := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
			ValidatorAddress: addr1.MarshalPB(),
			DelegatorAddress: addr1.MarshalPB(),
		})
		// get rewards delegaiton which is always at index 0
		delegation := checkDelegation.Delegations[REWARD_DELEGATION_INDEX]
		assert.Equal(t, delegation.Amount.Value.Cmp(&oldRewardsValue), 1)
		oldRewardsValue = delegation.Amount.Value
	}

	// Change WhitelistAmount and verify that it got changed correctly
	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	validator := listValidatorsResponse.Statistics[0]
	assert.Equal(t, whitelistAmount, validator.WhitelistAmount.Value)

	newWhitelistAmount := loom.BigUInt{big.NewInt(2000000000000)}

	// only oracle
	err = dposContract.ChangeWhitelistInfo(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &ChangeWhitelistInfoRequest{
		CandidateAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: newWhitelistAmount},
	})
	require.Error(t, err)

	err = dposContract.ChangeWhitelistInfo(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &ChangeWhitelistInfoRequest{
		CandidateAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: newWhitelistAmount},
	})
	require.Nil(t, err)

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	validator = listValidatorsResponse.Statistics[0]
	assert.Equal(t, newWhitelistAmount, validator.WhitelistAmount.Value)
}

func TestValidatorRewards(t *testing.T) {
	// Init the coin balances
	pctx := plugin.CreateFakeContext(delegatorAddress1, loom.Address{}).WithBlock(loom.BlockHeader{
		ChainID: chainID,
		Time:    startTime,
	})
	coinAddr := pctx.CreateContract(coin.Contract)

	coinContract := &coin.Coin{}
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 100000000),
			makeAccount(delegatorAddress2, 100000000),
			makeAccount(delegatorAddress3, 100000000),
			makeAccount(addr1, 100000000),
			makeAccount(addr2, 100000000),
			makeAccount(addr3, 100000000),
		},
	})

	// create dpos contract
	dpos, err := deployDPOSContract(pctx, 10, nil, &coinAddr, nil, nil, nil, nil, nil)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	// transfer coins to reward fund
	amount := big.NewInt(10)
	amount.Exp(amount, big.NewInt(19), nil)
	coinContract.Transfer(contractpb.WrapPluginContext(coinCtx), &coin.TransferRequest{
		To: dposAddr.MarshalPB(),
		Amount: &types.BigUInt{
			Value: common.BigUInt{amount},
		},
	})

	registrationFee := &types.BigUInt{Value: *scientificNotation(defaultRegistrationRequirement, tokenDecimals)}

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &RegisterCandidateRequest{
		PubKey: pubKey2,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr3)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &RegisterCandidateRequest{
		PubKey: pubKey3,
	})
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 3)

	listValidatorsResponse, err := dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)
	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 3)

	// Two delegators delegate 1/2 and 1/4 of a registration fee respectively
	smallDelegationAmount := loom.NewBigUIntFromInt(0)
	smallDelegationAmount.Div(&registrationFee.Value, loom.NewBigUIntFromInt(4))
	largeDelegationAmount := loom.NewBigUIntFromInt(0)
	largeDelegationAmount.Div(&registrationFee.Value, loom.NewBigUIntFromInt(2))

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *largeDelegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *largeDelegationAmount},
	})
	require.Nil(t, err)

	for i := 0; i < 10000; i++ {
		require.NoError(t, elect(pctx, dpos.Address))
	}

	checkResponse, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, checkResponse.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)
	assert.Equal(t, checkResponse.Amount.Value.Cmp(&checkResponse.Amount.Value), 0)

	delegator1Claim, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: delegatorAddress1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator1Claim.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)

	delegator2Claim, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: delegatorAddress2.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator2Claim.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)

	halvedDelegator2Claim := loom.NewBigUIntFromInt(0)
	halvedDelegator2Claim.Div(&delegator2Claim.Amount.Value, loom.NewBigUIntFromInt(2))
	difference := loom.NewBigUIntFromInt(0)
	difference.Sub(&delegator1Claim.Amount.Value, halvedDelegator2Claim)

	// Checking that Delegator2's claim is almost exactly half of Delegator1's claim
	maximumDifference := scientificNotation(1, tokenDecimals)
	assert.Equal(t, difference.Int.CmpAbs(maximumDifference.Int), -1)

	// Using unbond to claim reward delegation
	err = dposContract.Unbond(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &UnbondRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           loom.BigZeroPB(),
		Index:            REWARD_DELEGATION_INDEX,
	})
	require.Nil(t, err)

	// check that addr1's balance increases after rewards claim
	balanceBeforeUnbond, err := coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr1.MarshalPB(),
	})
	require.Nil(t, err)

	// allowing reward delegation to unbond
	require.NoError(t, elect(pctx, dpos.Address))
	require.Nil(t, err)

	balanceAfterUnbond, err := coinContract.BalanceOf(contractpb.WrapPluginContext(coinCtx), &coin.BalanceOfRequest{
		Owner: addr1.MarshalPB(),
	})
	require.Nil(t, err)

	assert.True(t, balanceAfterUnbond.Balance.Value.Cmp(&balanceBeforeUnbond.Balance.Value) > 0)

	// check that difference is exactly the undelegated amount

	// check current delegation amount
}

func TestReferrerRewards(t *testing.T) {
	// Init the coin balances
	pctx := plugin.CreateFakeContext(delegatorAddress1, loom.Address{}).WithBlock(loom.BlockHeader{
		ChainID: chainID,
		Time:    startTime,
	})
	coinAddr := pctx.CreateContract(coin.Contract)

	coinContract := &coin.Coin{}
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 100000000),
			makeAccount(delegatorAddress2, 100000000),
			makeAccount(delegatorAddress3, 100000000),
			makeAccount(addr1, 100000000),
		},
	})

	// create dpos contract
	dpos, err := deployDPOSContract(pctx, 10, nil, &coinAddr, nil, nil, nil, nil, &addr1)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	registrationFee := &types.BigUInt{Value: *scientificNotation(defaultRegistrationRequirement, tokenDecimals)}

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey:                pubKey1,
		Fee:                   2000,
		MaxReferralPercentage: 10000,
	})
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 1)

	listValidatorsResponse, err := dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)
	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 1)

	del1Name := "del1"
	// Register two referrers
	err = dposContract.RegisterReferrer(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterReferrerRequest{
		Name:    del1Name,
		Address: delegatorAddress1.MarshalPB(),
	})
	require.Nil(t, err)

	err = dposContract.RegisterReferrer(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterReferrerRequest{
		Name:    "del2",
		Address: delegatorAddress2.MarshalPB(),
	})
	require.Nil(t, err)

	delegationAmount := loom.NewBigUIntFromInt(1e18)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress3)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress3)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *delegationAmount},
		Referrer:         del1Name,
	})
	require.Nil(t, err)

	for i := 0; i < 10; i++ {
		require.NoError(t, elect(pctx, dpos.Address))
	}

	checkResponse, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: limboValidatorAddress.MarshalPB(),
		DelegatorAddress: delegatorAddress1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, checkResponse.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)
}

func TestRewardTiers(t *testing.T) {
	// Init the coin balances
	pctx := plugin.CreateFakeContext(delegatorAddress1, loom.Address{}).WithBlock(loom.BlockHeader{
		ChainID: chainID,
		Time:    startTime,
	})
	coinAddr := pctx.CreateContract(coin.Contract)

	coinContract := &coin.Coin{}
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 100000000),
			makeAccount(delegatorAddress2, 100000000),
			makeAccount(delegatorAddress3, 100000000),
			makeAccount(delegatorAddress4, 100000000),
			makeAccount(delegatorAddress5, 100000000),
			makeAccount(delegatorAddress6, 100000000),
			makeAccount(addr1, 100000000),
			makeAccount(addr2, 100000000),
			makeAccount(addr3, 100000000),
		},
	})

	// Init the dpos contract
	dpos, err := deployDPOSContract(pctx, 10, nil, &coinAddr, nil, nil, nil, nil, nil)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	// transfer coins to reward fund
	amount := big.NewInt(10)
	amount.Exp(amount, big.NewInt(19), nil)
	coinContract.Transfer(contractpb.WrapPluginContext(coinCtx), &coin.TransferRequest{
		To: dposAddr.MarshalPB(),
		Amount: &types.BigUInt{
			Value: common.BigUInt{amount},
		},
	})

	registrationFee := &types.BigUInt{Value: *scientificNotation(defaultRegistrationRequirement, tokenDecimals)}

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &RegisterCandidateRequest{
		PubKey: pubKey2,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr3)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  registrationFee,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &RegisterCandidateRequest{
		PubKey: pubKey3,
	})
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 3)

	listValidatorsResponse, err := dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)

	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 3)

	// tinyDelegationAmount = one LOOM token
	tinyDelegationAmount := scientificNotation(1, tokenDecimals)
	smallDelegationAmount := loom.NewBigUIntFromInt(0)
	smallDelegationAmount.Div(&registrationFee.Value, loom.NewBigUIntFromInt(4))
	largeDelegationAmount := loom.NewBigUIntFromInt(0)
	largeDelegationAmount.Div(&registrationFee.Value, loom.NewBigUIntFromInt(2))

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	// LocktimeTier should default to 0 for delegatorAddress1
	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *smallDelegationAmount},
		LocktimeTier:     2,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress3)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress3)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *smallDelegationAmount},
		LocktimeTier:     3,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress4)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *smallDelegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress4)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *smallDelegationAmount},
		LocktimeTier:     1,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress5)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *largeDelegationAmount},
	})
	require.Nil(t, err)

	// Though Delegator5 delegates to addr2 and not addr1 like the rest of the
	// delegators, he should still receive the same rewards proportional to his
	// delegation parameters
	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress5)), &DelegateRequest{
		ValidatorAddress: addr2.MarshalPB(),
		Amount:           &types.BigUInt{Value: *largeDelegationAmount},
		LocktimeTier:     2,
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress6)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *tinyDelegationAmount},
	})
	require.Nil(t, err)

	// by delegating a very small amount, delegator6 demonstrates that
	// delegators can contribute far less than 0.01% of a validator's total
	// delegation and still be rewarded
	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress6)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *tinyDelegationAmount},
	})
	require.Nil(t, err)

	for i := 0; i < 10000; i++ {
		require.NoError(t, elect(pctx, dpos.Address))
	}

	addr1Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, addr1Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	delegator1Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator1Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	delegator2Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator2Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	delegator3Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress3)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator3Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	delegator4Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress4)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator4Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	delegator5Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress5)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr2.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator5Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	delegator6Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress6)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator6Claim.Delegation.Amount.Value.Cmp(common.BigZero()), 1)

	maximumDifference := scientificNotation(1, tokenDecimals)
	difference := loom.NewBigUIntFromInt(0)

	// Checking that Delegator2's claim is almost exactly twice Delegator1's claim
	scaledDelegator1Claim := CalculateFraction(*loom.NewBigUIntFromInt(20000), delegator1Claim.Delegation.Amount.Value)
	difference.Sub(&scaledDelegator1Claim, &delegator2Claim.Delegation.Amount.Value)
	assert.Equal(t, difference.Int.CmpAbs(maximumDifference.Int), -1)

	// Checking that Delegator3's & Delegator5's claim is almost exactly four times Delegator1's claim
	scaledDelegator1Claim = CalculateFraction(*loom.NewBigUIntFromInt(40000), delegator1Claim.Delegation.Amount.Value)

	difference.Sub(&scaledDelegator1Claim, &delegator3Claim.Delegation.Amount.Value)
	assert.Equal(t, difference.Int.CmpAbs(maximumDifference.Int), -1)

	difference.Sub(&scaledDelegator1Claim, &delegator5Claim.Delegation.Amount.Value)
	assert.Equal(t, difference.Int.CmpAbs(maximumDifference.Int), -1)

	// Checking that Delegator4's claim is almost exactly 1.5 times Delegator1's claim
	scaledDelegator1Claim = CalculateFraction(*loom.NewBigUIntFromInt(15000), delegator1Claim.Delegation.Amount.Value)
	difference.Sub(&scaledDelegator1Claim, &delegator4Claim.Delegation.Amount.Value)
	assert.Equal(t, difference.Int.CmpAbs(maximumDifference.Int), -1)

	// Testing total delegation functionality

	checkAllDelegationsResponse, err := dposContract.CheckAllDelegations(contractpb.WrapPluginContext(dposCtx), &CheckAllDelegationsRequest{
		DelegatorAddress: delegatorAddress3.MarshalPB(),
	})
	require.Nil(t, err)
	assert.True(t, checkAllDelegationsResponse.Amount.Value.Cmp(smallDelegationAmount) > 0)
	expectedWeightedAmount := CalculateFraction(*loom.NewBigUIntFromInt(40000), *smallDelegationAmount)
	assert.True(t, checkAllDelegationsResponse.WeightedAmount.Value.Cmp(&expectedWeightedAmount) > 0)
}

// Besides reward cap functionality, this also demostrates 0-fee candidate registration
func TestRewardCap(t *testing.T) {
	// Init the coin balances
	pctx := plugin.CreateFakeContext(delegatorAddress1, loom.Address{}).WithBlock(loom.BlockHeader{
		ChainID: chainID,
		Time:    startTime,
	})
	coinAddr := pctx.CreateContract(coin.Contract)

	coinContract := &coin.Coin{}
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 100000000),
			makeAccount(delegatorAddress2, 100000000),
			makeAccount(delegatorAddress3, 100000000),
			makeAccount(addr1, 100000000),
			makeAccount(addr2, 100000000),
			makeAccount(addr3, 100000000),
		},
	})

	// Init the dpos contract

	maxReward := scientificNotation(100, tokenDecimals)
	dpos, err := deployDPOSContract(pctx, 10, nil, &coinAddr, maxReward, loom.NewBigUIntFromInt(0), nil, nil, nil)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	// transfer coins to reward fund
	amount := big.NewInt(10)
	amount.Exp(amount, big.NewInt(19), nil)
	coinContract.Transfer(contractpb.WrapPluginContext(coinCtx), &coin.TransferRequest{
		To: dposAddr.MarshalPB(),
		Amount: &types.BigUInt{
			Value: common.BigUInt{amount},
		},
	})

	require.Nil(t, err)
	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr2)), &RegisterCandidateRequest{
		PubKey: pubKey2,
	})
	require.Nil(t, err)

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr3)), &RegisterCandidateRequest{
		PubKey: pubKey3,
	})
	require.Nil(t, err)

	candidates, err := dpos.ListCandidates(dposCtx)
	require.Nil(t, err)
	assert.Equal(t, len(candidates), 3)

	listValidatorsResponse, err := dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)

	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 0)

	delegationAmount := scientificNotation(1000, tokenDecimals)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress2)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &DelegateRequest{
		ValidatorAddress: addr2.MarshalPB(),
		Amount:           &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	// With a default yearly reward of 5% of one's token holdings, the two
	// delegators should reach their rewards limits by both delegating exactly
	// 1000, or 2000 combined since 2000 = 100 (the max yearly reward) / 0.05

	require.NoError(t, elect(pctx, dpos.Address))

	listValidatorsResponse, err = dposContract.ListValidators(contractpb.WrapPluginContext(dposCtx), &ListValidatorsRequest{})
	require.Nil(t, err)
	assert.Equal(t, len(listValidatorsResponse.Statistics), 2)

	require.NoError(t, elect(pctx, dpos.Address))

	delegator1Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator1Claim.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)

	delegator2Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress2)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr2.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator2Claim.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)

	//                           |---- this 2 is the election cycle length used when,
	//    v--- delegationAmount  v     for testing, a 0-sec election time is set
	// ((1000 * 10**18) * 0.05 * 2) / (365 * 24 * 3600) = 3.1709791983764585e12
	expectedAmount := loom.NewBigUIntFromInt(3170979198376)
	assert.Equal(t, *expectedAmount, delegator2Claim.Delegation.Amount.Value)

	err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress3)), &coin.ApproveRequest{
		Spender: dposAddr.MarshalPB(),
		Amount:  &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress3)), &DelegateRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           &types.BigUInt{Value: *delegationAmount},
	})
	require.Nil(t, err)

	// run one election to get Delegator3 elected as a validator
	require.NoError(t, elect(pctx, dpos.Address))

	// run another election to get Delegator3 his first reward distribution
	require.NoError(t, elect(pctx, dpos.Address))

	delegator3Claim, err := dposContract.CheckRewardDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress3)), &CheckRewardDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, delegator3Claim.Delegation.Amount.Value.Cmp(&loom.BigUInt{big.NewInt(0)}), 1)
	// verifiying that claim is smaller than what was given when delegations
	// were smaller and below max yearly reward cap.
	// delegator3Claim should be ~2/3 of delegator2Claim
	assert.Equal(t, delegator2Claim.Delegation.Amount.Value.Cmp(&delegator3Claim.Delegation.Amount.Value), 1)
	scaledDelegator3Claim := CalculateFraction(*loom.NewBigUIntFromInt(15000), delegator3Claim.Delegation.Amount.Value)
	difference := common.BigZero()
	difference.Sub(&scaledDelegator3Claim, &delegator2Claim.Delegation.Amount.Value)
	// amounts must be within 7 * 10^-10 tokens of each other to be correct
	maximumDifference := loom.NewBigUIntFromInt(700000000)
	assert.Equal(t, difference.Int.CmpAbs(maximumDifference.Int), -1)
}

func TestMultiDelegate(t *testing.T) {
	pctx := plugin.CreateFakeContext(addr1, addr1)

	// Deploy the coin contract (DPOS Init() will attempt to resolve it)
	coinContract := &coin.Coin{}
	coinAddr := pctx.CreateContract(coin.Contract)
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(delegatorAddress1, 1000000000000000000),
			makeAccount(addr1, 1000000000000000000),
		},
	})

	dpos, err := deployDPOSContract(pctx, 21, nil, nil, nil, loom.NewBigUIntFromInt(0), nil, nil, nil)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	delegationAmount := &types.BigUInt{Value: loom.BigUInt{big.NewInt(2000)}}
	numberOfDelegations := int64(200)

	for i := uint64(0); i < uint64(numberOfDelegations); i++ {
		err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(addr1)), &coin.ApproveRequest{
			Spender: dposAddr.MarshalPB(),
			Amount:  delegationAmount,
		})
		require.Nil(t, err)

		err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &DelegateRequest{
			ValidatorAddress: addr1.MarshalPB(),
			Amount:           delegationAmount,
			LocktimeTier:     i % 4, // testing delegations with a variety of locktime tiers
		})
		require.Nil(t, err)

		require.NoError(t, elect(pctx, dpos.Address))
	}

	delegationResponse, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: addr1.MarshalPB(),
	})
	require.Nil(t, err)
	expectedAmount := common.BigZero()
	expectedAmount = expectedAmount.Mul(&delegationAmount.Value, &loom.BigUInt{big.NewInt(numberOfDelegations)})
	assert.True(t, delegationResponse.Amount.Value.Cmp(expectedAmount) == 0)
	// we add one to account for the rewards delegation
	assert.True(t, len(delegationResponse.Delegations) == int(numberOfDelegations+1))

	numDelegations := DelegationsCount(contractpb.WrapPluginContext(dposCtx))
	assert.Equal(t, numDelegations, 201)

	for i := uint64(0); i < uint64(numberOfDelegations); i++ {
		err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(delegatorAddress1)), &coin.ApproveRequest{
			Spender: dposAddr.MarshalPB(),
			Amount:  delegationAmount,
		})
		require.Nil(t, err)

		err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(delegatorAddress1)), &DelegateRequest{
			ValidatorAddress: addr1.MarshalPB(),
			Amount:           delegationAmount,
			LocktimeTier:     i % 4, // testing delegations with a variety of locktime tiers
		})
		require.Nil(t, err)

		require.NoError(t, elect(pctx, dpos.Address))
	}

	delegationResponse, err = dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: delegatorAddress1.MarshalPB(),
	})
	require.Nil(t, err)
	assert.True(t, delegationResponse.Amount.Value.Cmp(expectedAmount) == 0)
	assert.True(t, len(delegationResponse.Delegations) == int(numberOfDelegations+1))

	numDelegations = DelegationsCount(contractpb.WrapPluginContext(dposCtx))
	assert.Equal(t, numDelegations, 402)

	// advance contract time enough to unlock all delegations
	now := uint64(dposCtx.Now().Unix())
	dposCtx.SetTime(dposCtx.Now().Add(time.Duration(now+TierLocktimeMap[3]+1) * time.Second))

	err = dposContract.Unbond(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &UnbondRequest{
		ValidatorAddress: addr1.MarshalPB(),
		Amount:           delegationAmount,
		Index:            100,
	})
	require.Nil(t, err)

	require.NoError(t, elect(pctx, dpos.Address))

	numDelegations = DelegationsCount(contractpb.WrapPluginContext(dposCtx))
	assert.Equal(t, numDelegations, 402-1)

	// Check that all delegations have had their tier reset to TIER_ZERO
	listAllDelegationsResponse, err := dposContract.ListAllDelegations(contractpb.WrapPluginContext(dposCtx), &ListAllDelegationsRequest{})
	require.Nil(t, err)

	for _, listDelegationsResponse := range listAllDelegationsResponse.ListResponses {
		for _, delegation := range listDelegationsResponse.Delegations {
			assert.Equal(t, delegation.LocktimeTier, TIER_ZERO)
		}
	}
}

func TestLockup(t *testing.T) {
	pctx := plugin.CreateFakeContext(addr1, addr1)

	// Deploy the coin contract (DPOS Init() will attempt to resolve it)
	coinContract := &coin.Coin{}
	coinAddr := pctx.CreateContract(coin.Contract)
	coinCtx := pctx.WithAddress(coinAddr)
	coinContract.Init(contractpb.WrapPluginContext(coinCtx), &coin.InitRequest{
		Accounts: []*coin.InitialAccount{
			makeAccount(addr1, 1000000000000000000),
			makeAccount(delegatorAddress1, 1000000000000000000),
			makeAccount(delegatorAddress2, 1000000000000000000),
			makeAccount(delegatorAddress3, 1000000000000000000),
			makeAccount(delegatorAddress4, 1000000000000000000),
		},
	})

	dpos, err := deployDPOSContract(pctx, 21, nil, nil, nil, loom.NewBigUIntFromInt(0), nil, nil, nil)
	require.Nil(t, err)
	dposAddr := dpos.Address
	dposCtx := pctx.WithAddress(dposAddr)
	dposContract := dpos.Contract

	err = dposContract.RegisterCandidate(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &RegisterCandidateRequest{
		PubKey: pubKey1,
	})
	require.Nil(t, err)

	now := uint64(dposCtx.Now().Unix())
	delegationAmount := &types.BigUInt{Value: loom.BigUInt{big.NewInt(2000)}}

	var tests = []struct {
		Delegator loom.Address
		Tier      uint64
	}{
		{delegatorAddress1, 0},
		{delegatorAddress2, 1},
		{delegatorAddress3, 2},
		{delegatorAddress4, 3},
	}

	for _, test := range tests {
		expectedLockup := now + TierLocktimeMap[LocktimeTier(test.Tier)]

		// delegating
		err = coinContract.Approve(contractpb.WrapPluginContext(coinCtx.WithSender(test.Delegator)), &coin.ApproveRequest{
			Spender: dposAddr.MarshalPB(),
			Amount:  delegationAmount,
		})
		require.Nil(t, err)

		err = dposContract.Delegate(contractpb.WrapPluginContext(dposCtx.WithSender(test.Delegator)), &DelegateRequest{
			ValidatorAddress: addr1.MarshalPB(),
			Amount:           delegationAmount,
			LocktimeTier:     test.Tier,
		})
		require.Nil(t, err)

		// checking delegation pre-election
		checkDelegation, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
			ValidatorAddress: addr1.MarshalPB(),
			DelegatorAddress: test.Delegator.MarshalPB(),
		})
		require.Nil(t, err)
		delegation := checkDelegation.Delegations[len(checkDelegation.Delegations)-1]

		assert.Equal(t, expectedLockup, delegation.LockTime)
		assert.Equal(t, true, uint64(delegation.LocktimeTier) == test.Tier)
		assert.Equal(t, delegation.Amount.Value.Cmp(common.BigZero()), 0)
		assert.Equal(t, delegation.UpdateAmount.Value.Cmp(&delegationAmount.Value), 0)

		// running election
		require.NoError(t, elect(pctx, dpos.Address))

		// checking delegation post-election
		checkDelegation, err = dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
			ValidatorAddress: addr1.MarshalPB(),
			DelegatorAddress: test.Delegator.MarshalPB(),
		})
		require.Nil(t, err)
		delegation = checkDelegation.Delegations[len(checkDelegation.Delegations)-1]

		assert.Equal(t, expectedLockup, delegation.LockTime)
		assert.Equal(t, true, uint64(delegation.LocktimeTier) == test.Tier)
		assert.Equal(t, delegation.UpdateAmount.Value.Cmp(common.BigZero()), 0)
		assert.Equal(t, delegation.Amount.Value.Cmp(&delegationAmount.Value), 0)
	}

	// setting time to reset tiers of all delegations except the last
	dposCtx.SetTime(dposCtx.Now().Add(time.Duration(now+TierLocktimeMap[2]+1) * time.Second))

	// running election to trigger locktime resets
	require.NoError(t, elect(pctx, dpos.Address))

	delegationResponse, err := dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: delegatorAddress3.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, TIER_ZERO, delegationResponse.Delegations[len(delegationResponse.Delegations)-1].LocktimeTier)

	delegationResponse, err = dposContract.CheckDelegation(contractpb.WrapPluginContext(dposCtx.WithSender(addr1)), &CheckDelegationRequest{
		ValidatorAddress: addr1.MarshalPB(),
		DelegatorAddress: delegatorAddress4.MarshalPB(),
	})
	require.Nil(t, err)
	assert.Equal(t, TIER_THREE, delegationResponse.Delegations[len(delegationResponse.Delegations)-1].LocktimeTier)
}

func TestApplyPowerCap(t *testing.T) {
	var tests = []struct {
		input  []*Validator
		output []*Validator
	}{
		{
			[]*Validator{&Validator{Power: 10}},
			[]*Validator{&Validator{Power: 10}},
		},
		{
			[]*Validator{&Validator{Power: 10}, &Validator{Power: 1}},
			[]*Validator{&Validator{Power: 10}, &Validator{Power: 1}},
		},
		{
			[]*Validator{&Validator{Power: 30}, &Validator{Power: 30}, &Validator{Power: 30}, &Validator{Power: 30}},
			[]*Validator{&Validator{Power: 30}, &Validator{Power: 30}, &Validator{Power: 30}, &Validator{Power: 30}},
		},
		{
			[]*Validator{&Validator{Power: 33}, &Validator{Power: 30}, &Validator{Power: 22}, &Validator{Power: 22}},
			[]*Validator{&Validator{Power: 29}, &Validator{Power: 29}, &Validator{Power: 24}, &Validator{Power: 24}},
		},
		{
			[]*Validator{&Validator{Power: 100}, &Validator{Power: 20}, &Validator{Power: 5}, &Validator{Power: 5}, &Validator{Power: 5}},
			[]*Validator{&Validator{Power: 37}, &Validator{Power: 35}, &Validator{Power: 20}, &Validator{Power: 20}, &Validator{Power: 20}},
		},
		{
			[]*Validator{&Validator{Power: 150}, &Validator{Power: 100}, &Validator{Power: 77}, &Validator{Power: 15}, &Validator{Power: 15}, &Validator{Power: 10}},
			[]*Validator{&Validator{Power: 102}, &Validator{Power: 102}, &Validator{Power: 86}, &Validator{Power: 24}, &Validator{Power: 24}, &Validator{Power: 19}},
		},
	}
	for _, test := range tests {
		output := applyPowerCap(test.input)
		for i, o := range output {
			assert.Equal(t, test.output[i].Power, o.Power)
		}
	}
}

// UTILITIES

func makeAccount(owner loom.Address, bal uint64) *coin.InitialAccount {
	return &coin.InitialAccount{
		Owner:   owner.MarshalPB(),
		Balance: bal,
	}
}

func elect(pctx *plugin.FakeContext, dposAddr loom.Address) error {
	return Elect(contractpb.WrapPluginContext(pctx.WithAddress(dposAddr)))
}
