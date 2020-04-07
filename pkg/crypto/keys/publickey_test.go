package keys

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeInfinity(t *testing.T) {
	key := &PublicKey{}
	b, err := testserdes.EncodeBinary(key)
	require.NoError(t, err)
	require.Equal(t, 1, len(b))

	keyDecode := &PublicKey{}
	require.NoError(t, keyDecode.DecodeBytes(b))
	require.Equal(t, []byte{0x00}, keyDecode.Bytes())
}

func TestEncodeDecodePublicKey(t *testing.T) {
	for i := 0; i < 4; i++ {
		k, err := NewPrivateKey()
		require.NoError(t, err)
		p := k.PublicKey()
		testserdes.EncodeDecodeBinary(t, p, new(PublicKey))
	}

	errCases := [][]byte{{}, {0x02}, {0x04}}

	for _, tc := range errCases {
		require.Error(t, testserdes.DecodeBinary(tc, new(PublicKey)))
	}
}

func TestNewPublicKeyFromBytes(t *testing.T) {
	priv, err := NewPrivateKey()
	require.NoError(t, err)

	b := priv.PublicKey().Bytes()
	pub, err := NewPublicKeyFromBytes(b)
	require.NoError(t, err)
	require.Equal(t, priv.PublicKey(), pub)
}

func TestDecodeFromString(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	require.NoError(t, err)
	require.Equal(t, str, hex.EncodeToString(pubKey.Bytes()))

	_, err = NewPublicKeyFromString(str[2:])
	require.Error(t, err)

	str = "zzb209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	_, err = NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringBadCompressed(t *testing.T) {
	str := "02ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err := NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringBadXMoreThanP(t *testing.T) {
	str := "02ffffffff00000001000000000000000000000001ffffffffffffffffffffffff"
	_, err := NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringNotOnCurve(t *testing.T) {
	str := "04ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err := NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringUncompressed(t *testing.T) {
	str := "046b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c2964fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"
	_, err := NewPublicKeyFromString(str)
	require.NoError(t, err)
}

func TestPubkeyToAddress(t *testing.T) {
	pubKey, err := NewPublicKeyFromString("031ee4e73a17d8f76dc02532e2620bcb12425b33c0c9f9694cc2caa8226b68cad4")
	require.NoError(t, err)
	actual := pubKey.Address()
	expected := "AUpGsNCHzSimeMRVPQfhwrVdiUp8Q2N2Qx"
	require.Equal(t, expected, actual)
}

func TestDecodeBytes(t *testing.T) {
	pubKey := getPubKey(t)
	decodedPubKey := &PublicKey{}
	err := decodedPubKey.DecodeBytes(pubKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, pubKey, decodedPubKey)
}

func TestSort(t *testing.T) {
	pubs1 := make(PublicKeys, 10)
	for i := range pubs1 {
		priv, err := NewPrivateKey()
		require.NoError(t, err)
		pubs1[i] = priv.PublicKey()
	}

	pubs2 := make(PublicKeys, len(pubs1))
	copy(pubs2, pubs1)

	sort.Sort(pubs1)

	rand.Shuffle(len(pubs2), func(i, j int) {
		pubs2[i], pubs2[j] = pubs2[j], pubs2[i]
	})
	sort.Sort(pubs2)

	// Check that sort on the same set of values produce the same result.
	require.Equal(t, pubs1, pubs2)
}

func TestContains(t *testing.T) {
	pubKey := getPubKey(t)
	pubKeys := &PublicKeys{getPubKey(t)}
	pubKeys.Contains(pubKey)
	require.True(t, pubKeys.Contains(pubKey))
}

func TestUnique(t *testing.T) {
	pubKeys := &PublicKeys{getPubKey(t), getPubKey(t)}
	unique := pubKeys.Unique()
	require.Equal(t, 1, unique.Len())
}

func getPubKey(t *testing.T) *PublicKey {
	pubKey, err := NewPublicKeyFromString("031ee4e73a17d8f76dc02532e2620bcb12425b33c0c9f9694cc2caa8226b68cad4")
	require.NoError(t, err)
	return pubKey
}

func TestMarshallJSON(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	require.NoError(t, err)

	bytes, err := json.Marshal(&pubKey)
	require.NoError(t, err)
	require.Equal(t, []byte(`"`+str+`"`), bytes)
}

func TestUnmarshallJSON(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	expected, err := NewPublicKeyFromString(str)
	require.NoError(t, err)

	actual := &PublicKey{}
	err = json.Unmarshal([]byte(`"`+str+`"`), actual)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestUnmarshallJSONBadCompresed(t *testing.T) {
	str := `"02ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`
	actual := &PublicKey{}
	err := json.Unmarshal([]byte(str), actual)
	require.Error(t, err)
}

func TestUnmarshallJSONNotAHex(t *testing.T) {
	str := `"04Tb17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c2964fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"`
	actual := &PublicKey{}
	err := json.Unmarshal([]byte(str), actual)
	require.Error(t, err)
}

func TestUnmarshallJSONBadFormat(t *testing.T) {
	str := "046b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c2964fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"
	actual := &PublicKey{}
	err := json.Unmarshal([]byte(str), actual)
	require.Error(t, err)
}
