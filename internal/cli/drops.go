// Copyright 2026 bibihez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bibihez/immoweb-pp-cli/internal/store"
)

// newDropsCmd implements `immoweb drops` — surface listings whose price has
// dropped by at least a given percent over a given window. This is the
// primary prospecting signal for a real-estate agent: a price drop on a
// competitor's listing means the seller is unhappy with their current
// agent and is ripe for re-pitching.
//
// Reads observations only. Returns empty (no error) when the local store
// has not yet seen the same listing at two different prices.
func newDropsCmd(flags *rootFlags) *cobra.Command {
	var (
		minPct     float64
		days       int
		postal     string
		dbPathFlag string
	)
	cmd := &cobra.Command{
		Use:   "drops",
		Short: "Listings whose asking price has dropped by ≥N% in the last N days",
		Long: `Find listings where the maximum observed price within a window has
fallen to a lower current price by at least --min-pct. Useful for
prospecting frustrated sellers and spotting negotiation leverage.

The query compares each listing's MAX(price) over the window against
its most recent price; a drop is reported when the relative reduction
meets or exceeds --min-pct. Listings with only one observation are
silently excluded (no comparison possible).`,
		Example: `  # default: any drop ≥5% in last 30 days
  immoweb-pp-cli drops

  # bigger drops, longer window
  immoweb-pp-cli drops --min-pct 10 --days 60

  # one postal code
  immoweb-pp-cli drops --postal 1180 --min-pct 8`,
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

			whereExtra := ""
			var extraArgs []interface{}
			if postal != "" {
				codes := splitCSVTrim(postal)
				if len(codes) > 0 {
					placeholders := strings.Repeat("?,", len(codes))
					placeholders = strings.TrimRight(placeholders, ",")
					whereExtra = " AND postal_code IN (" + placeholders + ")"
					for _, c := range codes {
						extraArgs = append(extraArgs, c)
					}
				}
			}

			q := `
WITH win AS (
    SELECT listing_id, observed_at, price, postal_code, locality, flag_main, customer_name,
           ROW_NUMBER() OVER (PARTITION BY listing_id ORDER BY observed_at DESC) AS rn,
           MAX(price) OVER (PARTITION BY listing_id) AS peak_price
    FROM observations
    WHERE observed_at >= datetime('now', '-' || ? || ' days')
      AND price IS NOT NULL AND price > 0` + whereExtra + `
)
SELECT listing_id, COALESCE(postal_code,''), COALESCE(locality,''), peak_price, price AS current_price,
       (peak_price - price) AS abs_drop,
       ROUND(100.0 * (peak_price - price) / peak_price, 1) AS pct_drop,
       COALESCE(customer_name,''), COALESCE(flag_main,'')
FROM win
WHERE rn = 1
  AND peak_price > price
  AND (100.0 * (peak_price - price) / peak_price) >= ?
ORDER BY pct_drop DESC, abs_drop DESC`

			qArgs := append([]interface{}{days}, extraArgs...)
			qArgs = append(qArgs, minPct)

			rows, err := db.DB().Query(q, qArgs...)
			if err != nil {
				return fmt.Errorf("drops query: %w", err)
			}
			defer rows.Close()

			type drop struct {
				ListingID    int64   `json:"listing_id"`
				PostalCode   string  `json:"postal_code"`
				Locality     string  `json:"locality"`
				PeakPrice    int64   `json:"peak_price"`
				CurrentPrice int64   `json:"current_price"`
				AbsDrop      int64   `json:"abs_drop"`
				PctDrop      float64 `json:"pct_drop"`
				CustomerName string  `json:"customer_name"`
				FlagMain     string  `json:"flag_main"`
				URL          string  `json:"url"`
			}
			var out []drop
			for rows.Next() {
				var d drop
				if err := rows.Scan(&d.ListingID, &d.PostalCode, &d.Locality, &d.PeakPrice, &d.CurrentPrice,
					&d.AbsDrop, &d.PctDrop, &d.CustomerName, &d.FlagMain); err != nil {
					return err
				}
				d.URL = "https://www.immoweb.be/en/classified/" + strconv.FormatInt(d.ListingID, 10)
				out = append(out, d)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]interface{}{
					"drops":   out,
					"count":   len(out),
					"min_pct": minPct,
					"days":    days,
				})
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No drops ≥%.1f%% found in last %d days. Run more syncs to build history.\n", minPct, days)
				return nil
			}
			headers := []string{"ID", "Postal", "Locality", "Peak", "Now", "Δ €", "Δ %", "Agent", "Status"}
			rs := make([][]string, 0, len(out))
			for _, d := range out {
				rs = append(rs, []string{
					strconv.FormatInt(d.ListingID, 10),
					d.PostalCode,
					d.Locality,
					formatEuro(d.PeakPrice),
					formatEuro(d.CurrentPrice),
					fmt.Sprintf("-€%d", d.AbsDrop),
					fmt.Sprintf("-%.1f%%", d.PctDrop),
					d.CustomerName,
					d.FlagMain,
				})
			}
			return flags.printTable(cmd, headers, rs)
		},
	}
	cmd.Flags().Float64Var(&minPct, "min-pct", 5.0, "Minimum percent drop to surface")
	cmd.Flags().IntVar(&days, "days", 30, "Look-back window in days")
	cmd.Flags().StringVar(&postal, "postal", "", "Comma-separated postal codes to filter (default: all)")
	cmd.Flags().StringVar(&dbPathFlag, "db", "", "Path to the local database (default: ~/.immoweb-pp-cli/store.db)")
	return cmd
}
