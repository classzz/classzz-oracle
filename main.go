package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
)

type Candlestick struct {
	Elapsed string     `json:"elapsed"`
	Result  string     `json:"result"`
	Data    [][]string `json:"data"`
}

func main() {

	resp, err := http.Get("https://data.gateapi.io/api2/1/candlestick2/czz_usdt?group_sec=900&range_hour=4")
	if err != nil {
		fmt.Println(err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var res Candlestick
	_ = json.Unmarshal(body, &res)
	fmt.Println(res)

	prv := loadSigningKey([]string{"加密私钥位置"})

	// 计算价格
	rate := big.NewInt(0)

	send(rate, prv[0])
}

func loadSigningKey(keyfiles []string) []*ecdsa.PrivateKey {
	PrivateKey := []*ecdsa.PrivateKey{}
	password, _ := prompt.Stdin.PromptPassword("Please enter the password :")
	for _, v := range keyfiles {
		keyjson, err := ioutil.ReadFile(v)
		if err != nil {
			log.Error("failed to read the keyfile at", "keyfile", v, "err", err)
		}

		key, err := keystore.DecryptKey(keyjson, password)
		if err != nil {
			log.Error("error decrypting ", "err", err)
		}

		priKey := key.PrivateKey
		from := crypto.PubkeyToAddress(priKey.PublicKey)
		PrivateKey = append(PrivateKey, key.PrivateKey)
		fmt.Println("address ", from.Hex(), "key", hex.EncodeToString(priKey.PublicKey.X.Bytes()))
	}
	return PrivateKey
}

func send(rate *big.Int, privateKey *ecdsa.PrivateKey) {

	cAddress := common.HexToAddress("0xbcF031727072038370B8F4Cb27A3802851850209")
	czzClient, err := czzclient.Dial("https://node.classzz.com")
	if err != nil {
		log.Error("NewClient", "err", err)
	}

	instance, err := NewUsdz(cAddress, czzClient)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := czzClient.PendingNonceAt(context.TODO(), fromAddress)
	if err != nil {
		fmt.Println(err)
	}

	gasPrice, err := czzClient.SuggestGasPrice(context.TODO())
	if err != nil {
		log.Error("SuggestGasPrice", "err", err)
	}

	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(61))
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasPrice = gasPrice   // in wei

	tx, err := instance.ConfigCurCzzUsdzRate(auth, rate)
	if err != nil {
		fmt.Println("err", err)
	} else {
		log.Info("tx", "hash", tx.Hash())
	}

}
