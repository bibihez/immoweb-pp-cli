// Copyright 2026 bibihez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bibihez/immoweb-pp-cli/internal/store"
)

// newWatchCmd implements `immoweb watch` — diff the latest sync against the
// previous one and surface NEW listings, REMOVED ones, PRICE_CHANGED ones,
// and STATUS_CHANGED ones. The output is the daily prospecting list a real-
// estate agent runs before their first coffee: it identifies frustrated
// FSBO sellers (under_option flips, price drops) and gone-cold inventory
// that's ready for re-pitching.
//
// Reads observations only — calling this without running sync first returns
// empty results, not an error.
func newWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		postal     string
		dbPathFlag string
	)
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Diff latest sync vs previous: NEW, REMOVED, PRICE_CHANGED, STATUS_CHANGED",
		Long: `Compare the most recent observation of each listing against the
prior observation in the local database. Emits four diff classes:

  NEW             - listing appeared this sync, did not exist before
  REMOVED         - listing was here last sync, gone this sync
  PRICE_CHANGED   - same listing, price differs (drop or rise; magnitude in output)
  STATUS_CHANGED  - flag_main changed (e.g. active -> under_option, new -> sold)

Run 'sync' first to populate observations. Watching twice in the same second
shows nothing because the observation-log primary key collapses same-second
duplicates.`,
		Example: `  # diff full local store
  immoweb-pp-cli watch

  # restrict to one or more postal codes
  immoweb-pp-cli watch --postal 1180,1060

  # JSON for downstream piping
  immoweb-pp-cli watch --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPathFlag
			if path == "" {
				path = defaultDBPath("immoweb-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			pcFilter := ""
			var pcArgs []interface{}
			if postal != "" {
				codes := splitCSVTrim(postal)
				if len(codes) > 0 {
					placeholders := strings.Repeat("?,", len(codes))
					placeholders = strings.TrimRight(placeholders, ",")
					pcFilter = " AND postal_code IN (" + placeholders + ")"
					for _, c := range codes {
						pcArgs = append(pcArgs, c)
					}
				}
			}

			diffs, err := computeWatchDiffs(cmd.Context(), db.DB(), pcFilter, pcArgs)
			if err != nil {
				return err
			}
			return renderWatchDiffs(cmd, flags, diffs)
		},
	}
	cmd.Flags().StringVar(&postal, "postal", "", "Comma-separated postal codes to filter (default: all)")
	cmd.Flags().StringVar(&dbPathFlag, "db", "", "Path to the local database (default: ~/.immoweb-pp-cli/store.db)")
	return cmd
}

type watchDiff struct {
	Kind        string `json:"kind"`
	ListingID   int64  `json:"listing_id"`
	PostalCode  string `json:"postal_code,omitempty"`
	Locality    string `json:"locality,omitempty"`
	PriceBefore int64  `json:"price_before,omitempty"`
	PriceAfter  int64  `json:"price_after,omitempty"`
	PriceDelta  int64  `json:"price_delta,omitempty"`
	FlagBefore  string `json:"flag_before,omitempty"`
	FlagAfter   string `json:"flag_after,omitempty"`
	URL         string `json:"url"`
}

// computeWatchDiffs runs the diff query. It joins each listing's latest two
// observations and emits one row per change. The CTE pattern picks the two
// most recent observations per listing using ROW_NUMBER() — this works
// regardless of how often the user syncs (twice today vs. yesterday+today).
func computeWatchDiffs(ctx interface{}, db *sql.DB, pcFilter string, pcArgs []interface{}) ([]watchDiff, error) {
	// Snapshot model: each sync produces observations dated within the
	// same minute(ish). We treat a "snapshot" as one calendar day's
	// worth of observations and compare the two most recent distinct
	// days that have any observations. NEW/REMOVED are well-defined
	// against a snapshot pair; comparing per-listing latest-vs-prior
	// independently would mis-label listings that only ever appeared
	// once as NEW.
	q := `
WITH days AS (
    SELECT DISTINCT date(observed_at) AS d FROM observations
    WHERE 1=1` + pcFilter + `
    ORDER BY d DESC LIMIT 2
),
latest_day AS (SELECT d FROM days ORDER BY d DESC LIMIT 1),
prior_day  AS (SELECT d FROM days ORDER BY d ASC  LIMIT 1),
latest AS (
    -- Most recent observation per listing within the latest day
    SELECT listing_id, price, flag_main, postal_code, locality,
           ROW_NUMBER() OVER (PARTITION BY listing_id ORDER BY observed_at DESC) AS rn
    FROM observations
    WHERE date(observed_at) = (SELECT d FROM latest_day)` + pcFilter + `
),
prior AS (
    SELECT listing_id, price, flag_main, postal_code, locality,
           ROW_NUMBER() OVER (PARTITION BY listing_id ORDER BY observed_at DESC) AS rn
    FROM observations
    WHERE date(observed_at) = (SELECT d FROM prior_day)
      AND (SELECT d FROM latest_day) <> (SELECT d FROM prior_day)` + pcFilter + `
),
latest1 AS (SELECT * FROM latest WHERE rn = 1),
prior1  AS (SELECT * FROM prior  WHERE rn = 1)
SELECT 'NEW' AS kind, l.listing_id, COALESCE(l.postal_code,''), COALESCE(l.locality,''),
       0, COALESCE(l.price,0), '', COALESCE(l.flag_main,'')
FROM latest1 l
LEFT JOIN prior1 p ON p.listing_id = l.listing_id
WHERE p.listing_id IS NULL
UNION ALL
SELECT 'REMOVED', p.listing_id, COALESCE(p.postal_code,''), COALESCE(p.locality,''),
       COALESCE(p.price,0), 0, COALESCE(p.flag_main,''), ''
FROM prior1 p
LEFT JOIN latest1 l ON l.listing_id = p.listing_id
WHERE l.listing_id IS NULL
UNION ALL
SELECT 'PRICE_CHANGED', l.listing_id, COALESCE(l.postal_code,''), COALESCE(l.locality,''),
       COALESCE(p.price,0), COALESCE(l.price,0),
       COALESCE(p.flag_main,''), COALESCE(l.flag_main,'')
FROM latest1 l JOIN prior1 p ON p.listing_id = l.listing_id
WHERE l.price IS NOT NULL AND p.price IS NOT NULL AND l.price <> p.price
UNION ALL
SELECT 'STATUS_CHANGED', l.listing_id, COALESCE(l.postal_code,''), COALESCE(l.locality,''),
       COALESCE(p.price,0), COALESCE(l.price,0),
       COALESCE(p.flag_main,''), COALESCE(l.flag_main,'')
FROM latest1 l JOIN prior1 p ON p.listing_id = l.listing_id
WHERE COALESCE(l.flag_main,'') <> COALESCE(p.flag_main,'')
ORDER BY 1, 2`
	// pcFilter is interpolated 3 times in the CTE (days, latest, prior),
	// so we replay pcArgs 3 times when binding.
	allArgs := make([]interface{}, 0, len(pcArgs)*3)
	for i := 0; i < 3; i++ {
		allArgs = append(allArgs, pcArgs...)
	}
	rows, err := db.Query(q, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("watch query: %w", err)
	}
	defer rows.Close()

	var out []watchDiff
	for rows.Next() {
		var d watchDiff
		if err := rows.Scan(&d.Kind, &d.ListingID, &d.PostalCode, &d.Locality,
			&d.PriceBefore, &d.PriceAfter, &d.FlagBefore, &d.FlagAfter); err != nil {
			return nil, err
		}
		if d.PriceBefore > 0 && d.PriceAfter > 0 {
			d.PriceDelta = d.PriceAfter - d.PriceBefore
		}
		d.URL = "https://www.immoweb.be/en/classified/" + strconv.FormatInt(d.ListingID, 10)
		out = append(out, d)
	}
	return out, rows.Err()
}

func renderWatchDiffs(cmd *cobra.Command, flags *rootFlags, diffs []watchDiff) error {
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]interface{}{
			"diffs": diffs,
			"count": len(diffs),
		})
	}
	if len(diffs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No diffs — run 'sync' at least twice to populate observations.")
		return nil
	}
	headers := []string{"Kind", "ID", "Postal", "Locality", "Price Before", "Price After", "Δ", "Flag Before", "Flag After"}
	rows := make([][]string, 0, len(diffs))
	for _, d := range diffs {
		delta := ""
		if d.PriceDelta != 0 {
			delta = fmt.Sprintf("%+d", d.PriceDelta)
		}
		rows = append(rows, []string{
			d.Kind,
			strconv.FormatInt(d.ListingID, 10),
			d.PostalCode,
			d.Locality,
			formatEuro(d.PriceBefore),
			formatEuro(d.PriceAfter),
			delta,
			d.FlagBefore,
			d.FlagAfter,
		})
	}
	return flags.printTable(cmd, headers, rows)
}

func formatEuro(v int64) string {
	if v <= 0 {
		return ""
	}
	return fmt.Sprintf("€%d", v)
}

func splitCSVTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
