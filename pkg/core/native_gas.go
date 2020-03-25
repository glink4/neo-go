package core

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

type gasNative struct {
	nep5TokenNative
	neo *neoNative
}

const gasSyscallName = "Neo.Native.Tokens.GAS"

func newGasNative() *gasNative {
	g := &gasNative{
		nep5TokenNative: nep5TokenNative{
			name:     "GAS",
			symbol:   "gas",
			decimals: 8,
			factor:   100000000,
			hash:     getScriptHash(gasSyscallName),
		},
	}
	g.incBalance = g.increaseBalance
	return g
}

// initFromStore initializes variable contract parameters from the store.
func (g *gasNative) initFromStore(data []byte) error {
	g.totalSupply = emit.BytesToInt(data).Int64()
	return nil
}

func (g *gasNative) serializeState() []byte {
	return emit.IntToBytes(big.NewInt(g.totalSupply))
}

func (g *gasNative) increaseBalance(ic *interopContext, acc *state.Account, amount int64) error {
	if amount == 0 {
		return nil
	} else if amount < 0 && acc.GAS.Balance < -amount {
		return errors.New("insufficient funds")
	}
	acc.GAS.Balance += amount
	return nil
}

func (g *gasNative) Initialize(ic *interopContext) error {
	data, err := ic.dao.GetNativeContractState(g.hash)
	if err == nil {
		return g.initFromStore(data)
	} else if err != storage.ErrKeyNotFound {
		return err
	}

	if err := g.nep5TokenNative.Initialize(); err != nil {
		return err
	}
	h, _, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	g.mint(ic, h, 30000000*g.factor)
	return ic.dao.PutNativeContractState(g.hash, g.serializeState())
}

func (g *gasNative) toNativeContract() *NativeContract {
	c := g.nep5TokenNative.toNativeContract(gasSyscallName)

	desc := newDescriptor("getSysFeeAmount", smartcontract.IntegerType,
		manifest.NewParameter("index", smartcontract.IntegerType))
	md := newMethodMD(g.getSysFeeAmount, 1, smartcontract.NoneFlag)
	c.AddMethod(md, desc, true)

	c.OnPersist = g.onPersist

	return c
}

func (g *gasNative) onPersist(ic *interopContext) error {
	//for _ ,tx := range ic.block.Transactions {
	//	g.burn(ic, tx.Sender, tx.SystemFee + tx.NetworkFee)
	//}
	//validators := g.neo.getNextBlockValidators(ic)
	//var netFee util.Fixed8
	//for _, tx := range ic.block.Transactions {
	//	netFee += tx.NetworkFee
	//}
	//g.mint(ic, <primary>, netFee)
	if err := g.nep5TokenNative.onPersist(ic); err != nil {
		return err
	}
	return ic.dao.PutNativeContractState(g.hash, g.serializeState())
}

func (g *gasNative) getSysFeeAmount(ic *interopContext, args []vm.StackItem) vm.StackItem {
	index := toInt64(args[0])
	h := ic.bc.GetHeaderHash(int(index))
	_, sf, err := ic.dao.GetBlock(h)
	if err != nil {
		panic(err)
	}
	return vm.NewBigIntegerItem(int64(sf))
}

func getStandbyValidatorsHash(ic *interopContext) (util.Uint160, []*keys.PublicKey, error) {
	vs, err := ic.bc.GetStandByValidators()
	if err != nil {
		return util.Uint160{}, nil, err
	}
	s, err := smartcontract.CreateMultiSigRedeemScript(len(vs)/2+1, vs)
	if err != nil {
		return util.Uint160{}, nil, err
	}
	return hash.Hash160(s), vs, nil
}

func getScriptHash(name string) util.Uint160 {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, name)
	return hash.Hash160(w.Bytes())
}
