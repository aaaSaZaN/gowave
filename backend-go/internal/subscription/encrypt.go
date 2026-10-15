package subscription

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"filippo.io/age/tag"
)

// parseAgeRecipient accepts the recipient types age-encryption's
// Encrypter.addRecipient does.
func parseAgeRecipient(s string) (age.Recipient, error) {
	switch {
	case strings.HasPrefix(s, "age1pq1"):
		return age.ParseHybridRecipient(s)
	case strings.HasPrefix(s, "age1tag1"), strings.HasPrefix(s, "age1tagpq1"):
		return tag.ParseRecipient(s)
	case strings.HasPrefix(s, "age1"):
		return age.ParseX25519Recipient(s)
	}
	return nil, errors.New("unrecognized recipient type")
}

// encryptResponseBody ports common/helpers/response-encryption: an armored
// age file for the rule's recipient. Upstream's service turns any failure
// into an empty body.
func encryptResponseBody(body string, encryption *SRREncryption) string {
	if encryption.Method != "age1" && encryption.Method != "age1pq1" {
		return ""
	}
	recipient, err := parseAgeRecipient(strings.TrimSpace(encryption.Key))
	if err != nil {
		return ""
	}
	var out bytes.Buffer
	armored := armor.NewWriter(&out)
	writer, err := age.Encrypt(armored, recipient)
	if err != nil {
		return ""
	}
	if _, err := io.WriteString(writer, body); err != nil {
		return ""
	}
	if err := writer.Close(); err != nil {
		return ""
	}
	if err := armored.Close(); err != nil {
		return ""
	}
	return out.String()
}
