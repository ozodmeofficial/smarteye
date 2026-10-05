// Package security provides the cryptographic pieces SmartEYE relies on:
// self-signed TLS material for the LAN WebSocket, and the pairing-code logic
// that stops a stranger's computer from joining or impersonating the server.
//
// SmartEYE runs on a local network, so we do not rely on a public CA. Instead
// the server mints a self-signed certificate on first run and clients trust any
// server that can prove knowledge of the shared pairing code (the "network
// code" the admin typed during install). The code is never sent in the clear:
// clients send a salted hash, and the server compares against its own.
package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CodeSalt is mixed into every pairing-code hash. It is a constant (not a
// secret) whose only job is domain-separation so a SmartEYE hash can never
// collide with a bare SHA-256 of the same string used elsewhere.
const CodeSalt = "smarteye/v1/netcode"

// HashNetCode returns the hex SHA-256 of the salted, normalized pairing code.
// Both peers compute this independently; the plaintext code never crosses the
// wire.
func HashNetCode(normalized string) string {
	h := sha256.Sum256([]byte(CodeSalt + ":" + normalized))
	return hex.EncodeToString(h[:])
}

// VerifyNetCode compares two code hashes in constant time.
func VerifyNetCode(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GenerateCode returns a human-friendly 6-digit pairing code, grouped as
// "NNN NNN" for display. The randomness is cryptographic.
func GenerateCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("security: generate code: %w", err)
	}
	s := fmt.Sprintf("%06d", n.Int64())
	return s[:3] + " " + s[3:], nil
}

// EnsureServerCert loads an existing certificate/key pair from dir, or creates
// a fresh self-signed pair valid for this host's LAN addresses. Returns a
// tls.Certificate ready to hand to a tls.Config.
func EnsureServerCert(dir string) (tls.Certificate, error) {
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	if fileExists(certPath) && fileExists(keyPath) {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err == nil {
			// Regenerate if expired.
			if leaf, perr := x509.ParseCertificate(cert.Certificate[0]); perr == nil {
				if time.Now().Before(leaf.NotAfter.Add(-24 * time.Hour)) {
					return cert, nil
				}
			} else {
				return cert, nil
			}
		}
	}

	cert, certPEM, keyPEM, err := newSelfSigned()
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, fmt.Errorf("security: write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("security: write key: %w", err)
	}
	return cert, nil
}

// newSelfSigned creates a fresh P-256 self-signed certificate covering
// localhost and every local IP, valid for 5 years.
func newSelfSigned() (tls.Certificate, []byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "SmartEYE Server", Organization: []string{"SmartEYE"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost", "smarteye.local"},
		IPAddresses:           localIPs(),
		IsCA:                  false,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	return cert, certPEM, keyPEM, nil
}

// localIPs returns all non-loopback IPv4/IPv6 addresses plus loopback, so the
// certificate is valid however the client reaches the server on the LAN.
func localIPs() []net.IP {
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			ips = append(ips, ipnet.IP)
		}
	}
	return ips
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
