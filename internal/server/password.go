package server

import (
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	{
		params := strings.Split(parts[3], ",")
		if len(params) != 3 {
			return false
		}

		m, ok := strings.CutPrefix(params[0], "m=")
		if !ok {
			return false
		}
		t, ok := strings.CutPrefix(params[1], "t=")
		if !ok {
			return false
		}
		p, ok := strings.CutPrefix(params[2], "p=")
		if !ok {
			return false
		}

		m64, err := strconv.ParseUint(m, 10, 32)
		if err != nil {
			return false
		}
		t64, err := strconv.ParseUint(t, 10, 32)
		if err != nil {
			return false
		}
		p64, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return false
		}

		memory = uint32(m64)
		timeCost = uint32(t64)
		threads = uint8(p64)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	check := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, check) == 1
}
