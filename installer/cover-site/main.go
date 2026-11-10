// cover-site serves a pinned, self-contained game as a loopback-only REALITY
// target. It reads the panel's certificate directly, including after renewal.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//go:embed assets/*.html
var assets embed.FS

const version = "4178b08d569372a1492878d73c6018a90f564e5b"

var games = []string{"melon", "mosswing"}
var checksums = map[string]string{
	"melon":    "4f6ac30063d24fad963b166e444744b710f730afce4da79e68eb49def01e6b81",
	"mosswing": "3efe39af37efbdbe92c2310a321d410b34787cc329f2837e716a9e0d2975d2e3",
}

type gameState struct {
	Game    string `json:"game"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

func selectGame(path string, reroll bool) (gameState, error) {
	var state gameState
	data, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(data, &state) != nil || checksums[state.Game] == "" {
			return state, fmt.Errorf("invalid stored game state")
		}
	} else if !os.IsNotExist(err) {
		return state, err
	}
	if state.Game == "" || reroll {
		options := []string{}
		for _, name := range games {
			if !reroll || name != state.Game {
				options = append(options, name)
			}
		}
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(options))))
		if err != nil {
			return state, err
		}
		state.Game = options[i.Int64()]
	}
	state.Version = version
	state.SHA256 = checksums[state.Game]
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return state, err
	}
	encoded, _ := json.MarshalIndent(state, "", "  ")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".game-*")
	if err != nil {
		return state, err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(encoded)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	return state, err
}

func gameHandler(state gameState) (http.Handler, error) {
	data, err := assets.ReadFile("assets/" + state.Game + ".html")
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != checksums[state.Game] {
		return nil, fmt.Errorf("game checksum mismatch")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("Content-Security-Policy", "default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("ETag", `"`+state.SHA256+`"`)
		if r.Header.Get("If-None-Match") == `"`+state.SHA256+`"` {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == "GET" {
			_, _ = w.Write(data)
		}
	}), nil
}

type certificateLoader struct {
	mu                          sync.Mutex
	certPath, keyPath, identity string
	cert                        *tls.Certificate
	lastWarning                 time.Time
}

// Reload on every new handshake: panel renewals can replace files atomically.
// A transient mismatch during renewal keeps the last still-valid certificate.
func (l *certificateLoader) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cert, err := tls.LoadX509KeyPair(l.certPath, l.keyPath)
	if err == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			err = cert.Leaf.VerifyHostname(l.identity)
		}
		if err == nil && (now.Before(cert.Leaf.NotBefore) || !now.Before(cert.Leaf.NotAfter)) {
			err = fmt.Errorf("certificate not currently valid")
		}
	}
	if err == nil {
		l.cert = &cert
		return l.cert, nil
	}
	if l.cert != nil && now.Before(l.cert.Leaf.NotAfter) {
		if now.Sub(l.lastWarning) > time.Minute {
			log.Print("certificate reload failed; retaining previous valid certificate")
			l.lastWarning = now
		}
		return l.cert, nil
	}
	return nil, fmt.Errorf("panel certificate unavailable, expired or identity mismatch")
}

func main() {
	listen := flag.String("listen", "127.0.0.1:8443", "loopback TLS listener")
	cert := flag.String("cert", "/root/cert/ip/fullchain.pem", "panel certificate file")
	key := flag.String("key", "/root/cert/ip/privkey.pem", "panel key file")
	identity := flag.String("identity", "", "certificate IP/domain identity")
	statePath := flag.String("state", "/var/lib/ecacc-cover/state.json", "persistent game selection")
	prepare := flag.Bool("prepare", false, "select game and print receipt without listening")
	check := flag.Bool("check", false, "verify certificate/identity and embedded games without changing state or listening")
	reroll := flag.Bool("reroll", false, "choose a different game; use with -prepare")
	flag.Parse()
	if *check {
		if *identity == "" || *prepare || *reroll {
			log.Fatal("check requires identity and cannot be combined with prepare/reroll")
		}
		loader := &certificateLoader{certPath: *cert, keyPath: *key, identity: *identity}
		if _, err := loader.get(nil); err != nil {
			log.Fatal(err)
		}
		for _, name := range games {
			if _, err := gameHandler(gameState{Game: name}); err != nil {
				log.Fatal(err)
			}
		}
		fmt.Println("Certificate identity, key and embedded game checks passed")
		return
	}
	if *reroll && !*prepare {
		log.Fatal("reroll requires prepare")
	}
	state, err := selectGame(*statePath, *reroll)
	if err != nil {
		log.Fatal(err)
	}
	handler, err := gameHandler(state)
	if err != nil {
		log.Fatal(err)
	}
	if *prepare {
		_ = json.NewEncoder(os.Stdout).Encode(state)
		return
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("listen must use a literal loopback IP")
	}
	loader := &certificateLoader{certPath: *cert, keyPath: *key, identity: *identity}
	if *identity == "" {
		log.Fatal("identity is required")
	}
	if _, err = loader.get(nil); err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: loader.get}}
	log.Printf("serving %s on %s", state.Game, *listen)
	log.Fatal(server.ListenAndServeTLS("", ""))
}
