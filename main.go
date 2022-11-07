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
	"github.com/classzz/go-classzz-v2/crypto"
	"github.com/classzz/go-classzz-v2/czzclient"
	"github.com/classzz/go-classzz-v2/log"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
)

type Candlestick struct {
	Elapsed string     `json:"elapsed"`
	Result  string     `json:"result"`
	Data    [][]string `json:"data"`
}

var (
	cfg      config.Config
	baseUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(14), nil)
)

func main() {

	// Load configuration file
	config.LoadConfig(&cfg, "")

	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(true)))
	glogger.Verbosity(log.Lvl(cfg.DebugLevel))
	log.Root().SetHandler(glogger)

	privateKeys := loadSigningKey(cfg.PrivatePath, "")

	resp, err := http.Get("https://data.gateapi.io/api2/1/candlestick2/ethf_usdt?group_sec=900&range_hour=4")
	if err != nil {
		fmt.Println(err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var res Candlestick
	_ = json.Unmarshal(body, &res)
	fmt.Println(res)
	sendCzz(privateKeys, res)

}

func sendCzz(privateKeys map[common.Address]*ecdsa.PrivateKey, res Candlestick) {

	cAddress := common.HexToAddress("0xbcF031727072038370B8F4Cb27A3802851850209")
	czzClient, err := czzclient.Dial("https://node.classzz.com")
	if err != nil {
		log.Error("NewClient", "err", err)
	}

	instance, err := NewAggregator(cAddress, czzClient)
	latestRound, err := instance.LatestRound(nil)

	index := 2
	datas := res.Data[len(res.Data)-1]
	for _, v := range privateKeys {
		rate, _ := big.NewFloat(0.0).SetString(datas[index])
		rateInt, _ := big.NewFloat(0).Mul(rate, big.NewFloat(100000000)).Int(nil)
		sendTx(rateInt, uint32(latestRound.Uint64())+1, v, instance, czzClient)
		index++
	}
}

func sendTx(rate *big.Int, latestRound uint32, privateKey *ecdsa.PrivateKey, aggregator *Aggregator, client *czzclient.Client) {

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.TODO(), fromAddress)
	if err != nil {
		fmt.Println(err)
	}

	gasPrice, err := client.SuggestGasPrice(context.TODO())
	if err != nil {
		log.Error("SuggestGasPrice", "err", err)
	}

	chainId, err := client.ChainID(context.TODO())
	if err != nil {
		log.Error("SuggestGasPrice", "err", err)
	}

	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, chainId)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasPrice = gasPrice   // in wei

	tx, err := aggregator.Transmit(auth, latestRound, rate)
	if err != nil {
		fmt.Println("err", err)
	} else {
		log.Info("tx", "hash", tx.Hash())
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
