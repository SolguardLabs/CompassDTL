package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type OperationSpec struct {
	Domain        string `json:"domain"`
	ChainID       string `json:"chainId"`
	Target        string `json:"target"`
	Method        string `json:"method"`
	PayloadHash   string `json:"payloadHash"`
	Salt          string `json:"salt"`
	EarliestEpoch uint64 `json:"earliestEpoch"`
	ExpiresEpoch  uint64 `json:"expiresEpoch"`
	Predecessor   string `json:"predecessor,omitempty"`
}

func (spec OperationSpec) Validate() error {
	values := map[string]string{
		"domain":   spec.Domain,
		"chain id": spec.ChainID,
		"target":   spec.Target,
		"method":   spec.Method,
		"salt":     spec.Salt,
	}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if value != strings.TrimSpace(value) {
			return fmt.Errorf("%s must be normalized", name)
		}
	}
	if !digestPattern.MatchString(spec.PayloadHash) {
		return fmt.Errorf("payload hash must be lowercase SHA-256")
	}
	if spec.Predecessor != "" && !digestPattern.MatchString(spec.Predecessor) {
		return fmt.Errorf("predecessor must be a lowercase operation id")
	}
	if spec.ExpiresEpoch <= spec.EarliestEpoch {
		return fmt.Errorf("operation expiry must follow its earliest execution epoch")
	}
	return nil
}

func (spec OperationSpec) CanonicalBytes() ([]byte, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(spec)
}

func (spec OperationSpec) ID() (string, error) {
	encoded, err := spec.CanonicalBytes()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func HashPayload(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
