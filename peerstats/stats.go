package peerstats

import "time"

type Stat struct {
	Rx              int64
	Tx              int64
	LatestHandshake time.Time
	LatestEndpoint  string
}

type TotalStatCalc struct {
	prevTotalRx int64 // total sum not including the current period
	prevTotalTx int64 // total sum not including the current period
	curPeriod   Stat
}

func MakeTotalStatsCalc(
	prevTotalTx int64,
	prevTotalRx int64,
) TotalStatCalc {
	return TotalStatCalc{
		prevTotalTx: prevTotalTx,
		prevTotalRx: prevTotalRx,
	}
}

// newPeriod is true if the provided current period data looks like it was collected after the peer reset.
// But it is not guaranteed to detect all such resets and serves as a defensive check.
func (ts *TotalStatCalc) Calculate(curPeriod Stat) (total Stat, newPeriod bool) {
	newPeriod = ts.curPeriod.Tx > curPeriod.Tx ||
		ts.curPeriod.Rx > curPeriod.Rx ||
		!ts.curPeriod.LatestHandshake.IsZero() && curPeriod.LatestHandshake.IsZero() ||
		ts.curPeriod.LatestEndpoint != "" && curPeriod.LatestEndpoint == ""

	if newPeriod {
		ts.prevTotalTx += ts.curPeriod.Tx
		ts.prevTotalRx += ts.curPeriod.Rx
	}

	ts.curPeriod = curPeriod

	total = Stat{
		Tx:              ts.prevTotalTx + curPeriod.Tx,
		Rx:              ts.prevTotalRx + curPeriod.Rx,
		LatestHandshake: curPeriod.LatestHandshake,
		LatestEndpoint:  curPeriod.LatestEndpoint,
	}
	return
}
