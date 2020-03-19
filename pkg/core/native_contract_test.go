package core

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

type testNative struct {
	blocks chan uint32
}

func (tn *testNative) toNativeContract() *NativeContract {
	desc := &manifest.MethodDescriptor{
		Name: "sum",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("addend1", smartcontract.IntegerType),
			manifest.NewParameter("addend2", smartcontract.IntegerType),
		},
		ReturnType: smartcontract.IntegerType,
	}
	md := &MethodMD{
		Func:          tn.sum,
		Price:         1,
		RequiredFlags: smartcontract.NoneFlag,
	}
	c := NewNativeContract("Test.Native.Sum")
	c.AddMethod(md, desc, true)
	c.OnPersist = tn.onPersist
	return c
}

func (tn *testNative) onPersist(ic *interopContext) error {
	select {
	case tn.blocks <- ic.block.Index:
		return nil
	default:
		return errors.New("can't persist cache")
	}
}

func (tn *testNative) sum(_ *interopContext, args []vm.StackItem) vm.StackItem {
	s1, err := args[0].TryInteger()
	if err != nil {
		panic(err)
	}
	s2, err := args[1].TryInteger()
	if err != nil {
		panic(err)
	}
	return vm.NewBigIntegerItem(s1.Int64() + s2.Int64())
}

func TestNativeContract_Invoke(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Log("KEK")
	tn := &testNative{blocks: make(chan uint32, 1)}
	c := tn.toNativeContract()
	t.Log(c.OnPersist == nil)
	chain.RegisterNative(c)

	w := io.NewBufBinWriter()
	emit.Int(w.BinWriter, 14)
	emit.Int(w.BinWriter, 28)
	emit.Int(w.BinWriter, 2)
	emit.Opcode(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "sum")
	emit.AppCall(w.BinWriter, c.Hash, true)
	script := w.Bytes()
	tx := transaction.NewInvocationTX(script, 0)
	b := chain.newBlock(newMinerTX(), tx)
	require.NoError(t, chain.AddBlock(b))

	res, err := chain.GetAppExecResult(tx.Hash())
	require.NoError(t, err)
	require.Equal(t, "HALT", res.VMState)
	require.Equal(t, 1, len(res.Stack))
	require.Equal(t, smartcontract.IntegerType, res.Stack[0].Type)
	require.EqualValues(t, 42, res.Stack[0].Value)

	require.NoError(t, chain.persist())
	select {
	case index := <-tn.blocks:
		require.Equal(t, chain.blockHeight, index)
	default:
		require.Fail(t, "onPersist wasn't called")
	}
}
