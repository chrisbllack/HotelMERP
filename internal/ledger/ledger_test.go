package ledger

import "testing"

// Pure contract tests — validation, account routing and money conversion
// must hold before anything touches a database.

func validJournal() JournalInput {
	return JournalInput{
		PropertyID: "00000000-0000-0000-0000-000000000001",
		Date:       "2026-10-18",
		Source:     "manual",
		Lines: []JournalLine{
			{AccountCode: Cash, DebitKobo: 500_000, Description: "Settlement"},
			{AccountCode: GuestLedger, CreditKobo: 500_000, Description: "Settlement"},
		},
	}
}

func TestValidateAcceptsBalancedJournal(t *testing.T) {
	if err := Validate(validJournal()); err != nil {
		t.Fatalf("balanced journal rejected: %v", err)
	}
}

func TestValidateRejectsEmptyJournal(t *testing.T) {
	in := validJournal()
	in.Lines = nil
	if err := Validate(in); err == nil {
		t.Fatal("empty journal accepted")
	}
}

func TestValidateRejectsMissingProperty(t *testing.T) {
	in := validJournal()
	in.PropertyID = ""
	if err := Validate(in); err == nil {
		t.Fatal("propertyless journal accepted")
	}
}

func TestValidateRejectsUnbalancedJournal(t *testing.T) {
	in := validJournal()
	in.Lines[1].CreditKobo = 400_000
	if err := Validate(in); err == nil {
		t.Fatal("unbalanced journal accepted")
	}
}

func TestValidateRejectsNegativeAmounts(t *testing.T) {
	in := validJournal()
	in.Lines[0].DebitKobo = -1
	if err := Validate(in); err == nil {
		t.Fatal("negative amount accepted")
	}
}

func TestValidateRejectsDebitAndCreditOnOneLine(t *testing.T) {
	in := validJournal()
	in.Lines[0].CreditKobo = 100
	if err := Validate(in); err == nil {
		t.Fatal("line with both sides accepted")
	}
}

func TestValidateRejectsUnknownAccount(t *testing.T) {
	in := validJournal()
	in.Lines[0].AccountCode = "9999"
	if err := Validate(in); err == nil {
		t.Fatal("unknown account accepted")
	}
}

func TestValidateRejectsZeroMovement(t *testing.T) {
	in := validJournal()
	in.Lines[0].DebitKobo = 0
	in.Lines[1].CreditKobo = 0
	if err := Validate(in); err == nil {
		t.Fatal("zero-value journal accepted")
	}
}

func TestEveryStandardAccountIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range StandardCOA {
		if seen[a.Code] {
			t.Errorf("duplicate account code %s", a.Code)
		}
		seen[a.Code] = true
	}
	if len(StandardCOA) < 20 {
		t.Errorf("standard chart unexpectedly small: %d accounts", len(StandardCOA))
	}
}

func TestRevenueAccountRouting(t *testing.T) {
	cases := map[string]string{
		"ROOM":      RoomRevenue,
		"room":      RoomRevenue, // case-insensitive
		"F&B":       FBRevenue,
		"MINIBAR":   FBRevenue,
		"BANQUET":   EventsRevenue,
		"LAUNDRY":   OtherRevenue,
		"SOMETHING": OtherRevenue, // unknown codes never fail posting
	}
	for code, want := range cases {
		if got := RevenueAccount(code); got != want {
			t.Errorf("RevenueAccount(%q) = %s, want %s", code, got, want)
		}
	}
}

func TestPaymentAccountRouting(t *testing.T) {
	if got := PaymentAccount("cash"); got != Cash {
		t.Errorf("cash settlement debits %s, want %s", got, Cash)
	}
	if got := PaymentAccount("card"); got != Bank {
		t.Errorf("card settlement debits %s, want %s", got, Bank)
	}
	if got := PaymentAccount(""); got != Bank {
		t.Errorf("empty method debits %s, want %s (bank is the safe default)", got, Bank)
	}
}

func TestKoboConversionsRoundHalfUp(t *testing.T) {
	cases := map[float64]int64{
		125.50: 12550,
		0.005:  1,    // half-kobo rounds up, never a fractional kobo
		-1.00:  -100, // negatives convert too (refunds)
	}
	for naira, wantKobo := range cases {
		if got := NairaToKobo(naira); got != wantKobo {
			t.Errorf("NairaToKobo(%v) = %d, want %d", naira, got, wantKobo)
		}
	}
	// Exact kobo amounts survive a full round trip through float64 naira —
	// sub-kobo input does not, because kobo IS the storage precision.
	for _, kobo := range []int64{1, 12550, -100, 100_000_000} {
		if got := NairaToKobo(KoboToNaira(kobo)); got != kobo {
			t.Errorf("round trip of %d kobo = %d", kobo, got)
		}
	}
}
