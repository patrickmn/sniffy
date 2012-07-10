package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

var (
	// RFC 3280, 4.1.2.2: "Conformant CAs MUST NOT use serialNumber values longer than 20 octets."
	MaxSN    = "1461501637330902918203684832716283019655932542975" // 2^(8*20)-1 (base 10)
	maxSNInt *big.Int
)

func GetSN() (*big.Int, error) {
	if maxSNInt == nil {
		var success bool
		maxSNInt, success = new(big.Int).SetString(MaxSN, 10)
		if !success {
			return nil, fmt.Errorf("Could not set maximum cert SN")
		}
	}
	sn, err := rand.Int(rand.Reader, maxSNInt)
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve random integer for certificate SN: %s", err)
	}
	return sn, nil
}

func GetOrGenerateKeyPair(c, k, cn string, org []string, isCA bool, parent *x509.Certificate) (*tls.Certificate, error) {
	if _, err := os.Lstat(k); err != nil {
		sn, err := GetSN()
		if err != nil {
			return nil, err
		}
		GenerateRSAKeyPair(c, k, 1024, cn, org, sn, isCA, parent)
	}
	keypair, err := tls.LoadX509KeyPair(c, k)
	if err != nil {
		return nil, err
	}
	return &keypair, nil
}

func GenerateRSAKeyPair(c, k string, bits int, cn string, org []string, sn *big.Int, isCA bool, parent *x509.Certificate) error {
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return fmt.Errorf("Failed to generate private key: %v", err)
	}
	now := time.Now()
	if err != nil {
		return fmt.Errorf("Couldn't get new certificate serial number: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: org,
		},
		NotBefore:             now.Add(-2 * 24 * time.Hour).UTC(),
		NotAfter:              now.Add(time.Hour * 24 * 365 * 10).UTC(), // valid for 10 years.
		BasicConstraintsValid: isCA,
		IsCA:                  isCA,
		SubjectKeyId:          []byte{1, 2, 3, 4},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	if parent == nil {
		parent = &template
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, parent, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("Failed to create certificate: %s", err)
	}

	cOut, err := os.Create(c)
	if err != nil {
		return fmt.Errorf("Failed to open %s for writing: %v", c, err)
	}
	pem.Encode(cOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	cOut.Close()

	kOut, err := os.OpenFile(k, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Failed to open %s for writing: %v", k, err)
	}
	pem.Encode(kOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	kOut.Close()
	return nil
}
