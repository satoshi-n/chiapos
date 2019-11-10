package pos

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"math/big"

	"github.com/kargakis/gochia/pkg/utils"
)

const (
	// AES block size
	kBlockSizeBits = aes.BlockSize * 8

	// Extra bits of output from the f functions. Instead of being a function from k -> k bits,
	// it's a function from k -> k + kExtraBits bits. This allows less collisions in matches.
	// Refer to the paper for mathematical motivations.
	kExtraBits = 5

	// Convenience variable
	kExtraBitsPow = 1 << kExtraBits

	// B and C groups which constitute a bucket, or BC group. These groups determine how
	// elements match with each other. Two elements must be in adjacent buckets to match.
	kB      = 60
	kC  int = 509
	kBC     = kB * kC
)

// This (times k) is the length of the metadata that must be kept for each entry. For example,
// for a table 4 entry, we must keep 4k additional bits for each entry, which is used to
// compute f5.
var kVectorLens = map[int]int{
	2: 1,
	3: 2,
	4: 4,
	5: 4,
	6: 3,
	7: 2,
	8: 0,
}

type F1 struct {
	k   uint64
	key cipher.Block
}

func NewF1(k uint64, key []byte) (*F1, error) {
	if k < kMinPlotSize || k > kMaxPlotSize {
		return nil, fmt.Errorf("invalid k: %d, valid range: %d - %d", k, kMinPlotSize, kMaxPlotSize)
	}

	f1 := &F1{
		k: k,
	}

	aesKey := make([]byte, 32)
	// First byte is 1, the index of this table
	aesKey[0] = 1
	copy(aesKey[1:], key)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	f1.key = block

	return f1, nil
}

// Calculate expects an input of 2^k bits
func (f *F1) Calculate(x uint64) uint64 {
	q := big.NewInt(0)
	r := big.NewInt(0)
	q, r = q.DivMod(big.NewInt(0).SetUint64(x*f.k), big.NewInt(kBlockSizeBits), r)

	var qCipher [16]byte
	data := utils.FillToBlock(q.Bytes())
	f.key.Encrypt(qCipher[:], data)
	res := big.NewInt(0).SetBytes(qCipher[:])

	if r.Uint64()+f.k <= kBlockSizeBits {
		res = utils.Trunc(res, r.Uint64(), r.Uint64()+f.k, f.k)
	} else {
		part1 := utils.Trunc(res, r.Uint64(), f.k, f.k)
		var q1Cipher [16]byte
		data := utils.FillToBlock(q.Add(q, big.NewInt(1)).Bytes())
		f.key.Encrypt(q1Cipher[:], data)
		part2 := big.NewInt(0).SetBytes(q1Cipher[:])
		part2 = utils.Trunc(part2, 0, r.Uint64()+f.k-kBlockSizeBits, f.k)
		res = utils.Concat(f.k, part1.Uint64(), part2.Uint64())
	}

	f1x := utils.Concat(f.k, res.Uint64(), x%paramM).Uint64()
	fmt.Printf("Calculated f1(x)=%d for x=%d\n", f1x, x)
	return f1x
}