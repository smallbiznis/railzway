package format

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	seqPadRe = regexp.MustCompile(`\{SEQ(\d+)\}`)
)

const DefaultInvoiceNumberTemplate = "INV-{YYYY}{MM}{DD}-{SEQ6}"

// FormatInvoiceNumber formats a human-readable invoice number
// based on a template, invoice issue time, and monotonic sequence.
//
// This function is PURE:
// - No side effects
// - No DB access
// - Fully deterministic
func FormatInvoiceNumber(
	template string,
	issuedAt time.Time,
	seq int64,
) (string, error) {

	if template == "" {
		return "", fmt.Errorf("invoice number template is empty")
	}

	if seq <= 0 {
		return "", fmt.Errorf("invalid invoice sequence: %d", seq)
	}

	out := template

	// Date tokens
	out = strings.ReplaceAll(out, "{YYYY}", issuedAt.Format("2006"))
	out = strings.ReplaceAll(out, "{YY}", issuedAt.Format("06"))
	out = strings.ReplaceAll(out, "{MM}", issuedAt.Format("01"))
	out = strings.ReplaceAll(out, "{DD}", issuedAt.Format("02"))

	// Simple sequence
	out = strings.ReplaceAll(out, "{SEQ}", strconv.FormatInt(seq, 10))

	// Padded sequence
	out = seqPadRe.ReplaceAllStringFunc(out, func(m string) string {
		match := seqPadRe.FindStringSubmatch(m)
		if len(match) != 2 {
			return m // should never happen
		}

		width, err := strconv.Atoi(match[1])
		if err != nil || width <= 0 {
			return m
		}

		return fmt.Sprintf("%0*d", width, seq)
	})

	// Final safety check: unresolved tokens
	if strings.Contains(out, "{") || strings.Contains(out, "}") {
		return "", fmt.Errorf("unresolved token in invoice format: %s", out)
	}

	return out, nil
}
