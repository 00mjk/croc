package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
)

// Encryption stores the data
type Encryption struct {
	Encrypted, Salt, IV []byte
}

// Encrypt will generate an encryption
func Encrypt(plaintext []byte, passphrase []byte, dontencrypt ...bool) Encryption {
	if len(dontencrypt) > 0 && dontencrypt[0] {
		return Encryption{
			Encrypted: plaintext,
			Salt:      []byte("salt"),
			IV:        []byte("iv"),
		}
	}
	key, saltBytes := deriveKey(passphrase, nil)
	ivBytes := make([]byte, 12)
	// http://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf
	// Section 8.2
	rand.Read(ivBytes)
	b, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(b)
	encrypted := aesgcm.Seal(nil, ivBytes, plaintext, nil)
	return Encryption{
		Encrypted: encrypted,
		Salt:      saltBytes,
		IV:        ivBytes,
	}
}

// Decrypt an encryption
func (e Encryption) Decrypt(passphrase []byte, dontencrypt ...bool) (plaintext []byte, err error) {
	if len(dontencrypt) > 0 && dontencrypt[0] {
		return e.Encrypted, nil
	}
	key, _ := deriveKey(passphrase, e.Salt)
	b, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(b)
	plaintext, err = aesgcm.Open(nil, e.IV, e.Encrypted, nil)
	return
}

func deriveKey(passphrase []byte, salt []byte) ([]byte, []byte) {
	if salt == nil {
		salt = make([]byte, 8)
		// http://www.ietf.org/rfc/rfc2898.txt
		// Salt.
		rand.Read(salt)
	}
	return pbkdf2.Key([]byte(passphrase), salt, 100, 32, sha256.New), salt
}
