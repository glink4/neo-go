package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// dao is a data access object.
type dao struct {
	store *storage.MemCachedStore
}

func newDao(backend storage.Store) *dao {
	return &dao{store: storage.NewMemCachedStore(backend)}
}

// GetAndDecode performs get operation and decoding with serializable structures.
func (dao *dao) GetAndDecode(entity io.Serializable, key []byte) error {
	entityBytes, err := dao.store.Get(key)
	if err != nil {
		return err
	}
	reader := io.NewBinReaderFromBuf(entityBytes)
	entity.DecodeBinary(reader)
	return reader.Err
}

// Put performs put operation with serializable structures.
func (dao *dao) Put(entity io.Serializable, key []byte) error {
	return dao.putWithBuffer(entity, key, io.NewBufBinWriter())
}

// putWithBuffer performs put operation using buf as a pre-allocated buffer for serialization.
func (dao *dao) putWithBuffer(entity io.Serializable, key []byte, buf *io.BufBinWriter) error {
	entity.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.store.Put(key, buf.Bytes())
}

// -- start accounts.

// GetAccountStateOrNew retrieves Account from temporary or persistent Store
// or creates a new one if it doesn't exist and persists it.
func (dao *dao) GetAccountStateOrNew(hash util.Uint160) (*state.Account, error) {
	account, err := dao.GetAccountState(hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		account = state.NewAccount(hash)
	}
	return account, nil
}

// GetAccountState returns Account from the given Store if it's
// present there. Returns nil otherwise.
func (dao *dao) GetAccountState(hash util.Uint160) (*state.Account, error) {
	account := &state.Account{}
	key := storage.AppendPrefix(storage.STAccount, hash.BytesBE())
	err := dao.GetAndDecode(account, key)
	if err != nil {
		return nil, err
	}
	return account, err
}

func (dao *dao) PutAccountState(as *state.Account) error {
	return dao.putAccountState(as, io.NewBufBinWriter())
}

func (dao *dao) putAccountState(as *state.Account, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STAccount, as.ScriptHash.BytesBE())
	return dao.putWithBuffer(as, key, buf)
}

// -- end accounts.

// -- start assets.

// GetAssetState returns given asset state as recorded in the given store.
func (dao *dao) GetAssetState(assetID util.Uint256) (*state.Asset, error) {
	asset := &state.Asset{}
	key := storage.AppendPrefix(storage.STAsset, assetID.BytesBE())
	err := dao.GetAndDecode(asset, key)
	if err != nil {
		return nil, err
	}
	if asset.ID != assetID {
		return nil, fmt.Errorf("found asset id is not equal to expected")
	}
	return asset, nil
}

// PutAssetState puts given asset state into the given store.
func (dao *dao) PutAssetState(as *state.Asset) error {
	key := storage.AppendPrefix(storage.STAsset, as.ID.BytesBE())
	return dao.Put(as, key)
}

// -- end assets.

// -- start contracts.

// GetContractState returns contract state as recorded in the given
// store by the given script hash.
func (dao *dao) GetContractState(hash util.Uint160) (*state.Contract, error) {
	contract := &state.Contract{}
	key := storage.AppendPrefix(storage.STContract, hash.BytesBE())
	err := dao.GetAndDecode(contract, key)
	if err != nil {
		return nil, err
	}
	if contract.ScriptHash() != hash {
		return nil, fmt.Errorf("found script hash is not equal to expected")
	}

	return contract, nil
}

// PutContractState puts given contract state into the given store.
func (dao *dao) PutContractState(cs *state.Contract) error {
	key := storage.AppendPrefix(storage.STContract, cs.ScriptHash().BytesBE())
	return dao.Put(cs, key)
}

// DeleteContractState deletes given contract state in the given store.
func (dao *dao) DeleteContractState(hash util.Uint160) error {
	key := storage.AppendPrefix(storage.STContract, hash.BytesBE())
	return dao.store.Delete(key)
}

// GetNativeContractState retrieves native contract state from the store.
func (dao *dao) GetNativeContractState(h util.Uint160) ([]byte, error) {
	key := storage.AppendPrefix(storage.STNativeContract, h.BytesBE())
	return dao.store.Get(key)
}

// PutNativeContractState puts native contract state into the store.
func (dao *dao) PutNativeContractState(h util.Uint160, value []byte) error {
	key := storage.AppendPrefix(storage.STNativeContract, h.BytesBE())
	return dao.store.Put(key, value)
}

// -- end contracts.

// -- start nep5 balances.

// GetNEP5Balances retrieves nep5 balances from the cache.
func (dao *dao) GetNEP5Balances(acc util.Uint160) (*state.NEP5Balances, error) {
	key := storage.AppendPrefix(storage.STNEP5Balances, acc.BytesBE())
	bs := state.NewNEP5Balances()
	err := dao.GetAndDecode(bs, key)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}
	return bs, nil
}

// PutNEP5Balances saves nep5 balances from the cache.
func (dao *dao) PutNEP5Balances(acc util.Uint160, bs *state.NEP5Balances) error {
	return dao.putNEP5Balances(acc, bs, io.NewBufBinWriter())
}

func (dao *dao) putNEP5Balances(acc util.Uint160, bs *state.NEP5Balances, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STNEP5Balances, acc.BytesBE())
	return dao.putWithBuffer(bs, key, buf)
}

// -- end nep5 balances.

// -- start transfer log.

const nep5TransferBatchSize = 128

func getNEP5TransferLogKey(acc util.Uint160, index uint32) []byte {
	key := make([]byte, 1+util.Uint160Size+4)
	key[0] = byte(storage.STNEP5Transfers)
	copy(key[1:], acc.BytesBE())
	binary.LittleEndian.PutUint32(key[util.Uint160Size:], index)
	return key
}

// GetNEP5TransferLog retrieves transfer log from the cache.
func (dao *dao) GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.NEP5TransferLog, error) {
	key := getNEP5TransferLogKey(acc, index)
	value, err := dao.store.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return new(state.NEP5TransferLog), nil
		}
		return nil, err
	}
	return &state.NEP5TransferLog{Raw: value}, nil
}

// PutNEP5TransferLog saves given transfer log in the cache.
func (dao *dao) PutNEP5TransferLog(acc util.Uint160, index uint32, lg *state.NEP5TransferLog) error {
	key := getNEP5TransferLogKey(acc, index)
	return dao.store.Put(key, lg.Raw)
}

// AppendNEP5Transfer appends a single NEP5 transfer to a log.
// First return value signalizes that log size has exceeded batch size.
func (dao *dao) AppendNEP5Transfer(acc util.Uint160, index uint32, tr *state.NEP5Transfer) (bool, error) {
	lg, err := dao.GetNEP5TransferLog(acc, index)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return false, err
		}
		lg = new(state.NEP5TransferLog)
	}
	if err := lg.Append(tr); err != nil {
		return false, err
	}
	return lg.Size() >= nep5TransferBatchSize, dao.PutNEP5TransferLog(acc, index, lg)
}

// -- end transfer log.

// -- start unspent coins.

// GetUnspentCoinState retrieves UnspentCoinState from the given store.
func (dao *dao) GetUnspentCoinState(hash util.Uint256) (*state.UnspentCoin, error) {
	unspent := &state.UnspentCoin{}
	key := storage.AppendPrefix(storage.STCoin, hash.BytesLE())
	err := dao.GetAndDecode(unspent, key)
	if err != nil {
		return nil, err
	}
	return unspent, nil
}

// PutUnspentCoinState puts given UnspentCoinState into the given store.
func (dao *dao) PutUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin) error {
	return dao.putUnspentCoinState(hash, ucs, io.NewBufBinWriter())
}

func (dao *dao) putUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STCoin, hash.BytesLE())
	return dao.putWithBuffer(ucs, key, buf)
}

// -- end unspent coins.

// -- start validator.

// GetValidatorStateOrNew gets validator from store or created new one in case of error.
func (dao *dao) GetValidatorStateOrNew(publicKey *keys.PublicKey) (*state.Validator, error) {
	validatorState, err := dao.GetValidatorState(publicKey)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		validatorState = &state.Validator{PublicKey: publicKey}
	}
	return validatorState, nil

}

// GetValidators returns all validators from store.
func (dao *dao) GetValidators() []*state.Validator {
	var validators []*state.Validator
	dao.store.Seek(storage.STValidator.Bytes(), func(k, v []byte) {
		r := io.NewBinReaderFromBuf(v)
		validator := &state.Validator{}
		validator.DecodeBinary(r)
		if r.Err != nil {
			return
		}
		validators = append(validators, validator)
	})
	return validators
}

// GetValidatorState returns validator by publicKey.
func (dao *dao) GetValidatorState(publicKey *keys.PublicKey) (*state.Validator, error) {
	validatorState := &state.Validator{}
	key := storage.AppendPrefix(storage.STValidator, publicKey.Bytes())
	err := dao.GetAndDecode(validatorState, key)
	if err != nil {
		return nil, err
	}
	return validatorState, nil
}

// PutValidatorState puts given Validator into the given store.
func (dao *dao) PutValidatorState(vs *state.Validator) error {
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return dao.Put(vs, key)
}

// DeleteValidatorState deletes given Validator into the given store.
func (dao *dao) DeleteValidatorState(vs *state.Validator) error {
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return dao.store.Delete(key)
}

// GetValidatorsCount returns current ValidatorsCount or new one if there is none
// in the DB.
func (dao *dao) GetValidatorsCount() (*state.ValidatorsCount, error) {
	vc := &state.ValidatorsCount{}
	key := []byte{byte(storage.IXValidatorsCount)}
	err := dao.GetAndDecode(vc, key)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}
	return vc, nil
}

// PutValidatorsCount put given ValidatorsCount in the store.
func (dao *dao) PutValidatorsCount(vc *state.ValidatorsCount) error {
	key := []byte{byte(storage.IXValidatorsCount)}
	return dao.Put(vc, key)
}

// -- end validator.

// -- start notification event.

// GetAppExecResult gets application execution result from the
// given store.
func (dao *dao) GetAppExecResult(hash util.Uint256) (*state.AppExecResult, error) {
	aer := &state.AppExecResult{}
	key := storage.AppendPrefix(storage.STNotification, hash.BytesBE())
	err := dao.GetAndDecode(aer, key)
	if err != nil {
		return nil, err
	}
	return aer, nil
}

// PutAppExecResult puts given application execution result into the
// given store.
func (dao *dao) PutAppExecResult(aer *state.AppExecResult) error {
	key := storage.AppendPrefix(storage.STNotification, aer.TxHash.BytesBE())
	return dao.Put(aer, key)
}

// -- end notification event.

// -- start storage item.

// GetStorageItem returns StorageItem if it exists in the given Store.
func (dao *dao) GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem {
	b, err := dao.store.Get(makeStorageItemKey(scripthash, key))
	if err != nil {
		return nil
	}
	r := io.NewBinReaderFromBuf(b)

	si := &state.StorageItem{}
	si.DecodeBinary(r)
	if r.Err != nil {
		return nil
	}

	return si
}

// PutStorageItem puts given StorageItem for given script with given
// key into the given Store.
func (dao *dao) PutStorageItem(scripthash util.Uint160, key []byte, si *state.StorageItem) error {
	return dao.Put(si, makeStorageItemKey(scripthash, key))
}

// DeleteStorageItem drops storage item for the given script with the
// given key from the Store.
func (dao *dao) DeleteStorageItem(scripthash util.Uint160, key []byte) error {
	return dao.store.Delete(makeStorageItemKey(scripthash, key))
}

// GetStorageItems returns all storage items for a given scripthash.
func (dao *dao) GetStorageItems(hash util.Uint160) (map[string]*state.StorageItem, error) {
	var siMap = make(map[string]*state.StorageItem)
	var err error

	saveToMap := func(k, v []byte) {
		if err != nil {
			return
		}
		r := io.NewBinReaderFromBuf(v)
		si := &state.StorageItem{}
		si.DecodeBinary(r)
		if r.Err != nil {
			err = r.Err
			return
		}

		// Cut prefix and hash.
		siMap[string(k[21:])] = si
	}
	dao.store.Seek(storage.AppendPrefix(storage.STStorage, hash.BytesLE()), saveToMap)
	if err != nil {
		return nil, err
	}
	return siMap, nil
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func makeStorageItemKey(scripthash util.Uint160, key []byte) []byte {
	return storage.AppendPrefix(storage.STStorage, append(scripthash.BytesLE(), key...))
}

// -- end storage item.

// -- other.

// GetBlock returns Block by the given hash if it exists in the store.
func (dao *dao) GetBlock(hash util.Uint256) (*block.Block, uint32, error) {
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesLE())
	b, err := dao.store.Get(key)
	if err != nil {
		return nil, 0, err
	}

	block, err := block.NewBlockFromTrimmedBytes(b[4:])
	if err != nil {
		return nil, 0, err
	}
	return block, binary.LittleEndian.Uint32(b[:4]), nil
}

// GetVersion attempts to get the current version stored in the
// underlying Store.
func (dao *dao) GetVersion() (string, error) {
	version, err := dao.store.Get(storage.SYSVersion.Bytes())
	return string(version), err
}

// GetCurrentBlockHeight returns the current block height found in the
// underlying Store.
func (dao *dao) GetCurrentBlockHeight() (uint32, error) {
	b, err := dao.store.Get(storage.SYSCurrentBlock.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[32:36]), nil
}

// GetCurrentHeaderHeight returns the current header height and hash from
// the underlying Store.
func (dao *dao) GetCurrentHeaderHeight() (i uint32, h util.Uint256, err error) {
	var b []byte
	b, err = dao.store.Get(storage.SYSCurrentHeader.Bytes())
	if err != nil {
		return
	}
	i = binary.LittleEndian.Uint32(b[32:36])
	h, err = util.Uint256DecodeBytesLE(b[:32])
	return
}

// GetHeaderHashes returns a sorted list of header hashes retrieved from
// the given underlying Store.
func (dao *dao) GetHeaderHashes() ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	dao.store.Seek(storage.IXHeaderHashList.Bytes(), func(k, v []byte) {
		storedCount := binary.LittleEndian.Uint32(k[1:])
		hashes, err := read2000Uint256Hashes(v)
		if err != nil {
			panic(err)
		}
		hashMap[storedCount] = hashes
	})

	var (
		hashes     = make([]util.Uint256, 0, len(hashMap))
		sortedKeys = make([]uint32, 0, len(hashMap))
	)

	for k := range hashMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Sort(slice(sortedKeys))

	for _, key := range sortedKeys {
		hashes = append(hashes[:key], hashMap[key]...)
	}

	return hashes, nil
}

// GetTransaction returns Transaction and its height by the given hash
// if it exists in the store.
func (dao *dao) GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error) {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	b, err := dao.store.Get(key)
	if err != nil {
		return nil, 0, err
	}
	r := io.NewBinReaderFromBuf(b)

	var height = r.ReadU32LE()

	tx := &transaction.Transaction{}
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, 0, r.Err
	}

	return tx, height, nil
}

// PutVersion stores the given version in the underlying Store.
func (dao *dao) PutVersion(v string) error {
	return dao.store.Put(storage.SYSVersion.Bytes(), []byte(v))
}

// PutCurrentHeader stores current header.
func (dao *dao) PutCurrentHeader(hashAndIndex []byte) error {
	return dao.store.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndex)
}

// read2000Uint256Hashes attempts to read 2000 Uint256 hashes from
// the given byte array.
func read2000Uint256Hashes(b []byte) ([]util.Uint256, error) {
	r := bytes.NewReader(b)
	br := io.NewBinReaderFromIO(r)
	hashes := make([]util.Uint256, 0)
	br.ReadArray(&hashes)
	if br.Err != nil {
		return nil, br.Err
	}
	return hashes, nil
}

// HasTransaction returns true if the given store contains the given
// Transaction hash.
func (dao *dao) HasTransaction(hash util.Uint256) bool {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	if _, err := dao.store.Get(key); err == nil {
		return true
	}
	return false
}

// StoreAsBlock stores the given block as DataBlock.
func (dao *dao) StoreAsBlock(block *block.Block, sysFee uint32) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesLE())
		buf = io.NewBufBinWriter()
	)
	buf.WriteU32LE(sysFee)
	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.WriteBytes(b)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.store.Put(key, buf.Bytes())
}

// StoreAsCurrentBlock stores the given block witch prefix SYSCurrentBlock.
func (dao *dao) StoreAsCurrentBlock(block *block.Block) error {
	buf := io.NewBufBinWriter()
	h := block.Hash()
	h.EncodeBinary(buf.BinWriter)
	buf.WriteU32LE(block.Index)
	return dao.store.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// StoreAsTransaction stores the given TX as DataTransaction.
func (dao *dao) StoreAsTransaction(tx *transaction.Transaction, index uint32) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesLE())
	buf := io.NewBufBinWriter()
	buf.WriteU32LE(index)
	tx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.store.Put(key, buf.Bytes())
}

// IsDoubleSpend verifies that the input transactions are not double spent.
func (dao *dao) IsDoubleSpend(tx *transaction.Transaction) bool {
	return dao.checkUsedInputs(tx.Inputs, state.CoinSpent)
}

// IsDoubleClaim verifies that given claim inputs are not already claimed by another tx.
func (dao *dao) IsDoubleClaim(claim *transaction.ClaimTX) bool {
	return dao.checkUsedInputs(claim.Claims, state.CoinClaimed)
}

func (dao *dao) checkUsedInputs(inputs []transaction.Input, coin state.Coin) bool {
	if len(inputs) == 0 {
		return false
	}
	for _, inputs := range transaction.GroupInputsByPrevHash(inputs) {
		prevHash := inputs[0].PrevHash
		unspent, err := dao.GetUnspentCoinState(prevHash)
		if err != nil {
			return true
		}
		for _, input := range inputs {
			if int(input.PrevIndex) >= len(unspent.States) || (unspent.States[input.PrevIndex].State&coin) != 0 {
				return true
			}
		}
	}
	return false
}

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store.
func (dao *dao) Persist() (int, error) {
	return dao.store.Persist()
}
