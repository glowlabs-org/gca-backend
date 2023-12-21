package glow

import (
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// PublicKey represents a 32-byte public key.
type PublicKey [32]byte

// PrivateKey represents a 32-byte private key.
type PrivateKey [32]byte

// Signature represents a 64-byte signature.
type Signature [64]byte

// GenerateKeyPair generates a new ECDSA private and public key pair.
// The function panics if there is an error during key generation.
// It also makes sure that the public key has the prefix 0x02.
func GenerateKeyPair() (PublicKey, PrivateKey) {
	for i := 0; i < 500; i++ {
		// Generate an ECDSA private key.
		// The function panics if an error occurs.
		privateKeyECDSA, err := crypto.GenerateKey()
		if err != nil {
			panic("Failed to generate private key: " + err.Error())
		}
		var privateKey PrivateKey
		copy(privateKey[:], crypto.FromECDSA(privateKeyECDSA))

		// Obtain the compressed public key.
		publicKeyCompressed := crypto.CompressPubkey(&privateKeyECDSA.PublicKey)

		// Check if the public key prefix is 0x02.
		// If yes, proceed; otherwise, generate a new key pair.
		if publicKeyCompressed[0] == 0x02 {
			var publicKey PublicKey
			copy(publicKey[:], publicKeyCompressed[1:]) // Skip the first byte (0x02 prefix)
			/*
				fmt.Printf("ADDR: %x\n", crypto.PubkeyToAddress(privateKeyECDSA.PublicKey))
				fmt.Printf("PUB: %x\n", publicKey)
				addr, err := PubKeyToAddr(publicKey)
				if err != nil {
					panic("consistency error with key generation")
				}
				fmt.Printf("ADDR2: %s\n", addr)
			*/
			// fmt.Printf("PRIV: %x\n", privateKey)
			return publicKey, privateKey
		}
	}
	panic("did not generate a good key in 500 attempts")
}

// Sign generates an Ethereum signature for given data using a private key.
func Sign(data []byte, privateKey PrivateKey) Signature {
	privateKeyECDSA, err := crypto.ToECDSA(privateKey[:])
	if err != nil {
		panic(err)
	}
	hash := crypto.Keccak256Hash(data)
	sig, err := crypto.Sign(hash.Bytes(), privateKeyECDSA)
	if err != nil {
		panic(err)
	}
	var signature Signature
	copy(signature[:], sig[:64])
	return signature
}

// Verify checks the Ethereum signature for given data and a public key.
func Verify(publicKey PublicKey, data []byte, signature Signature) bool {
	// Add back the 0x02 prefix to the public key.
	publicKey33 := append([]byte{0x02}, publicKey[:]...)

	// Decompress the public key to get the ECDSA public key, then verify.
	publicKeyECDSA, err := crypto.DecompressPubkey(publicKey33)
	if err != nil {
		return false
	}
	hash := crypto.Keccak256Hash(data)
	return crypto.VerifySignature(crypto.FromECDSAPub(publicKeyECDSA), hash.Bytes(), signature[:])
}

// PubKeyToAddr will convert a PublicKey to its corresponding Ethereum Address.
func PubKeyToAddr(pk PublicKey) (string, error) {
	// Decompress the public key
	var compressedKey [33]byte
	compressedKey[0] = 2
	copy(compressedKey[1:], pk[:])
	ecdsaKey, err := crypto.DecompressPubkey(compressedKey[:])
	if err != nil {
		return "", fmt.Errorf("unable to decompress pubkey: %v", err)
	}

	// Convert to ECDSA public key
	address := crypto.PubkeyToAddress(*ecdsaKey)
	return address.String(), nil
}
