package core

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// ecdsaVerify checks ECDSA signature.
func (ic *interopContext) ecdsaVerify(v *vm.VM) error {
	msg := v.Estack().Pop().Bytes()
	hashToCheck := hash.Sha256(msg).BytesBE()
	keyb := v.Estack().Pop().Bytes()
	signature := v.Estack().Pop().Bytes()
	pkey, err := keys.NewPublicKeyFromBytes(keyb)
	if err != nil {
		return err
	}
	res := pkey.Verify(signature, hashToCheck)
	v.Estack().PushVal(res)
	return nil
}

// ecdsaCheckMultisig checks multiple ECDSA signatures at once.
func (ic *interopContext) ecdsaCheckMultisig(v *vm.VM) error {
	msg, err := v.Estack().Pop().Item().TryBytes()
	if err != nil {
		return err
	}
	hashToCheck := hash.Sha256(msg).BytesBE()
	pkeys, err := v.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %s", err.Error())
	}
	sigs, err := v.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %s", err.Error())
	}
	// It's ok to have more keys than there are signatures (it would
	// just mean that some keys didn't sign), but not the other way around.
	if len(pkeys) < len(sigs) {
		return errors.New("more signatures than there are keys")
	}
	v.SetCheckedHash(hashToCheck)
	sigok := vm.CheckMultisigPar(v, pkeys, sigs)
	v.Estack().PushVal(sigok)
	return nil
}
