package main

import (
	"fmt"
	"math/big"
	"os"
	"os/exec"

	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

var keyPrefixes [9][]byte = [9][]byte{[]byte("LastHeader"), []byte("LastBlock"), []byte("LastFast"), []byte("h"), []byte("r"), []byte("b"), []byte("H"), []byte("ethereum-config-"), []byte("secure-key-")}

func databaseMap() error {
	fmt.Printf("Initial state 1\n")
	// Configure and generate a sample block chain
	var (
		db       = rawdb.NewMemoryDatabase()
		key, _   = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key1, _  = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		key2, _  = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		address  = crypto.PubkeyToAddress(key.PublicKey)
		address1 = crypto.PubkeyToAddress(key1.PublicKey)
		address2 = crypto.PubkeyToAddress(key2.PublicKey)
		gspec    = &core.Genesis{
			Config: &params.ChainConfig{
				HomesteadBlock:      big.NewInt(0),
				EIP150Block:         big.NewInt(0),
				EIP155Block:         big.NewInt(0),
				EIP158Block:         big.NewInt(0),
				ByzantiumBlock:      big.NewInt(0),
				ConstantinopleBlock: big.NewInt(0),
				PetersburgBlock:     big.NewInt(0),
				IstanbulBlock:       big.NewInt(0),
			},
			Alloc: core.GenesisAlloc{
				address:  {Balance: big.NewInt(9000000000000000000)},
				address1: {Balance: big.NewInt(200000000000000000)},
				address2: {Balance: big.NewInt(300000000000000000)},
			},
		}
	)
	gspec.MustCommit(db)
	engine := ethash.NewFaker()
	chainConfig, _, err := core.SetupGenesisBlock(db, gspec)
	if err != nil {
		return err
	}
	blockchain, err := core.NewBlockChain(db, nil, chainConfig, engine, vm.Config{}, nil)
	if err != nil {
		return err
	}
	_ = blockchain.StateCache().TrieDB()
	// construct
	filename := fmt.Sprintf("db.dot")
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	i := 0
	startGraph(f)
	k_vMap := make(map[string]string)
	iterator := db.NewIterator()
	for iterator.Next() {
		k_vMap[string(iterator.Key())] = string(iterator.Value())
	}
	for n, prefix := range keyPrefixes {

		iterator = db.NewIteratorWithPrefix(prefix)
		startCluster(f, n, string(prefix))
		for iterator.Next() {
			delete(k_vMap, string(iterator.Key()))
			fmt.Fprintf(f, `k_%d -> v_%d`, i, i)
			key := trie.KeybytesTohex(iterator.Key())
			val := trie.KeybytesTohex(iterator.Value())
			horizontal(f, key, 0, fmt.Sprintf("k_%d", i), HexIndexColors, HexFontColors, 0)
			if len(val) > 0 {
				if len(val) > 32 {
					shortenedVal := val[:32]
					horizontal(f, shortenedVal, 0, fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, 0)
				} else {
					horizontal(f, val, 0, fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, 0)
				}
			} else {
				circle(f, fmt.Sprintf("v_%d", i), "...", false)
			}
			i++
		}
		endCluster(f)
	}
	startCluster(f, i, "hashes")
	for k, v := range k_vMap {
		key := trie.KeybytesTohex([]byte(k))
		val := trie.KeybytesTohex([]byte(v))
		fmt.Fprintf(f, `k_%d -> v_%d`, i, i)
		horizontal(f, key, 0, fmt.Sprintf("k_%d", i), HexIndexColors, HexFontColors, 0)
		if len(val) > 0 {
			if len(val) > 32 {
				shortenedVal := val[:32]
				horizontal(f, shortenedVal, 0, fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, 0)
			} else {
				horizontal(f, val, 0, fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, 0)
			}
		} else {
			circle(f, fmt.Sprintf("v_%d", i), "...", false)
		}
		i++
	}
	endCluster(f)
	endGraph(f)
	if err := f.Close(); err != nil {
		return err
	}
	cmd := exec.Command("dot", "-Tpng:gd", "-O", filename)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("error: %v, output: %s\n", err, output)
	}
	return nil
}
