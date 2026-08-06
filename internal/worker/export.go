package worker

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bitorsic/export-service/internal/jobs"
)

const exportsDir = "exports"

// returned when the export query matched zero rows for the given seller/date range
var ErrNoData = errors.New("no data found for the given seller and date range")

// stream the results to a CSV file, return the path to the written file
func runExport(ctx context.Context, pool *pgxpool.Pool, job *jobs.Job) (string, error) {
	rows, err := queryOrderExport(ctx, pool, job)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("running export query: %w", err)
		}
		return "", ErrNoData
	}

	return writeExportCSV(ctx, job.ID, rows)
}

// IMPORTANT //
func queryOrderExport(ctx context.Context, pool *pgxpool.Pool, job *jobs.Job) (pgx.Rows, error) {
	rows, err := pool.Query(ctx, `
		SELECT o.order_id, o.order_status, o.order_purchase_timestamp,
		       c.customer_id, c.customer_city, c.customer_state,
		       p.product_category,
		       oi.price, oi.freight_value
		FROM order_items oi
		JOIN orders o ON o.order_id = oi.order_id
		JOIN customers c ON c.customer_id = o.customer_id
		JOIN products p ON p.product_id = oi.product_id
		WHERE oi.seller_id = $1
		  AND o.order_purchase_timestamp BETWEEN $2 AND $3
	`, job.SellerID, job.DateFrom, job.DateTo)
	if err != nil {
		return nil, fmt.Errorf("running export query: %w", err)
	}
	return rows, nil
}

// streams rows to a CSV file, one at a time
func writeExportCSV(ctx context.Context, jobID string, rows pgx.Rows) (string, error) {
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		return "", fmt.Errorf("creating exports dir: %w", err)
	}

	filePath := filepath.Join(exportsDir, jobID+".csv")
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"order_id", "order_status", "order_purchase_timestamp",
		"customer_id", "customer_city", "customer_state",
		"product_category", "price", "freight_value",
	}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("writing csv header: %w", err)
	}

	// already advanced the cursor once
	for {
		if ctx.Err() != nil {
			return "", fmt.Errorf("export cancelled: %w", ctx.Err())
		}

		if err := writeRow(writer, rows); err != nil {
			return "", err
		}

		if !rows.Next() {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterating rows: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flushing csv: %w", err)
	}

	return filePath, nil
}

// scans the current row and writes it as one CSV record
func writeRow(writer *csv.Writer, rows pgx.Rows) error {
	var (
		orderID, status, customerID, city, state, category string
		purchasedAt                                        time.Time
		price, freight                                     float64
	)
	if err := rows.Scan(&orderID, &status, &purchasedAt, &customerID, &city, &state, &category, &price, &freight); err != nil {
		return fmt.Errorf("scanning row: %w", err)
	}

	record := []string{
		orderID, status, purchasedAt.Format(time.RFC3339),
		customerID, city, state, category,
		fmt.Sprintf("%.2f", price), fmt.Sprintf("%.2f", freight),
	}
	if err := writer.Write(record); err != nil {
		return fmt.Errorf("writing csv row: %w", err)
	}
	return nil
}
