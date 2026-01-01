package domain

// ValidateBalanced ensures ledger lines sum to a balanced double-entry posting.
func ValidateBalanced(lines []LedgerEntryLine) error {
	if len(lines) < 2 {
		return ErrInvalidEntryLines
	}

	var debitTotal int64
	var creditTotal int64
	for _, line := range lines {
		if line.Amount < 0 {
			return ErrInvalidLineAmount
		}
		switch line.Direction {
		case LedgerEntryDirectionDebit:
			debitTotal += line.Amount
		case LedgerEntryDirectionCredit:
			creditTotal += line.Amount
		default:
			return ErrInvalidLineDirection
		}
	}

	if debitTotal != creditTotal {
		return ErrUnbalancedEntry
	}
	return nil
}
