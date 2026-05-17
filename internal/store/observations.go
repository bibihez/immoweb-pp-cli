// Copyright 2026 bibihez. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"fmt"
)

// AppendObservationsBatch appends one row per item to the observations log,
// snapshotting the search-results fields that the temporal-analytics layer
// (watch, drops, stale, market) queries. Designed for Immoweb's classified
// shape; tolerates extra/missing fields by zeroing them rather than failing
// the whole batch.
//
// The (listing_id, observed_at) primary key means duplicate calls within the
// same second collapse harmlessly via INSERT OR IGNORE.
func (s *Store) AppendObservationsBatch(items []json.RawMessage) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO observations (
			listing_id, price, customer_name, flag_main,
			bedroom_count, surface, postal_code, locality,
			property_type, property_subtype
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	type listingShape struct {
		ID           int64  `json:"id"`
		CustomerName string `json:"customerName"`
		Flags        struct {
			Main string `json:"main"`
		} `json:"flags"`
		Price struct {
			MainValue float64 `json:"mainValue"`
		} `json:"price"`
		Property struct {
			Type                string  `json:"type"`
			Subtype             string  `json:"subtype"`
			BedroomCount        int     `json:"bedroomCount"`
			NetHabitableSurface float64 `json:"netHabitableSurface"`
			Location            struct {
				PostalCode string `json:"postalCode"`
				Locality   string `json:"locality"`
			} `json:"location"`
		} `json:"property"`
	}

	inserted := 0
	for _, raw := range items {
		var x listingShape
		if err := json.Unmarshal(raw, &x); err != nil {
			continue
		}
		if x.ID == 0 {
			continue
		}
		var priceVal interface{}
		if x.Price.MainValue > 0 {
			priceVal = int64(x.Price.MainValue)
		}
		var surfVal interface{}
		if x.Property.NetHabitableSurface > 0 {
			surfVal = x.Property.NetHabitableSurface
		}
		var bedVal interface{}
		if x.Property.BedroomCount > 0 {
			bedVal = x.Property.BedroomCount
		}
		if _, err := stmt.Exec(
			x.ID, priceVal, x.CustomerName, x.Flags.Main,
			bedVal, surfVal, x.Property.Location.PostalCode, x.Property.Location.Locality,
			x.Property.Type, x.Property.Subtype,
		); err != nil {
			continue
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}
