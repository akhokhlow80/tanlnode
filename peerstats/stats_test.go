package peerstats_test

import (
	"akhokhlow80/tanlnode/peerstats"
	"testing"
	"time"
)

func TestTotalStatsCalculate(t *testing.T) {
	calc := peerstats.MakeTotalStatsCalc(78902, 1078)

	now := time.Now()

	total, newPeriod := calc.Calculate(peerstats.Stat{
		Tx:              700,
		Rx:              368,
		LatestHandshake: now,
		LatestEndpoint:  "127.0.0.1:1234",
	})
	if newPeriod {
		t.Fatalf("newPeriod expected to be false")
	}
	if total != (peerstats.Stat{
		Tx:              78902 + 700,
		Rx:              1078 + 368,
		LatestHandshake: now,
		LatestEndpoint:  "127.0.0.1:1234",
	}) {
		t.Fatalf("total peer stat differs from expected")
	}

	now = time.Now()
	total, newPeriod = calc.Calculate(peerstats.Stat{
		Tx:              700 + 144,
		Rx:              368 + 168,
		LatestHandshake: now,
		LatestEndpoint:  "127.0.0.1:1235",
	})
	if newPeriod {
		t.Fatalf("newPeriod expected to be false")
	}
	if total != (peerstats.Stat{
		Tx:              78902 + 700 + 144,
		Rx:              1078 + 368 + 168,
		LatestHandshake: now,
		LatestEndpoint:  "127.0.0.1:1235",
	}) {
		t.Fatalf("total peer stat differs from expected")
	}

	total, newPeriod = calc.Calculate(peerstats.Stat{
		Tx:              89088,
		Rx:              86,
		LatestHandshake: time.Time{},
		LatestEndpoint:  "",
	})
	if !newPeriod {
		t.Fatalf("newPeriod expected to be true")
	}
	if total != (peerstats.Stat{
		Tx:              78902 + 700 + 144 + 89088,
		Rx:              368 + 1078 + 168 + 86,
		LatestHandshake: time.Time{},
		LatestEndpoint:  "",
	}) {
		t.Fatalf("total peer stat differs from expected")
	}
}
