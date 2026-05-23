package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func canonicalProposalSHA256(proposal MemoryProposal) (string, error) {
	return canonicalJSONSHA256(proposal)
}

func canonicalApplyPreviewSHA256(preview ApplyPreview) (string, error) {
	return canonicalJSONSHA256(preview)
}

func canonicalJSONSHA256(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
