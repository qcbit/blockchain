package cmd

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/qcbit/blockchain/foundation/blockchain/database"
)

var (
	url   string
	nonce uint64
	from  string
	to    string
	value uint64
	tip   uint64
	data  []byte
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send transaction",
	Run:   sendRun,
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVarP(&url, "url", "w", "http://localhost:8080", "URL of the node.")
	sendCmd.Flags().Uint64VarP(&nonce, "nonce", "n", 0, "Transaction ID.")
	sendCmd.Flags().StringVarP(&from, "from", "f", "", "Sender.")
	sendCmd.Flags().StringVarP(&to, "to", "t", "", "Recipient.")
	sendCmd.Flags().Uint64VarP(&value, "value", "v", 0, "Send amount.")
	sendCmd.Flags().Uint64VarP(&tip, "tip", "c", 0, "Tip amount.")
	sendCmd.Flags().BytesHexVarP(&data, "data", "d", nil, "Data payload.")
}

func sendRun(cmd *cobra.Command, args []string) {
	privateKey, err := crypto.LoadECDSA(getPrivateKeyPath())
	if err != nil {
		log.Fatal(err)
	}

	sendWithDetails(privateKey)
}

func sendWithDetails(privateKey *ecdsa.PrivateKey) {
	fromAccount, err := database.ToAccountID(from)
	if err != nil {
		log.Fatal(err)
	}

	toAccount, err := database.ToAccountID(to)
	if err != nil {
		log.Fatal(err)
	}

	const chainID = 1
	tx, err := database.NewTx(chainID, fromAccount, toAccount, value, nonce, tip, data)
	if err != nil {
		log.Fatal(err)
	}

	signedTx, err := tx.Sign(privateKey)
	if err != nil {
		log.Fatal(err)
	}

	data, err := json.Marshal(signedTx)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/v1/tx/submit", url), "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}
