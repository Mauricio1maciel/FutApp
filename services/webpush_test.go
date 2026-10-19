package services

import (
	"App-Futebol/database"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// browser faz o papel do navegador: as chaves da inscrição e a decifragem da mensagem
type browser struct {
	priv *ecdh.PrivateKey
	auth []byte
}

func newBrowser(t *testing.T) browser {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	return browser{priv: priv, auth: auth}
}

func (b browser) target(endpoint string) database.PushTarget {
	return database.PushTarget{
		Token: endpoint, Platform: "web",
		P256dh: base64.RawURLEncoding.EncodeToString(b.priv.PublicKey().Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(b.auth),
	}
}

// decrypt segue a RFC 8291 (aes128gcm), como o navegador faz ao receber
func (b browser) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()
	if len(body) < 21 {
		t.Fatalf("corpo curto demais: %d bytes", len(body))
	}
	salt := body[:16]
	_ = binary.BigEndian.Uint32(body[16:20]) // tamanho do registro
	idlen := int(body[20])
	serverPub := body[21 : 21+idlen]
	ciphertext := body[21+idlen:]

	asPub, err := ecdh.P256().NewPublicKey(serverPub)
	if err != nil {
		t.Fatalf("chave do servidor inválida: %v", err)
	}
	secret, err := b.priv.ECDH(asPub)
	if err != nil {
		t.Fatal(err)
	}
	prkKey, _ := hkdf.Extract(sha256.New, secret, b.auth)
	keyInfo := "WebPush: info\x00" + string(b.priv.PublicKey().Bytes()) + string(serverPub)
	ikm, _ := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	prk, _ := hkdf.Extract(sha256.New, ikm, salt)
	cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)

	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("o navegador não conseguiria decifrar a mensagem: %v", err)
	}
	// Tira o preenchimento: o último byte diferente de zero é o delimitador 0x02
	end := bytes.LastIndexFunc(plain, func(r rune) bool { return r != 0 })
	if end < 0 || plain[end] != 0x02 {
		t.Fatalf("delimitador do último registro ausente")
	}
	return plain[:end]
}

func setVAPID(t *testing.T) (pub string) {
	t.Helper()
	priv, pub, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAPID_PUBLIC_KEY", pub)
	t.Setenv("VAPID_PRIVATE_KEY", priv)
	t.Setenv("VAPID_SUBJECT", "mailto:teste@example.com")
	return pub
}

func TestSendWebPushCriptografiaEAssinatura(t *testing.T) {
	vapidPub := setVAPID(t)
	b := newBrowser(t)

	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	msg := pushMessage{Title: "⚽ GOL! Flamengo", Body: "Flamengo 1 x 0 Palmeiras",
		Data: map[string]string{"type": "goal", "match_id": "42"}, Tag: "jogo-42", TTL: 3600}
	gone, err := sendWebPush(context.Background(), b.target(srv.URL+"/push/abc"), msg)
	if err != nil || gone {
		t.Fatalf("envio: gone=%v err=%v", gone, err)
	}

	// Cabeçalhos do protocolo
	for h, want := range map[string]string{"Content-Encoding": "aes128gcm", "TTL": "3600", "Urgency": "high", "Topic": "jogo-42"} {
		if v := got.Header.Get(h); v != want {
			t.Errorf("header %s: esperado %q, veio %q", h, want, v)
		}
	}

	// A mensagem decifrada é o JSON do aviso
	var p webPushPayload
	if err := json.Unmarshal(b.decrypt(t, body), &p); err != nil {
		t.Fatalf("conteúdo decifrado não é JSON: %v", err)
	}
	if p.Title != msg.Title || p.Body != msg.Body || p.Tag != "jogo-42" || p.Data["match_id"] != "42" {
		t.Errorf("conteúdo decifrado: %+v", p)
	}

	// Assinatura VAPID: "vapid t=<JWT>, k=<chave pública>", JWT ES256 para a origem do serviço
	auth := got.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "vapid t=") || !strings.Contains(auth, "k="+vapidPub) {
		t.Fatalf("Authorization fora do formato VAPID: %q", auth)
	}
	token := strings.TrimSuffix(strings.SplitN(strings.TrimPrefix(auth, "vapid t="), ",", 2)[0], " ")
	raw, _ := base64.RawURLEncoding.DecodeString(vapidPub)
	key := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(raw[1:33]), Y: new(big.Int).SetBytes(raw[33:])}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) { return key, nil },
		jwt.WithValidMethods([]string{"ES256"})); err != nil {
		t.Fatalf("assinatura VAPID inválida: %v", err)
	}
	if claims["aud"] != srv.URL || claims["sub"] != "mailto:teste@example.com" {
		t.Errorf("claims do VAPID: %v", claims)
	}
}

func TestSendWebPushInscricaoExpirada(t *testing.T) {
	setVAPID(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	gone, err := sendWebPush(context.Background(), newBrowser(t).target(srv.URL), pushMessage{Title: "x"})
	if !gone || err == nil {
		t.Errorf("410 deveria marcar a inscrição como expirada: gone=%v err=%v", gone, err)
	}
}

func TestSendWebPushDesligadoSemChaves(t *testing.T) {
	t.Setenv("VAPID_PUBLIC_KEY", "")
	if _, err := sendWebPush(context.Background(), database.PushTarget{Token: "https://x"}, pushMessage{}); err != errWebPushOff {
		t.Errorf("sem chaves deveria dar errWebPushOff, veio %v", err)
	}
	if WebPushPublicKey() != "" {
		t.Error("sem chaves, a chave pública deveria vir vazia")
	}
}
