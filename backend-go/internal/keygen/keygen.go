package keygen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gowave/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	s := &Service{db: db}
	_ = s.EnsureKeygen()
	return s
}

func (s *Service) EnsureKeygen() error {
	var count int64
	s.db.Model(&database.Keygen{}).Count(&count)
	if count > 0 {
		return nil
	}

	caCert, caKey, clientCert, clientKey, err := generateMasterCerts()
	if err != nil {
		return fmt.Errorf("failed to generate master certs: %w", err)
	}

	pubKey, privKey, err := generateJwtKeypair()
	if err != nil {
		return fmt.Errorf("failed to generate JWT keypair: %w", err)
	}

	record := &database.Keygen{
		UUID:       uuid.NewString(),
		CACert:     caCert,
		CAKey:      caKey,
		ClientCert: clientCert,
		ClientKey:  clientKey,
		PubKey:     pubKey,
		PrivKey:    privKey,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	return s.db.Create(record).Error
}

func (s *Service) GetMasterKeygen() (*database.Keygen, error) {
	var k database.Keygen
	err := s.db.Order("created_at asc").First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.EnsureKeygen(); err != nil {
				return nil, err
			}
			return s.GetMasterKeygen()
		}
		return nil, err
	}
	return &k, nil
}

func (s *Service) GenerateNodeSecretKey() (string, error) {
	k, err := s.GetMasterKeygen()
	if err != nil {
		return "", err
	}

	nodeCertPem, nodeKeyPem, err := generateNodeCert(k.CACert, k.CAKey)
	if err != nil {
		return "", fmt.Errorf("failed generating node cert: %w", err)
	}

	payload := map[string]string{
		"caCertPem":    k.CACert,
		"jwtPublicKey": k.PubKey,
		"nodeCertPem":  nodeCertPem,
		"nodeKeyPem":   nodeKeyPem,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(payloadJSON), nil
}

func generateMasterCerts() (caCertPem, caKeyPem, clientCertPem, clientKeyPem string, err error) {
	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", "", err
	}

	serialNum, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	caTemplate := &x509.Certificate{
		SerialNumber: serialNum,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("rw-ca-%s", uuid.NewString()[:8]),
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		return "", "", "", "", err
	}

	caCertBuf := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	caKeyBytes, err := x509.MarshalPKCS8PrivateKey(caPriv)
	if err != nil {
		return "", "", "", "", err
	}
	caKeyBuf := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: caKeyBytes})

	clientPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", "", err
	}

	clientSerial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	clientTemplate := &x509.Certificate{
		SerialNumber: clientSerial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("rw-client-%s", uuid.NewString()[:8]),
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientBytes, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientPriv.PublicKey, caPriv)
	if err != nil {
		return "", "", "", "", err
	}

	clientCertBuf := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientBytes})
	clientKeyBytes, err := x509.MarshalPKCS8PrivateKey(clientPriv)
	if err != nil {
		return "", "", "", "", err
	}
	clientKeyBuf := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyBytes})

	return string(caCertBuf), string(caKeyBuf), string(clientCertBuf), string(clientKeyBuf), nil
}

func generateNodeCert(caCertPem, caKeyPem string) (nodeCertPem, nodeKeyPem string, err error) {
	caCertBlock, _ := pem.Decode([]byte(caCertPem))
	if caCertBlock == nil {
		return "", "", errors.New("failed decoding ca cert pem")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return "", "", err
	}

	caKeyBlock, _ := pem.Decode([]byte(caKeyPem))
	if caKeyBlock == nil {
		return "", "", errors.New("failed decoding ca key pem")
	}
	caKeyAny, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return "", "", err
	}

	nodePriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	nodeSerial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	nodeTemplate := &x509.Certificate{
		SerialNumber: nodeSerial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("rw-node-%s", uuid.NewString()[:8]),
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(3, 0, 0),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	nodeBytes, err := x509.CreateCertificate(rand.Reader, nodeTemplate, caCert, &nodePriv.PublicKey, caKeyAny)
	if err != nil {
		return "", "", err
	}

	nodeCertBuf := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: nodeBytes})
	nodeKeyBytes, err := x509.MarshalPKCS8PrivateKey(nodePriv)
	if err != nil {
		return "", "", err
	}
	nodeKeyBuf := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: nodeKeyBytes})

	return string(nodeCertBuf), string(nodeKeyBuf), nil
}

func generateJwtKeypair() (pubKeyPem, privKeyPem string, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	privBuf := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubBuf := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	return string(pubBuf), string(privBuf), nil
}
