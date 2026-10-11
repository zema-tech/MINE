package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

func encodeHash(p ArgonParams, salt, hash []byte) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		19, p.Memory, p.Iterations, p.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(hash))
}

func decodeHash(encoded string) (ArgonParams, []byte, []byte, error) {
	var p ArgonParams
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, fmt.Errorf("invalid hash format")
	}
	var mem, iter uint32
	var par uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &par); err != nil {
		// fallback: parse manually
		for _, kv := range strings.Split(parts[3], ",") {
			kvs := strings.SplitN(kv, "=", 2)
			if len(kvs) != 2 {
				continue
			}
			switch kvs[0] {
			case "m":
				v, _ := strconv.Atoi(kvs[1])
				mem = uint32(v)
			case "t":
				v, _ := strconv.Atoi(kvs[1])
				iter = uint32(v)
			case "p":
				v, _ := strconv.Atoi(kvs[1])
				par = uint8(v)
			}
		}
	}
	p = ArgonParams{Memory: mem, Iterations: iter, Parallelism: par, SaltLength: 16, KeyLength: 32}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, err
	}
	hash, err := b64.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, err
	}
	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(hash))
	return p, salt, hash, nil
}

func subtleEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
