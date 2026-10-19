// Package webauthntest is a software WebAuthn authenticator for tests: an
// ES256 key answering registration ("none" attestation) and login options the
// way a browser and a security key would, without a browser.
package webauthntest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
)

// Authenticator data flags (WebAuthn §6.1).
const (
	flagUP = 0x01 // user present
	flagUV = 0x04 // user verified
	flagAT = 0x40 // attested credential data included
)

var b64 = base64.RawURLEncoding

// Key is one credential held by the software authenticator.
type Key struct {
	ID        []byte
	priv      *ecdsa.PrivateKey
	userID    []byte
	SignCount uint32
}

// NewKey generates a P-256 credential with a random 32-byte ID.
func NewKey() (*Key, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	return &Key{ID: id, priv: priv}, nil
}

// options is the subset of PublicKeyCredential{Creation,Request}Options the
// authenticator reads.
type options struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		RP        struct {
			ID string `json:"id"`
		} `json:"rp"`
		RPID string `json:"rpId"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"publicKey"`
}

func parseOptions(optionsJSON []byte) (options, error) {
	var o options
	if err := json.Unmarshal(optionsJSON, &o); err != nil {
		return o, fmt.Errorf("parse options: %w", err)
	}
	return o, nil
}

func clientData(typ, challenge, origin string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": origin, "crossOrigin": false})
	return b
}

func authData(rpID string, flags byte, count uint32, attested []byte) []byte {
	h := sha256.Sum256([]byte(rpID))
	out := append(h[:], flags)
	out = binary.BigEndian.AppendUint32(out, count)
	return append(out, attested...)
}

// coseKey encodes the public key as a COSE_Key (EC2, ES256, P-256).
func (k *Key) coseKey() ([]byte, error) {
	pub, err := k.priv.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil {
		return nil, err
	}
	return webauthncbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: pub[1:33], -3: pub[33:65]})
}

// Create answers registration options (navigator.credentials.create) as seen
// from origin and returns the PublicKeyCredential JSON a browser would post.
func (k *Key) Create(optionsJSON []byte, origin string) ([]byte, error) {
	o, err := parseOptions(optionsJSON)
	if err != nil {
		return nil, err
	}
	if k.userID, err = b64.DecodeString(o.PublicKey.User.ID); err != nil {
		return nil, fmt.Errorf("user id: %w", err)
	}
	cose, err := k.coseKey()
	if err != nil {
		return nil, err
	}
	attested := make([]byte, 16) // zero AAGUID
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(k.ID)))
	attested = append(attested, k.ID...)
	attested = append(attested, cose...)
	att, err := webauthncbor.Marshal(map[string]any{
		"fmt":      "none",
		"attStmt":  map[string]any{},
		"authData": authData(o.PublicKey.RP.ID, flagUP|flagUV|flagAT, k.SignCount, attested),
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"id": b64.EncodeToString(k.ID), "rawId": b64.EncodeToString(k.ID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(clientData("webauthn.create", o.PublicKey.Challenge, origin)),
			"attestationObject": b64.EncodeToString(att),
			"transports":        []string{"usb"},
		},
	})
}

// Get answers login options (navigator.credentials.get) as seen from origin,
// signing with the current SignCount, and returns the PublicKeyCredential
// JSON. The caller moves SignCount to model the authenticator's counter.
func (k *Key) Get(optionsJSON []byte, origin string) ([]byte, error) {
	o, err := parseOptions(optionsJSON)
	if err != nil {
		return nil, err
	}
	cd := clientData("webauthn.get", o.PublicKey.Challenge, origin)
	ad := authData(o.PublicKey.RPID, flagUP|flagUV, k.SignCount, nil)
	cdHash := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte(nil), ad...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.priv, digest[:])
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"id": b64.EncodeToString(k.ID), "rawId": b64.EncodeToString(k.ID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(cd),
			"authenticatorData": b64.EncodeToString(ad),
			"signature":         b64.EncodeToString(sig),
			"userHandle":        b64.EncodeToString(k.userID),
		},
	})
}
