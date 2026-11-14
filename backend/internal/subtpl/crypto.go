package subtpl

import (
	"crypto/ecdh"
	"crypto/mldsa"
	"crypto/mlkem"
	"errors"
	"strings"
	"sync"

	"gowave/internal/subtpl/js"
	"gowave/internal/subtpl/ojson"
)

var keyCache sync.Map // input → derived string ("" when invalid)

func cached(prefix, input string, derive func(string) (string, error)) (string, bool) {
	key := prefix + "\x00" + input
	if value, ok := keyCache.Load(key); ok {
		s := value.(string)
		return s, s != ""
	}
	value, err := derive(input)
	if err != nil {
		value = ""
	}
	keyCache.Store(key, value)
	return value, value != ""
}

// x25519PublicKey derives the Reality public key (base64url, unpadded) from
// a base64 private key.
func x25519PublicKey(private string) (string, error) {
	raw := js.DecodeBase64(private)
	key, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", err
	}
	return js.Base64URL(key.PublicKey().Bytes()), nil
}

// mldsa65PublicKey ports getMlDsa65PublicKey: ML-DSA-65 key from a 32-byte
// seed, public key as base64url.
func mldsa65PublicKey(seed string) (string, error) {
	key, err := mldsa.NewPrivateKey(mldsa.MLDSA65(), js.DecodeBase64(seed))
	if err != nil {
		return "", err
	}
	return js.Base64URL(key.PublicKey().Bytes()), nil
}

// encryptionFromDecryption ports generateEncryptionFromDecryption for VLESS
// "mlkem768x25519plus" server decryption strings.
func encryptionFromDecryption(decryption string) (string, error) {
	parts := strings.Split(decryption, ".")
	if len(parts) < 3 || parts[0] != "mlkem768x25519plus" {
		return "", errors.New("invalid protocol")
	}
	mode, ticket := parts[1], parts[2]
	if mode != "native" && mode != "xorpub" && mode != "random" {
		return "", errors.New("invalid mode")
	}
	if ticket == "" {
		return "", errors.New("ticket lifetime is required")
	}
	var padding, keys []string
	foundKey := false
	for _, part := range parts[3:] {
		if strings.TrimSpace(part) == "" {
			if !foundKey {
				padding = append(padding, part)
			}
			continue
		}
		length := len(js.DecodeBase64(part))
		switch {
		case length == 32 || length == 64:
			foundKey = true
			keys = append(keys, part)
		case !foundKey:
			padding = append(padding, part)
		default:
			return "", errors.New("invalid key length after keys started")
		}
	}
	if len(keys) == 0 {
		return "", errors.New("at least one key is required")
	}
	var publicKeys []string
	for _, key := range keys {
		raw := js.DecodeBase64(key)
		switch len(raw) {
		case 32:
			public, err := x25519PublicKey(key)
			if err != nil {
				return "", err
			}
			publicKeys = append(publicKeys, public)
		case 64:
			decap, err := mlkem.NewDecapsulationKey768(raw)
			if err != nil {
				return "", err
			}
			publicKeys = append(publicKeys, js.Base64URL(decap.EncapsulationKey().Bytes()))
		default:
			return "", errors.New("cannot detect key type")
		}
	}
	rtt := "0rtt"
	if ticket == "0s" {
		rtt = "1rtt"
	}
	out := []string{parts[0], mode, rtt}
	if p := strings.Join(padding, "."); p != "" {
		out = append(out, p)
	}
	return strings.Join(append(out, publicKeys...), "."), nil
}

func stringAt(root any, path ...string) string {
	value, _ := ojson.Get(root, path)
	s, _ := value.(string)
	return s
}

// inboundKeys builds the per-tag maps resolveProxyConfig precomputes.
func inboundKeys(inbounds []any) (publicKeys, mldsa65Keys, encryption map[string]string) {
	publicKeys, mldsa65Keys, encryption = map[string]string{}, map[string]string{}, map[string]string{}
	for _, inbound := range inbounds {
		tag := stringAt(inbound, "tag")
		if private := stringAt(inbound, "streamSettings", "realitySettings", "privateKey"); private != "" {
			if _, seen := publicKeys[tag]; !seen {
				if value, ok := cached("x25519", private, x25519PublicKey); ok {
					publicKeys[tag] = value
				}
			}
		}
		if seed := stringAt(inbound, "streamSettings", "realitySettings", "mldsa65Seed"); seed != "" {
			if _, seen := mldsa65Keys[tag]; !seen {
				if value, ok := cached("mldsa65", seed, mldsa65PublicKey); ok {
					mldsa65Keys[tag] = value
				}
			}
		}
		if stringAt(inbound, "protocol") == "vless" {
			if decryption := stringAt(inbound, "settings", "decryption"); decryption != "" && decryption != "none" {
				if _, seen := encryption[tag]; !seen {
					if value, ok := cached("encryption", decryption, encryptionFromDecryption); ok {
						encryption[tag] = value
					}
				}
			}
		}
	}
	return
}
