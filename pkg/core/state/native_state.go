package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// NEP5BalanceState represents balance state of a NEP5-token.
type NEP5BalanceState struct {
	Balance int64
}

// NEOBalanceState represents balance state of a NEO-token.
type NEOBalanceState struct {
	NEP5BalanceState
	BalanceHeight uint32
}

// EncodeBinary implements io.Serializable interface.
func (s *NEP5BalanceState) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(uint64(s.Balance))
}

// DecodeBinary implements io.Serializable interface.
func (s *NEP5BalanceState) DecodeBinary(r *io.BinReader) {
	s.Balance = int64(r.ReadU64LE())
}

// EncodeBinary implements io.Serializable interface.
func (s *NEOBalanceState) EncodeBinary(w *io.BinWriter) {
	s.NEP5BalanceState.EncodeBinary(w)
	w.WriteU32LE(s.BalanceHeight)
}

// DecodeBinary implements io.Serializable interface.
func (s *NEOBalanceState) DecodeBinary(r *io.BinReader) {
	s.NEP5BalanceState.DecodeBinary(r)
	s.BalanceHeight = r.ReadU32LE()
}
