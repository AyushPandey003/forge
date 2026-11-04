package stitch

import "encoding/base64"

func base64Std(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func base64Decode(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}
