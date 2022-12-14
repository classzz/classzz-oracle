package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/classzz/classzz-orace/config"
	"github.com/classzz/go-classzz-v2/accounts/abi/bind"
	"github.com/classzz/go-classzz-v2/accounts/keystore"
	"github.com/classzz/go-classzz-v2/common"
	"github.com/classzz/go-classzz-v2/console/prompt"
	"github.com/classzz/go-classzz-v2/core/types"
	"github.com/classzz/go-classzz-v2/crypto"
	"github.com/classzz/go-classzz-v2/czzclient"
	"github.com/classzz/go-classzz-v2/log"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"time"
)

type Candlestick struct {
	Elapsed string     `json:"elapsed"`
	Result  string     `json:"result"`
	Data    [][]string `json:"data"`
}

var (
	cfg           config.Config
	startInterval = 5 * time.Minute
)

func main() {

	// Load configuration file
	config.LoadConfig(&cfg, "")
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(true)))
	glogger.Verbosity(log.Lvl(cfg.DebugLevel))
	log.Root().SetHandler(glogger)
	privateKeys := loadSigningKey(cfg.PrivatePath, "")
	for _, v := range cfg.Coins {
		go send(v, privateKeys)
	}
	select {}
}

func send(coin config.Coins, privateKeys map[common.Address]*ecdsa.PrivateKey) {

	startTicker := time.NewTicker(startInterval)
	hourcount := 0
	for {
		select {
		case <-startTicker.C:
			hourcount++
			resp, err := http.Get(coin.Url)
			if err != nil {
				fmt.Println(err)
			}
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			var res Candlestick
			_ = json.Unmarshal(body, &res)

			sendCzz(privateKeys, res, common.HexToAddress(coin.CzzAddress), hourcount)
			sendEthf(privateKeys, res, common.HexToAddress(coin.EthfAddress), hourcount)
		}
	}
}

func sendCzz(privateKeys map[common.Address]*ecdsa.PrivateKey, res Candlestick, cAddress common.Address, hourcount int) {

	czzClient, err := czzclient.Dial("https://node.classzz.com")
	if err != nil {
		log.Error("NewClient", "err", err)
	}

	instance, err := NewAggregator(cAddress, czzClient)
	latestRoundData, err := instance.LatestRoundData(nil)

	index := 2
	datas := res.Data[len(res.Data)-1]
	for _, v := range privateKeys {
		log.Info("sendCzz", "latestRound", latestRoundData.RoundId, "index", index)
		rate, _ := big.NewFloat(0.0).SetString(datas[index])
		rateInt, _ := big.NewFloat(0).Mul(rate, big.NewFloat(100000000)).Int(nil)

		a := big.NewInt(0).Sub(rateInt, latestRoundData.Answer)
		b := a.Abs(a)
		c := b.Sub(b, big.NewInt(20))
		if c.Cmp(rateInt) <= 0 && hourcount < 12 {
			return
		}
		hourcount = 0
		sendTx(rateInt, uint32(latestRoundData.RoundId.Uint64())+1, v, instance, czzClient)
		index++
	}
}

func sendEthf(privateKeys map[common.Address]*ecdsa.PrivateKey, res Candlestick, cAddress common.Address, hourcount int) {

	czzClient, err := czzclient.Dial("https://rpc.etherfair.org")
	if err != nil {
		log.Error("NewClient", "err", err)
	}

	instance, err := NewAggregator(cAddress, czzClient)
	latestRoundData, err := instance.LatestRoundData(nil)

	index := 2
	datas := res.Data[len(res.Data)-1]
	for _, v := range privateKeys {
		log.Info("sendEthf", "latestRound", latestRoundData.RoundId, "index", index)

		rate, _ := big.NewFloat(0.0).SetString(datas[index])
		rateInt, _ := big.NewFloat(0).Mul(rate, big.NewFloat(100000000)).Int(nil)
		a := big.NewInt(0).Sub(rateInt, latestRoundData.Answer)
		b := a.Abs(a)
		c := b.Sub(b, big.NewInt(20))
		if c.Cmp(rateInt) <= 0 && hourcount < 12 {
			return
		}
		hourcount = 0
		sendTx(rateInt, uint32(latestRoundData.RoundId.Uint64())+1, v, instance, czzClient)
		index++
	}
}

func sendTx(rate *big.Int, latestRound uint32, privateKey *ecdsa.PrivateKey, aggregator *Aggregator, client *czzclient.Client) {

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Error("error casting public key to ECDSA")
		return
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.TODO(), fromAddress)
	if err != nil {
		log.Error("PendingNonceAt", "err", err)
		return
	}

	gasPrice, err := client.SuggestGasPrice(context.TODO())
	if err != nil {
		log.Error("SuggestGasPrice", "err", err)
		return
	}

	chainId, err := client.ChainID(context.TODO())
	if err != nil {
		log.Error("SuggestGasPrice", "err", err)
		return
	}

	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, chainId)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasPrice = gasPrice   // in wei

	tx, err := aggregator.Transmit(auth, latestRound, rate)
	if err != nil {
		log.Error("Transmit", "err", err)
		return
	} else {
		log.Info("tx", "hash", tx.Hash())
	}
	check(tx, client)
}

func check(checkTx *types.Transaction, client *czzclient.Client) {
	count := 0
	for {
		count++
		receipt, err := client.TransactionReceipt(context.TODO(), checkTx.Hash())

		if receipt == nil {

			if err != nil {
				log.Error("TransactionReceipt", "err", err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Error("TransactionReceipt", "Receipt", receipt)
			time.Sleep(5 * time.Second)
			if count >= 10 {
				log.Warn("Please command query later.", "txHash", checkTx.Hash().String())
			}
			continue
		}

		log.Info("Please success ", "txHash", checkTx.Hash().String())
		break
	}
}

func loadSigningKey(keyfiles []string, password string) map[common.Address]*ecdsa.PrivateKey {
	PrivateKey := map[common.Address]*ecdsa.PrivateKey{}
	if password == "" {
		password, _ = prompt.Stdin.PromptPassword("Please enter the password :")
	}
	for _, v := range keyfiles {
		keyjson, err := ioutil.ReadFile(v)
		if err != nil {
			log.Error("failed to read the keyfile at", "keyfile", v, "err", err)
			os.Exit(0)
		}

		key, err := keystore.DecryptKey(keyjson, password)
		if err != nil {
			log.Error("error decrypting ", "err", err)
			os.Exit(0)
		}

		from := crypto.PubkeyToAddress(key.PrivateKey.PublicKey)
		PrivateKey[from] = key.PrivateKey
	}
	return PrivateKey
}
