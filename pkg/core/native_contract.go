package core

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/pkg/errors"
)

// NativeMethod is a signature for a native method.
type NativeMethod = func(ic *interopContext, args []vm.StackItem) vm.StackItem

// MethodMD is a native-contract method descriptor.
type MethodMD struct {
	Func          NativeMethod
	Price         int64
	RequiredFlags smartcontract.CallFlag
}

// NativeContract represents native contract instance.
type NativeContract struct {
	Manifest    manifest.Manifest
	ServiceName string
	ServiceHash uint32
	Script      []byte
	Hash        util.Uint160
	ID          int32
	Methods     map[string]MethodMD
	OnPersist   func(*interopContext) error
}

// NativeContracts is a set of registered native contracts.
type NativeContracts struct {
	byID      map[uint32]*NativeContract
	byHash    map[util.Uint160]*NativeContract
	Contracts []NativeContract
}

// NewNativeContract returns NativeContract with the specified list of methods.
func NewNativeContract(name string) *NativeContract {
	c := &NativeContract{
		ServiceName: name,
		ServiceHash: vm.InteropNameToID([]byte(name)),
		Methods:     make(map[string]MethodMD),
	}

	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, c.ServiceName) // TODO syscall via ID
	c.Script = w.Bytes()
	c.Hash = hash.Hash160(c.Script)
	c.Manifest = *manifest.DefaultManifest(c.Hash)

	return c
}

// AddMethod adds new method to a native contract.
func (c *NativeContract) AddMethod(md *MethodMD, desc *manifest.MethodDescriptor, safe bool) {
	c.Manifest.ABI.Methods = append(c.Manifest.ABI.Methods, *desc)
	c.Methods[desc.Name] = *md
	if safe {
		c.Manifest.SafeMethods.Add(desc.Name)
	}
}

// AddEvent adds new event to a native contract.
func (c *NativeContract) AddEvent(name string, ps ...manifest.Parameter) {
	c.Manifest.ABI.Events = append(c.Manifest.ABI.Events, manifest.EventDescriptor{
		Name:       name,
		Parameters: ps,
	})
}

// NewNativeContracts returns new empty set of native contracts.
func NewNativeContracts() *NativeContracts {
	return &NativeContracts{
		byID:      make(map[uint32]*NativeContract),
		byHash:    make(map[util.Uint160]*NativeContract),
		Contracts: []NativeContract{},
	}
}

// Add adds new native contracts to the list.
func (cs *NativeContracts) Add(c *NativeContract) {
	ln := len(cs.Contracts)
	cs.Contracts = append(cs.Contracts, *c)
	cs.byHash[c.Hash] = &cs.Contracts[ln]
	cs.byID[c.ServiceHash] = &cs.Contracts[ln]
}

// getNativeInterop returns an interop getter for a given set of contracts.
func (cs *NativeContracts) getNativeInterop(ic *interopContext) func(uint32) *vm.InteropFuncPrice {
	return func(id uint32) *vm.InteropFuncPrice {
		if c, ok := cs.byID[id]; ok {
			return &vm.InteropFuncPrice{
				Func:  ic.getNativeInterop(c),
				Price: 0, // TODO price func
			}
		}
		return nil
	}
}

// getNativeInterop returns native contract interop.
func (ic *interopContext) getNativeInterop(c *NativeContract) func(v *vm.VM) error {
	return func(v *vm.VM) error {
		h := getContextScriptHash(v, 0)
		if !h.Equals(c.Hash) {
			return errors.New("invalid hash")
		}
		name := string(v.Estack().Pop().Bytes())
		args := v.Estack().Pop().Array()
		m, ok := c.Methods[name]
		if !ok {
			return fmt.Errorf("method %s not found", name)
		}
		result := m.Func(ic, args)
		v.Estack().PushVal(result)
		return nil
	}
}
