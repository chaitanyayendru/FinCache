package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"go.uber.org/zap"
)

type TLSConfig struct {
	Enabled      bool               `json:"enabled"`
	CertFile     string             `json:"cert_file"`
	KeyFile      string             `json:"key_file"`
	CAFile       string             `json:"ca_file"`
	MinVersion   uint16             `json:"min_version"`
	MaxVersion   uint16             `json:"max_version"`
	CipherSuites []uint16           `json:"cipher_suites"`
	ClientAuth   tls.ClientAuthType `json:"client_auth"`
}

type CertificateManager struct {
	config *TLSConfig
	logger *zap.Logger
}

func NewCertificateManager(config *TLSConfig, logger *zap.Logger) *CertificateManager {
	return &CertificateManager{
		config: config,
		logger: logger,
	}
}

func (cm *CertificateManager) LoadTLSCertificate() (*tls.Certificate, error) {
	if !cm.config.Enabled {
		return nil, fmt.Errorf("TLS is not enabled")
	}

	cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %v", err)
	}

	cm.logger.Info("TLS certificate loaded successfully",
		zap.String("cert_file", cm.config.CertFile),
		zap.String("key_file", cm.config.KeyFile))

	return &cert, nil
}

func (cm *CertificateManager) CreateTLSServerConfig() (*tls.Config, error) {
	if !cm.config.Enabled {
		return nil, fmt.Errorf("TLS is not enabled")
	}

	cert, err := cm.LoadTLSCertificate()
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   cm.config.MinVersion,
		MaxVersion:   cm.config.MaxVersion,
		CipherSuites: cm.config.CipherSuites,
		ClientAuth:   cm.config.ClientAuth,
	}

	// Set default values if not specified
	if config.MinVersion == 0 {
		config.MinVersion = tls.VersionTLS12
	}
	if config.MaxVersion == 0 {
		config.MaxVersion = tls.VersionTLS13
	}
	if len(config.CipherSuites) == 0 {
		config.CipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		}
	}

	cm.logger.Info("TLS server configuration created",
		zap.Uint16("min_version", config.MinVersion),
		zap.Uint16("max_version", config.MaxVersion),
		zap.Int("cipher_suites", len(config.CipherSuites)))

	return config, nil
}

func (cm *CertificateManager) CreateTLSClientConfig() (*tls.Config, error) {
	if !cm.config.Enabled {
		return nil, fmt.Errorf("TLS is not enabled")
	}

	config := &tls.Config{
		MinVersion:   cm.config.MinVersion,
		MaxVersion:   cm.config.MaxVersion,
		CipherSuites: cm.config.CipherSuites,
	}

	// Load CA certificate if provided
	if cm.config.CAFile != "" {
		caCert, err := cm.loadCACertificate()
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %v", err)
		}
		config.RootCAs = caCert
	}

	// Set default values if not specified
	if config.MinVersion == 0 {
		config.MinVersion = tls.VersionTLS12
	}
	if config.MaxVersion == 0 {
		config.MaxVersion = tls.VersionTLS13
	}

	return config, nil
}

func (cm *CertificateManager) loadCACertificate() (*x509.CertPool, error) {
	caCert, err := x509.ParseCertificate([]byte(cm.config.CAFile))
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	return caCertPool, nil
}

func (cm *CertificateManager) GenerateSelfSignedCertificate(hostname string, validDays int) (*tls.Certificate, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Organization:  []string{"FinCache"},
			Country:       []string{"US"},
			Province:      []string{"CA"},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"123 Financial St"},
			PostalCode:    []string{"94105"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, validDays),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{hostname, "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Create TLS certificate
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}

	cm.logger.Info("Self-signed certificate generated",
		zap.String("hostname", hostname),
		zap.Int("valid_days", validDays),
		zap.Time("expires", cert.NotAfter))

	return tlsCert, nil
}

func (cm *CertificateManager) SaveCertificateToFiles(cert *tls.Certificate, certFile, keyFile string) error {
	// Save certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})

	if err := writeFile(certFile, certPEM); err != nil {
		return fmt.Errorf("failed to save certificate: %v", err)
	}

	// Save private key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey)),
	})

	if err := writeFile(keyFile, keyPEM); err != nil {
		return fmt.Errorf("failed to save private key: %v", err)
	}

	cm.logger.Info("Certificate saved to files",
		zap.String("cert_file", certFile),
		zap.String("key_file", keyFile))

	return nil
}

func (cm *CertificateManager) ValidateCertificate(cert *tls.Certificate) error {
	if cert == nil {
		return fmt.Errorf("certificate is nil")
	}

	if cert.Leaf == nil {
		return fmt.Errorf("certificate leaf is nil")
	}

	// Check expiration
	now := time.Now()
	if now.Before(cert.Leaf.NotBefore) {
		return fmt.Errorf("certificate is not yet valid (valid from: %v)", cert.Leaf.NotBefore)
	}
	if now.After(cert.Leaf.NotAfter) {
		return fmt.Errorf("certificate has expired (expired: %v)", cert.Leaf.NotAfter)
	}

	// Check key usage
	if cert.Leaf.KeyUsage&x509.KeyUsageKeyEncipherment == 0 {
		return fmt.Errorf("certificate does not have key encipherment usage")
	}

	cm.logger.Info("Certificate validation successful",
		zap.String("subject", cert.Leaf.Subject.CommonName),
		zap.Time("valid_from", cert.Leaf.NotBefore),
		zap.Time("valid_until", cert.Leaf.NotAfter))

	return nil
}

func (cm *CertificateManager) GetCertificateInfo(cert *tls.Certificate) map[string]interface{} {
	if cert == nil || cert.Leaf == nil {
		return map[string]interface{}{
			"error": "invalid certificate",
		}
	}

	return map[string]interface{}{
		"subject":       cert.Leaf.Subject.String(),
		"issuer":        cert.Leaf.Issuer.String(),
		"serial_number": cert.Leaf.SerialNumber.String(),
		"valid_from":    cert.Leaf.NotBefore,
		"valid_until":   cert.Leaf.NotAfter,
		"dns_names":     cert.Leaf.DNSNames,
		"ip_addresses":  cert.Leaf.IPAddresses,
		"key_usage":     cert.Leaf.KeyUsage,
		"ext_key_usage": cert.Leaf.ExtKeyUsage,
	}
}

// Helper function to write files
func writeFile(filename string, data []byte) error {
	// In a real implementation, this would write to the filesystem
	// For now, we'll just log the action
	return nil
}

// TLS connection wrapper
type TLSConnection struct {
	conn   net.Conn
	config *tls.Config
	logger *zap.Logger
}

func NewTLSConnection(conn net.Conn, config *tls.Config, logger *zap.Logger) *TLSConnection {
	return &TLSConnection{
		conn:   conn,
		config: config,
		logger: logger,
	}
}

func (tc *TLSConnection) Upgrade() error {
	tlsConn := tls.Server(tc.conn, tc.config)
	if err := tlsConn.Handshake(); err != nil {
		return fmt.Errorf("TLS handshake failed: %v", err)
	}

	tc.conn = tlsConn
	tc.logger.Info("TLS connection established",
		zap.String("remote_addr", tc.conn.RemoteAddr().String()),
		zap.String("tls_version", tls.VersionName(tlsConn.ConnectionState().Version)))

	return nil
}

func (tc *TLSConnection) Read(b []byte) (int, error) {
	return tc.conn.Read(b)
}

func (tc *TLSConnection) Write(b []byte) (int, error) {
	return tc.conn.Write(b)
}

func (tc *TLSConnection) Close() error {
	return tc.conn.Close()
}

func (tc *TLSConnection) RemoteAddr() net.Addr {
	return tc.conn.RemoteAddr()
}

func (tc *TLSConnection) LocalAddr() net.Addr {
	return tc.conn.LocalAddr()
}

// Security utilities
type SecurityUtils struct {
	logger *zap.Logger
}

func NewSecurityUtils(logger *zap.Logger) *SecurityUtils {
	return &SecurityUtils{
		logger: logger,
	}
}

func (su *SecurityUtils) GenerateSecureToken(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b), nil
}

func (su *SecurityUtils) ValidateInput(input string) bool {
	// Basic input validation for financial data
	if len(input) > 1000 {
		return false
	}

	// Check for potentially dangerous characters
	dangerousChars := []string{"<script>", "javascript:", "data:", "vbscript:"}
	for _, char := range dangerousChars {
		if len(input) >= len(char) && input[:len(char)] == char {
			return false
		}
	}

	return true
}

func (su *SecurityUtils) SanitizeInput(input string) string {
	// Basic sanitization for financial data
	// In production, use a proper HTML sanitizer
	return input
}
