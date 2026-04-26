package util

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	// DefaultTopLevelPowTarget is the weakest target accepted by the chaincode.
	// Smaller targets remain valid and make the primary-defense experiment harder
	// to forge.
	DefaultTopLevelPowTarget = "0000ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	topLevelPowFunction      = "TopLevelUpdate"
)

var maxTopLevelPowTarget = mustParseTopLevelPowTarget(DefaultTopLevelPowTarget)

func mustParseTopLevelPowTarget(targetHex string) *big.Int {
	target, err := ParseTopLevelPowTarget(targetHex)
	if err != nil {
		panic(err)
	}
	return target
}

func normalizeTopLevelPowTarget(targetHex string) string {
	targetHex = strings.TrimSpace(strings.ToLower(targetHex))
	return strings.TrimPrefix(targetHex, "0x")
}

// ParseTopLevelPowTarget parses a hexadecimal proof-of-work target.
func ParseTopLevelPowTarget(targetHex string) (*big.Int, error) {
	normalized := normalizeTopLevelPowTarget(targetHex)
	if normalized == "" {
		return nil, fmt.Errorf("pow target is empty")
	}

	target := new(big.Int)
	if _, ok := target.SetString(normalized, 16); !ok || target.Sign() <= 0 {
		return nil, fmt.Errorf("pow target is invalid")
	}

	return target, nil
}

// TopLevelUpdatePowPayload returns the canonical payload bound to the PoW.
func TopLevelUpdatePowPayload(domain string, authority string) string {
	return fmt.Sprintf("%s|%s|%s", topLevelPowFunction, domain, authority)
}

func topLevelUpdatePowHash(domain string, authority string, nonce string) *big.Int {
	sum := sha256.Sum256([]byte(nonce + "|" + TopLevelUpdatePowPayload(domain, authority)))
	hash := new(big.Int)
	hash.SetBytes(sum[:])
	return hash
}

// TopLevelUpdatePowMeetsTarget reports whether the nonce satisfies the parsed
// target for the supplied domain and authority pair.
func TopLevelUpdatePowMeetsTarget(domain string, authority string, nonce string, target *big.Int) bool {
	return topLevelUpdatePowHash(domain, authority, nonce).Cmp(target) <= 0
}

// ValidateTopLevelUpdatePow checks that the supplied nonce satisfies the PoW
// target and that the target is not weaker than the allowed maximum.
func ValidateTopLevelUpdatePow(domain string, authority string, targetHex string, nonce string) error {
	target, err := ParseTopLevelPowTarget(targetHex)
	if err != nil {
		return err
	}
	if target.Cmp(maxTopLevelPowTarget) > 0 {
		return fmt.Errorf("pow target is weaker than allowed")
	}
	if !TopLevelUpdatePowMeetsTarget(domain, authority, nonce, target) {
		return fmt.Errorf("proof of work validation failed")
	}
	return nil
}

// FindTopLevelUpdateNonce searches for a nonce that satisfies the target.
func FindTopLevelUpdateNonce(domain string, authority string, targetHex string) (string, error) {
	target, err := ParseTopLevelPowTarget(targetHex)
	if err != nil {
		return "", err
	}
	if target.Cmp(maxTopLevelPowTarget) > 0 {
		return "", fmt.Errorf("pow target is weaker than allowed")
	}

	for nonce := uint64(0); ; nonce++ {
		nonceStr := strconv.FormatUint(nonce, 10)
		if TopLevelUpdatePowMeetsTarget(domain, authority, nonceStr, target) {
			return nonceStr, nil
		}
	}
}
