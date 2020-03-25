package core

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// nep5TokenNative represents NEP-5 token contract.
type nep5TokenNative struct {
	name        string
	symbol      string
	decimals    int64
	factor      int64
	totalSupply int64
	hash        util.Uint160
	incBalance  func(*interopContext, *state.Account, int64) error
}

func (c *nep5TokenNative) Initialize() error {
	return nil
}

func (c *nep5TokenNative) Name(_ *interopContext, _ []vm.StackItem) vm.StackItem {
	return vm.NewByteArrayItem([]byte(c.name))
}

func (c *nep5TokenNative) Symbol(_ *interopContext, _ []vm.StackItem) vm.StackItem {
	return vm.NewByteArrayItem([]byte(c.symbol))
}

func (c *nep5TokenNative) Decimals(_ *interopContext, _ []vm.StackItem) vm.StackItem {
	return vm.NewBigIntegerItem(c.decimals)
}

func (c *nep5TokenNative) Transfer(ic *interopContext, args []vm.StackItem) vm.StackItem {
	from := toUint160(args[0])
	to := toUint160(args[1])
	amount := toInt64(args[2])
	err := c.transfer(ic, from, to, amount)
	return vm.NewBoolItem(err == nil)
}

func addrToStackItem(u *util.Uint160) vm.StackItem {
	if u == nil {
		return nil
	}
	return vm.NewByteArrayItem(u.BytesBE())
}

func (c *nep5TokenNative) emitTransfer(ic *interopContext, from, to *util.Uint160, amount int64) {
	ne := state.NotificationEvent{
		ScriptHash: c.hash,
		Item: vm.NewArrayItem([]vm.StackItem{
			vm.NewByteArrayItem([]byte("Transfer")),
			addrToStackItem(from),
			addrToStackItem(to),
			vm.NewBigIntegerItem(amount),
		}),
	}
	ic.notifications = append(ic.notifications, ne)
}

func (c *nep5TokenNative) transfer(ic *interopContext, from, to util.Uint160, amount int64) error {
	if amount < 0 {
		return errors.New("negative amount")
	}

	accFrom, err := ic.dao.GetAccountStateOrNew(from)
	if err != nil {
		return err
	}

	isEmpty := from.Equals(to) || amount == 0
	inc := amount
	if isEmpty {
		inc = 0
	}
	if err := c.incBalance(ic, accFrom, inc); err != nil {
		return err
	}
	if err := ic.dao.PutAccountState(accFrom); err != nil {
		return err
	}

	if !isEmpty {
		accTo, err := ic.dao.GetAccountStateOrNew(to)
		if err != nil {
			return err
		}
		if err := c.incBalance(ic, accTo, amount); err != nil {
			return err
		}
		if err := ic.dao.PutAccountState(accTo); err != nil {
			return err
		}
	}

	c.emitTransfer(ic, &from, &to, amount)
	return nil
}

func (c *nep5TokenNative) balanceOf(ic *interopContext, args []vm.StackItem) vm.StackItem {
	h := toUint160(args[0])
	bs, err := ic.dao.GetNEP5Balances(h)
	if err != nil {
		panic(err)
	}
	balance := bs.Trackers[c.hash].Balance
	return vm.NewBigIntegerItem(balance)
}

func (c *nep5TokenNative) mint(ic *interopContext, h util.Uint160, amount int64) {
	if amount < 0 {
		panic("negative amount")
	} else if amount == 0 {
		return
	}

	acc, err := ic.dao.GetAccountStateOrNew(h)
	if err != nil {
		panic(err)
	}
	if err := c.incBalance(ic, acc, amount); err != nil {
		panic(err)
	}
	if err := ic.dao.PutAccountState(acc); err != nil {
		panic(err)
	}

	c.totalSupply += amount

	c.emitTransfer(ic, nil, &h, amount)
}

func (c *nep5TokenNative) burn(ic *interopContext, h util.Uint160, amount int64) {
	if amount < 0 {
		panic("negative amount")
	} else if amount == 0 {
		return
	}

	acc, err := ic.dao.GetAccountStateOrNew(h)
	if err != nil {
		panic(err)
	}
	if err := c.incBalance(ic, acc, -amount); err != nil {
		panic(err)
	}
	if err := ic.dao.PutAccountState(acc); err != nil {
		panic(err)
	}

	c.totalSupply -= amount

	c.emitTransfer(ic, &h, nil, amount)
}

func (c *nep5TokenNative) onPersist(*interopContext) error { return nil }

func (c *nep5TokenNative) toNativeContract(name string) *NativeContract {
	n := NewNativeContract(name)
	n.Hash = c.hash

	desc := newDescriptor("name", smartcontract.StringType)
	md := newMethodMD(c.Name, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("symbol", smartcontract.StringType)
	md = newMethodMD(c.Symbol, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("decimals", smartcontract.IntegerType)
	md = newMethodMD(c.Decimals, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodMD(c.balanceOf, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("transfer", smartcontract.BoolType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
	)
	md = newMethodMD(c.Transfer, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, false)
	n.AddEvent("Transfer", desc.Parameters...)

	n.OnPersist = c.onPersist

	return n
}

func newDescriptor(name string, ret smartcontract.ParamType, ps ...manifest.Parameter) *manifest.MethodDescriptor {
	return &manifest.MethodDescriptor{
		Name:       name,
		Parameters: ps,
		ReturnType: ret,
	}
}

func newMethodMD(f NativeMethod, price int64, flags smartcontract.CallFlag) *MethodMD {
	return &MethodMD{
		Func:          f,
		Price:         price,
		RequiredFlags: flags,
	}
}

func toInt64(s vm.StackItem) int64 {
	bi, err := s.TryInteger()
	if err != nil {
		panic(err)
	}
	return bi.Int64()
}

func toUint160(s vm.StackItem) util.Uint160 {
	buf, err := s.TryBytes()
	if err != nil {
		panic(err)
	}
	u, err := util.Uint160DecodeBytesBE(buf)
	if err != nil {
		panic(err)
	}
	return u
}
