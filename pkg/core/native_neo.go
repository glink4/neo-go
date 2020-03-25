package core

import (
	"math/big"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/pkg/errors"
)

type neoNative struct {
	nep5TokenNative
	gas *gasNative
}

const neoSyscallName = "Neo.Native.Tokens.NEO"

func newNeoNative() *neoNative {
	n := &neoNative{
		nep5TokenNative: nep5TokenNative{
			name:     "NEO",
			symbol:   "neo",
			decimals: 0,
			factor:   1,
			hash:     getScriptHash(neoSyscallName),
		},
	}
	n.incBalance = n.increaseBalance
	return n
}

func (n *neoNative) Initialize(ic *interopContext) error {
	data, err := ic.dao.GetNativeContractState(n.hash)
	if err == nil {
		return n.initFromStore(data)
	} else if err != storage.ErrKeyNotFound {
		return err
	}

	if err := n.nep5TokenNative.Initialize(); err != nil {
		return err
	}

	h, vs, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	n.mint(ic, h, 100000000*n.factor)

	for i := range vs {
		if err := n.registerValidatorInternal(ic, vs[i]); err != nil {
			return err
		}
	}

	return ic.dao.PutNativeContractState(n.hash, n.serializeState())
}

// initFromStore initializes variable contract parameters from the store.
func (n *neoNative) initFromStore(data []byte) error {
	n.totalSupply = emit.BytesToInt(data).Int64()
	return nil
}

func (n *neoNative) serializeState() []byte {
	return emit.IntToBytes(big.NewInt(n.totalSupply))
}

func (n *neoNative) toNativeContract() *NativeContract {
	c := n.nep5TokenNative.toNativeContract(neoSyscallName)

	desc := newDescriptor("unclaimedGas", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("end", smartcontract.IntegerType))
	md := newMethodMD(n.unclaimedGas, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, true)

	desc = newDescriptor("registerValidator", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodMD(n.registerValidator, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, false)

	desc = newDescriptor("vote", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("pubkeys", smartcontract.ArrayType))
	md = newMethodMD(n.vote, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, false)

	desc = newDescriptor("getRegisteredValidators", smartcontract.ArrayType)
	md = newMethodMD(n.getRegisteredValidators, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, true)

	desc = newDescriptor("getValidators", smartcontract.ArrayType)
	md = newMethodMD(n.getValidators, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, true)

	desc = newDescriptor("getNextBlockValidators", smartcontract.ArrayType)
	md = newMethodMD(n.getNextBlockValidators, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, true)

	c.OnPersist = n.onPersist

	return c
}

func (n *neoNative) onPersist(ic *interopContext) error {
	// TODO change validators
	if err := n.nep5TokenNative.onPersist(ic); err != nil {
		return err
	}
	return ic.dao.PutNativeContractState(n.hash, n.serializeState())
}

func (n *neoNative) increaseBalance(ic *interopContext, acc *state.Account, amount int64) error {
	if amount == 0 {
		return nil
	} else if amount < 0 && acc.NEO.Balance < -amount {
		return errors.New("insufficient funds")
	}
	if err := n.distributeGas(ic, acc); err != nil {
		return err
	}
	acc.NEO.Balance += amount
	return nil
}

func (n *neoNative) distributeGas(ic *interopContext, acc *state.Account) error {
	if ic.block == nil {
		return nil
	}
	sys, net, err := ic.bc.CalculateClaimable(util.Fixed8(acc.NEO.Balance), acc.NEO.BalanceHeight, ic.block.Index)
	if err != nil {
		return err
	}
	acc.NEO.BalanceHeight = ic.block.Index
	n.gas.mint(ic, acc.ScriptHash, int64(sys+net))
	return nil
}

func (n *neoNative) unclaimedGas(ic *interopContext, args []vm.StackItem) vm.StackItem {
	u := toUint160(args[0])
	end := uint32(toInt64(args[1]))
	bs, err := ic.dao.GetNEP5Balances(u)
	if err != nil {
		panic(err)
	}
	tr := bs.Trackers[n.hash]

	sys, net, err := ic.bc.CalculateClaimable(util.Fixed8(tr.Balance), tr.LastUpdatedBlock, end)
	if err != nil {
		panic(err)
	}
	return vm.NewBigIntegerItem(int64(sys.Add(net)))
}

func (n *neoNative) registerValidator(ic *interopContext, args []vm.StackItem) vm.StackItem {
	err := n.registerValidatorInternal(ic, toPublicKey(args[0]))
	return vm.NewBoolItem(err == nil)
}
func (n *neoNative) registerValidatorInternal(ic *interopContext, pub *keys.PublicKey) error {
	_, err := ic.dao.GetValidatorState(pub)
	if err == nil {
		return err
	}
	return ic.dao.PutValidatorState(&state.Validator{PublicKey: pub})
}
func (n *neoNative) vote(ic *interopContext, args []vm.StackItem) vm.StackItem {
	acc := toUint160(args[0])
	arr := args[1].Value().([]vm.StackItem)
	var pubs keys.PublicKeys
	for i := range arr {
		pub := new(keys.PublicKey)
		bs, err := arr[i].TryBytes()
		if err != nil {
			panic(err)
		} else if err := pub.DecodeBytes(bs); err != nil {
			panic(err)
		}
		pubs = append(pubs, pub)
	}
	err := n.voteInternal(ic, acc, pubs)
	return vm.NewBoolItem(err == nil)
}
func (n *neoNative) voteInternal(ic *interopContext, h util.Uint160, pubs keys.PublicKeys) error {
	ok, err := ic.checkHashedWitness(h)
	if err != nil {
		return err
	} else if !ok {
		return errors.New("invalid signature")
	}
	panic("TODO")
}
func (n *neoNative) getRegisteredValidators(ic *interopContext, _ []vm.StackItem) vm.StackItem {
	vs := ic.dao.GetValidators()
	arr := make([]vm.StackItem, len(vs))
	for i := range vs {
		arr[i] = vm.NewStructItem([]vm.StackItem{
			vm.NewByteArrayItem(vs[i].PublicKey.Bytes()),
			vm.NewBigIntegerItem(int64(vs[i].Votes)),
		})
	}
	return vm.NewArrayItem(arr)
}
func (n *neoNative) getValidators(ic *interopContext, _ []vm.StackItem) vm.StackItem {
	validatorsCount, err := ic.dao.GetValidatorsCount()
	if err != nil {
		panic(err)
	} else if len(validatorsCount) == 0 {
		sb, err := ic.bc.GetStandByValidators()
		if err != nil {
			panic(err)
		}
		return pubsToArray(sb)
	}

	validators := ic.dao.GetValidators()
	sort.Slice(validators, func(i, j int) bool {
		// Unregistered validators go to the end of the list.
		if validators[i].Registered != validators[j].Registered {
			return validators[i].Registered
		}
		// The most-voted validators should end up in the front of the list.
		if validators[i].Votes != validators[j].Votes {
			return validators[i].Votes > validators[j].Votes
		}
		// Ties are broken with public keys.
		return validators[i].PublicKey.Cmp(validators[j].PublicKey) == -1
	})

	count := validatorsCount.GetWeightedAverage()
	standByValidators, err := ic.bc.GetStandByValidators()
	if err != nil {
		panic(err)
	}
	if count < len(standByValidators) {
		count = len(standByValidators)
	}

	uniqueSBValidators := standByValidators.Unique()
	result := keys.PublicKeys{}
	for _, validator := range validators {
		if validator.RegisteredAndHasVotes() || uniqueSBValidators.Contains(validator.PublicKey) {
			result = append(result, validator.PublicKey)
		}
	}

	if result.Len() >= count {
		result = result[:count]
	} else {
		for i := 0; i < uniqueSBValidators.Len() && result.Len() < count; i++ {
			if !result.Contains(uniqueSBValidators[i]) {
				result = append(result, uniqueSBValidators[i])
			}
		}
	}
	sort.Sort(result)
	return pubsToArray(result)
}

func (n *neoNative) getNextBlockValidators(_ *interopContext, _ []vm.StackItem) vm.StackItem {
	panic("TODO")
}

func pubsToArray(pubs keys.PublicKeys) vm.StackItem {
	arr := make([]vm.StackItem, len(pubs))
	for i := range pubs {
		arr[i] = vm.NewByteArrayItem(pubs[i].Bytes())
	}
	return vm.NewArrayItem(arr)
}

func toPublicKey(s vm.StackItem) *keys.PublicKey {
	buf, err := s.TryBytes()
	if err != nil {
		panic(err)
	}
	pub := new(keys.PublicKey)
	if err := pub.DecodeBytes(buf); err != nil {
		panic(err)
	}
	return pub
}
