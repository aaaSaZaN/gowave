package nodes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

func hkdfDerive(ikm, salt, info []byte, length int) []byte {
	if len(salt) == 0 {
		salt = make([]byte, sha256.Size)
	}
	h := hmac.New(sha256.New, salt)
	h.Write(ikm)
	prk := h.Sum(nil)

	var okm []byte
	var t []byte
	counter := byte(1)
	for len(okm) < length {
		h = hmac.New(sha256.New, prk)
		h.Write(t)
		h.Write(info)
		h.Write([]byte{counter})
		t = h.Sum(nil)
		okm = append(okm, t...)
		counter++
	}
	return okm[:length]
}

func DeriveSNI(caCertPem, jwtPubKeyPem string) string {
	reHeader := regexp.MustCompile(`-----[^-]+-----`)
	reNonB64 := regexp.MustCompile(`[^A-Za-z0-9+/=]`)
	canon := func(pem string) string {
		s := reHeader.ReplaceAllString(pem, "")
		return reNonB64.ReplaceAllString(s, "")
	}

	ikm := append([]byte(canon(jwtPubKeyPem)), []byte(canon(caCertPem))...)
	okm := hkdfDerive(ikm, nil, []byte("rw-v1"), 22)

	host := hex.EncodeToString(okm[:16])
	sub := hex.EncodeToString(okm[16:21])
	tlds := []string{"com", "net", "org", "io", "dev", "app"}
	tld := tlds[int(okm[21])%len(tlds)]
	return fmt.Sprintf("%s.%s.%s", host, sub, tld)
}
