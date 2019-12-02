package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/cmd/pics/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

var keyPrefixes = []struct {
	prefix string
	label  string
}{
	{"LastHeader", "Last Header"},
	{"LastBlock", "Last Block"},
	{"LastFast", "Last Fast"},
	{"TrieSync", "Trie Sync"},
	{"h", "Headers"},
	{"r", "Receipts"},
	{"b", "Block Bodies"},
	{"H", "Header Numbers"},
	{"ethereum-config-", "Config"},
	{"secure-key-", "Preimages"},
	{"l", "Transaction Index"},
	{"B", "Bloom Bits"},
	{"t", "Total Difficulty"},
	{"n", "Header Hash"},
	{"DatabaseVersion", "Database Version"},
}

// Assumes that the changes are only insertions, now deletions
func stateDatabaseComparison(first *memorydb.Database, second *memorydb.Database, number int) error {
	filename := fmt.Sprintf("geth_changes_%d.dot", number)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	perBucketFiles := make(map[string]*os.File)
	startGraph(f)
	kvMap := make(map[string][]int)
	var hashes []int
	noValues := make(map[int]struct{})
	it := second.NewIterator()
	i := 0
	for it.Next() {
		// Filter out items that were already present in the fist one and did not change
		if firstV, err := first.Get(it.Key()); err == nil && firstV != nil && bytes.Equal(firstV, it.Value()) {
			continue
		}
		// Produce pair of nodes
		k := string(it.Key())
		key := trie.KeybytesTohex(it.Key())
		val := trie.KeybytesTohex(it.Value())
		horizontal(f, key, 0, fmt.Sprintf("k_%d", i), HexIndexColors, HexFontColors, 0)
		if len(val) > 0 {
			if len(val) > 64 {
				compression := len(val) - 64
				horizontal(f, val, 0, fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, compression)
			} else {
				horizontal(f, val, 0, fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, 0)
			}
		} else {
			noValues[i] = struct{}{}
		}
		// Produce edge
		fmt.Fprintf(f, "k_%d -> v_%d;\n", i, i)
		var prefixFound bool
		var prefix string
		var clusterLabel string
		for _, p := range keyPrefixes {
			if strings.HasPrefix(k, p.prefix) {
				l := kvMap[p.prefix]
				l = append(l, i)
				kvMap[p.prefix] = l
				prefixFound = true
				prefix = p.prefix
				clusterLabel = p.label
				break
			}
		}
		if !prefixFound {
			hashes = append(hashes, i)
			prefix = "hashes"
			clusterLabel = "hashes"
		}
		var f1 *os.File
		var ok bool
		if f1, ok = perBucketFiles[prefix]; !ok {
			f1, err = os.Create(fmt.Sprintf("geth_changes_%d_%s_%d.dot", number, prefix, len(perBucketFiles)))
			if err != nil {
				return err
			}
			startGraph(f1)
			startCluster(f1, 0, clusterLabel)
			perBucketFiles[prefix] = f1
		}
		horizontal(f1, key, len(key), fmt.Sprintf("k_%d", i), HexIndexColors, HexFontColors, 0)
		if len(val) > 0 {
			if len(val) > 64 {
				compression := len(val) - 64
				horizontal(f1, val, len(val), fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, compression)
			} else {
				horizontal(f1, val, len(val), fmt.Sprintf("v_%d", i), HexIndexColors, HexFontColors, 0)
			}
		} else {
			noValues[i] = struct{}{}
		}
		// Produce edge
		fmt.Fprintf(f1, "k_%d -> v_%d;\n", i, i)
		i++
	}
	for n, p := range keyPrefixes {
		lst := kvMap[p.prefix]
		if len(lst) == 0 {
			continue
		}
		startCluster(f, n, p.label)
		for _, item := range lst {
			if _, ok1 := noValues[item]; ok1 {
				fmt.Fprintf(f, "k_%d;", item)
			} else {
				fmt.Fprintf(f, "k_%d;v_%d;", item, item)
			}
		}
		fmt.Fprintf(f, "\n")
		endCluster(f)
	}
	startCluster(f, i, "hashes")
	for _, item := range hashes {
		if _, ok1 := noValues[item]; ok1 {
			fmt.Fprintf(f, "k_%d;", item)
		} else {
			fmt.Fprintf(f, "k_%d;v_%d;", item, item)
		}
	}
	fmt.Fprintf(f, "\n")
	endCluster(f)
	endCluster(f)
	if err := f.Close(); err != nil {
		return err
	}
	cmd := exec.Command("dot", "-Tpng:gd", "-O", filename)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("error: %v, output: %s\n", err, output)
	}
	for _, f1 := range perBucketFiles {
		fmt.Fprintf(f1, "\n")
		endCluster(f1)
		endCluster(f1)
		if err := f1.Close(); err != nil {
			return err
		}
		cmd := exec.Command("dot", "-Tpng:gd", "-O", f1.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("error: %v, output: %s\n", err, output)
		}
	}
	return nil
}

func initialState1() error {
	fmt.Printf("Initial state 1\n")
	// Configure and generate a sample block chain
	var (
		memDb    = memorydb.New()
		db       = rawdb.NewDatabase(memDb)
		key, _   = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key1, _  = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		key2, _  = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		address  = crypto.PubkeyToAddress(key.PublicKey)
		address1 = crypto.PubkeyToAddress(key1.PublicKey)
		address2 = crypto.PubkeyToAddress(key2.PublicKey)
		theAddr  = common.Address{1}
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
		signer = types.HomesteadSigner{}
	)
	snapshotDb := memDb.MemCopy()
	genesis := gspec.MustCommit(db)
	genesisDb := rawdb.NewDatabase(memDb.MemCopy())
	engine := ethash.NewFaker()
	chainConfig, _, err := core.SetupGenesisBlock(db, gspec)
	if err != nil {
		return err
	}
	blockchain, err := core.NewBlockChain(db, &core.CacheConfig{
		TrieDirtyDisabled: true,
	}, chainConfig, engine, vm.Config{}, nil)
	if err != nil {
		return err
	}
	_ = blockchain.StateCache().TrieDB()
	// construct the first diff
	if err = stateDatabaseComparison(snapshotDb, memDb, 1); err != nil {
		return err
	}

	contractBackend := backends.NewSimulatedBackend(gspec.Alloc, gspec.GasLimit)
	transactOpts := bind.NewKeyedTransactor(key)
	transactOpts1 := bind.NewKeyedTransactor(key1)
	transactOpts2 := bind.NewKeyedTransactor(key2)

	var tokenContract *contracts.Token

	blocks, _ := core.GenerateChain(gspec.Config, genesis, engine, genesisDb, 8, func(i int, block *core.BlockGen) {
		var (
			tx  *types.Transaction
			txs []*types.Transaction
		)

		ctx := context.Background()

		switch i {
		case 0:
			tx, err = types.SignTx(types.NewTransaction(block.TxNonce(address), theAddr, big.NewInt(1000000000000000), 21000, new(big.Int), nil), signer, key)
			err = contractBackend.SendTransaction(ctx, tx)
			if err != nil {
				panic(err)
			}
		case 1:
			tx, err = types.SignTx(types.NewTransaction(block.TxNonce(address), theAddr, big.NewInt(1000000000000000), 21000, new(big.Int), nil), signer, key)
			err = contractBackend.SendTransaction(ctx, tx)
			if err != nil {
				panic(err)
			}
		case 2:
			_, tx, tokenContract, err = contracts.DeployToken(transactOpts, contractBackend, address1)
		case 3:
			tx, err = tokenContract.Mint(transactOpts1, address2, big.NewInt(10))
		case 4:
			tx, err = tokenContract.Transfer(transactOpts2, address, big.NewInt(3))
		case 5:
			// Muliple transactions sending small amounts of ether to various accounts
			var j uint64
			var toAddr common.Address
			nonce := block.TxNonce(address)
			for j = 1; j <= 32; j++ {
				binary.BigEndian.PutUint64(toAddr[:], j)
				tx, err = types.SignTx(types.NewTransaction(nonce, toAddr, big.NewInt(1000000000000000), 21000, new(big.Int), nil), signer, key)
				if err != nil {
					panic(err)
				}
				err = contractBackend.SendTransaction(ctx, tx)
				if err != nil {
					panic(err)
				}
				txs = append(txs, tx)
				nonce++
			}
		case 6:
			_, tx, tokenContract, err = contracts.DeployToken(transactOpts, contractBackend, address1)
			if err != nil {
				panic(err)
			}
			txs = append(txs, tx)
			tx, err = tokenContract.Mint(transactOpts1, address2, big.NewInt(100))
			if err != nil {
				panic(err)
			}
			txs = append(txs, tx)
			// Muliple transactions sending small amounts of ether to various accounts
			var j uint64
			var toAddr common.Address
			for j = 1; j <= 32; j++ {
				binary.BigEndian.PutUint64(toAddr[:], j)
				tx, err = tokenContract.Transfer(transactOpts2, toAddr, big.NewInt(1))
				if err != nil {
					panic(err)
				}
				txs = append(txs, tx)
			}
		case 7:
			var toAddr common.Address
			nonce := block.TxNonce(address)
			binary.BigEndian.PutUint64(toAddr[:], 4)
			tx, err = types.SignTx(types.NewTransaction(nonce, toAddr, big.NewInt(1000000000000000), 21000, new(big.Int), nil), signer, key)
			if err != nil {
				panic(err)
			}
			err = contractBackend.SendTransaction(ctx, tx)
			if err != nil {
				panic(err)
			}
			txs = append(txs, tx)
			binary.BigEndian.PutUint64(toAddr[:], 12)
			tx, err = tokenContract.Transfer(transactOpts2, toAddr, big.NewInt(1))
			if err != nil {
				panic(err)
			}
			txs = append(txs, tx)
		}

		if err != nil {
			panic(err)
		}
		if txs == nil && tx != nil {
			txs = append(txs, tx)
		}

		for _, tx := range txs {
			block.AddTx(tx)
		}
		contractBackend.Commit()
	})

	// BLOCK 1
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[0]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 2); err != nil {
		return err
	}
	// BLOCK 2
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[1]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 3); err != nil {
		return err
	}
	// BLOCK 3
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[2]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 4); err != nil {
		return err
	}
	// BLOCK 4
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[3]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 5); err != nil {
		return err
	}
	// BLOCK 5
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[4]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 6); err != nil {
		return err
	}
	// BLOCK 6
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[5]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 7); err != nil {
		return err
	}
	// BLOCK 7
	snapshotDb = memDb.MemCopy()
	if _, err = blockchain.InsertChain(types.Blocks{blocks[6]}); err != nil {
		return err
	}
	if err = stateDatabaseComparison(snapshotDb, memDb, 6); err != nil {
		return err
	}
	return nil
}
