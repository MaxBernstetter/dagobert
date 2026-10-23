package framework

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type fillEvent struct {
	closeMs int64
	side    string
}

func derivePositionFromFills(dataDir, playbookID, symbol string, beforeCandleClose int64) (string, error) {
	path := filepath.Join(dataDir, "execution.sqlite")
	if _, err := os.Stat(path); err != nil {
		return "flat", nil
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return "", err
	}
	defer db.Close()

	prefix := playbookID + ":" + symbol + ":"
	rows, err := db.Query(`SELECT side, order_link_id FROM fill_journal WHERE order_link_id LIKE ?`, prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var events []fillEvent
	for rows.Next() {
		var side, link string
		if err := rows.Scan(&side, &link); err != nil {
			return "", err
		}
		closeMs, ok := candleCloseFromLink(link, playbookID, symbol)
		if !ok || closeMs >= beforeCandleClose {
			continue
		}
		events = append(events, fillEvent{closeMs: closeMs, side: strings.ToLower(side)})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	sort.Slice(events, func(i, j int) bool { return events[i].closeMs < events[j].closeMs })

	position := "flat"
	for _, ev := range events {
		switch ev.side {
		case "buy":
			position = "long"
		case "sell":
			position = "flat"
		}
	}
	return position, nil
}

func candleCloseFromLink(link, playbookID, symbol string) (int64, bool) {
	prefix := playbookID + ":" + symbol + ":"
	if !strings.HasPrefix(link, prefix) {
		return 0, false
	}
	ms, err := strconv.ParseInt(link[len(prefix):], 10, 64)
	if err != nil || ms <= 0 {
		return 0, false
	}
	return ms, true
}
