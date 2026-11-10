package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCertificate(t *testing.T, dir string, serial int64) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "fixture"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("192.0.2.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPanelCertificateRenewalReloadsWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	writeCertificate(t, dir, 1)
	l := &certificateLoader{certPath: filepath.Join(dir, "cert.pem"), keyPath: filepath.Join(dir, "key.pem"), identity: "192.0.2.1"}
	a, err := l.get(nil)
	if err != nil {
		t.Fatal(err)
	}
	writeCertificate(t, dir, 2)
	b, err := l.get(nil)
	if err != nil || a.Leaf.SerialNumber.Int64() != 1 || b.Leaf.SerialNumber.Int64() != 2 {
		t.Fatal("renewal not loaded")
	}
	_ = os.WriteFile(l.keyPath, []byte("incomplete renewal"), 0600)
	b, err = l.get(nil)
	if err != nil || b.Leaf.SerialNumber.Int64() != 2 {
		t.Fatal("partial renewal discarded last valid certificate")
	}
	bad := &certificateLoader{certPath: l.certPath, keyPath: l.keyPath, identity: "192.0.2.2"}
	if _, err = bad.get(nil); err == nil {
		t.Fatal("unusable initial certificate accepted")
	}
}

func TestRandomChoicePersistsAndRerollChangesGame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	a, err := selectGame(path, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := selectGame(path, false)
	if err != nil || a != b {
		t.Fatal("game changed on restart")
	}
	c, err := selectGame(path, true)
	if err != nil || c.Game == a.Game {
		t.Fatal("reroll did not change game")
	}
	for _, game := range []gameState{a, c} {
		h, err := gameHandler(game)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "https://192.0.2.1/", nil))
		if w.Code != 200 || w.Body.Len() < 100000 || w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
			t.Fatal("game not served")
		}
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "https://192.0.2.1/panel/", nil))
		if w.Code != 404 {
			t.Fatal("unexpected route exposed")
		}
	}
}
