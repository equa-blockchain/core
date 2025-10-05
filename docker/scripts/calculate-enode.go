package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/equa/go-equa/crypto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: calculate-enode <private_key_hex>")
		os.Exit(1)
	}

	privKeyHex := os.Args[1]
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		fmt.Printf("Error decoding private key: %v\n", err)
		os.Exit(1)
	}

	privKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		fmt.Printf("Error converting to ECDSA: %v\n", err)
		os.Exit(1)
	}

	pubKey := privKey.Public().(*ecdsa.PublicKey)
	pubKeyBytes := crypto.FromECDSAPub(pubKey)

	// Remove o prefixo 0x04 (primeiro byte que indica formato não comprimido)
	nodeID := pubKeyBytes[1:]

	fmt.Printf("Public Key: %x\n", pubKeyBytes)
	fmt.Printf("Node ID: %x\n", nodeID)
	fmt.Printf("\nEnode (use com IP correto):\n")
	fmt.Printf("enode://%x@IP:PORT\n", nodeID)
}

