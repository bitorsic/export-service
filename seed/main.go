package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- Tunable volumes -------------------------------------------------

const (
	numSellers   = 500
	numCustomers = 200_000
	numProducts  = 2_000
	numOrders    = 1_000_000
	numItems     = 3_000_000

	copyBatchSize = 50_000 // rows per COPY batch
)

// power-law skew: this fraction of sellers get this fraction of order_items
const (
	powerSellerFraction = 0.05
	powerSellerShare    = 0.60
)

var productCategories = []string{
	"electronics", "home_appliances", "toys", "books", "fashion",
	"sports", "beauty", "furniture", "groceries", "automotive",
}

var orderStatuses = []string{"delivered", "shipped", "processing", "canceled"}

func main() {
	ctx := context.Background()
	gofakeit.Seed(42) // deterministic, reproducible data across runs

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set (e.g. postgres://user:pass@localhost:5432/exports)")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connecting to db: %v", err)
	}
	defer pool.Close()

	start := time.Now()

	sellerIDs := seedSellers(ctx, pool)
	log.Printf("seeded %d sellers (%s elapsed)", len(sellerIDs), time.Since(start))

	customerIDs := seedCustomers(ctx, pool)
	log.Printf("seeded %d customers (%s elapsed)", len(customerIDs), time.Since(start))

	productIDs := seedProducts(ctx, pool)
	log.Printf("seeded %d products (%s elapsed)", len(productIDs), time.Since(start))

	orderIDs := seedOrders(ctx, pool, customerIDs)
	log.Printf("seeded %d orders (%s elapsed)", len(orderIDs), time.Since(start))

	seedOrderItems(ctx, pool, orderIDs, productIDs, sellerIDs)
	log.Printf("seeded %d order_items (%s elapsed) — done", numItems, time.Since(start))
}

func seedSellers(ctx context.Context, pool *pgxpool.Pool) []string {
	rows := make([][]any, 0, numSellers)
	ids := make([]string, 0, numSellers)

	for range numSellers {
		id := gofakeit.UUID()
		ids = append(ids, id)
		rows = append(rows, []any{id, gofakeit.Company()})
	}

	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"sellers"},
		[]string{"seller_id", "seller_name"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		log.Fatalf("copying sellers: %v", err)
	}
	return ids
}

func seedCustomers(ctx context.Context, pool *pgxpool.Pool) []string {
	rows := make([][]any, 0, copyBatchSize)
	ids := make([]string, 0, numCustomers)

	for i := range numCustomers {
		id := gofakeit.UUID()
		ids = append(ids, id)
		rows = append(rows, []any{id, gofakeit.City(), gofakeit.StateAbr()})

		if len(rows) == copyBatchSize || i == numCustomers-1 {
			_, err := pool.CopyFrom(ctx,
				pgx.Identifier{"customers"},
				[]string{"customer_id", "customer_city", "customer_state"},
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				log.Fatalf("copying customers: %v", err)
			}
			rows = rows[:0]
		}
	}
	return ids
}

func seedProducts(ctx context.Context, pool *pgxpool.Pool) []string {
	rows := make([][]any, 0, numProducts)
	ids := make([]string, 0, numProducts)

	for range numProducts {
		id := gofakeit.UUID()
		ids = append(ids, id)
		category := productCategories[rand.Intn(len(productCategories))]
		rows = append(rows, []any{id, category})
	}

	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"products"},
		[]string{"product_id", "product_category"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		log.Fatalf("copying products: %v", err)
	}
	return ids
}

// randomTimestampWithinLast2Years returns a purchase timestamp spread over
// roughly the last 2 years, so a date-range filter in the export query has
// real, uneven data to work with.
func randomTimestampWithinLast2Years() time.Time {
	now := time.Now()
	twoYearsAgo := now.AddDate(-2, 0, 0)
	delta := now.Unix() - twoYearsAgo.Unix()
	sec := rand.Int63n(delta)
	return twoYearsAgo.Add(time.Duration(sec) * time.Second)
}

func seedOrders(ctx context.Context, pool *pgxpool.Pool, customerIDs []string) []string {
	ids := make([]string, 0, numOrders)
	rows := make([][]any, 0, copyBatchSize)

	for i := range numOrders {
		id := gofakeit.UUID()
		ids = append(ids, id)

		custID := customerIDs[rand.Intn(len(customerIDs))]
		status := orderStatuses[rand.Intn(len(orderStatuses))]
		ts := randomTimestampWithinLast2Years()

		rows = append(rows, []any{id, custID, status, ts})

		if len(rows) == copyBatchSize || i == numOrders-1 {
			_, err := pool.CopyFrom(ctx,
				pgx.Identifier{"orders"},
				[]string{"order_id", "customer_id", "order_status", "order_purchase_timestamp"},
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				log.Fatalf("copying orders (batch ending at %d): %v", i, err)
			}
			rows = rows[:0]
		}
	}
	return ids
}

// buildSellerWeights returns, for each seller index, a cumulative weight
// used to bias random selection so `powerSellerFraction` of sellers receive
// `powerSellerShare` of all order_items.
func buildSellerWeights(n int) []float64 {
	powerCount := int(float64(n) * powerSellerFraction)
	if powerCount == 0 {
		powerCount = 1
	}
	weights := make([]float64, n)

	powerWeightEach := powerSellerShare / float64(powerCount)
	normalWeightEach := (1 - powerSellerShare) / float64(n-powerCount)

	for i := range n {
		if i < powerCount {
			weights[i] = powerWeightEach
		} else {
			weights[i] = normalWeightEach
		}
	}

	// convert to cumulative distribution for weighted random pick
	cumulative := make([]float64, n)
	sum := 0.0
	for i, w := range weights {
		sum += w
		cumulative[i] = sum
	}
	return cumulative
}

func pickWeightedSeller(cumulative []float64, sellerIDs []string) string {
	r := rand.Float64()
	for i, c := range cumulative {
		if r <= c {
			return sellerIDs[i]
		}
	}
	return sellerIDs[len(sellerIDs)-1] // fallback for float rounding edge case
}

func seedOrderItems(ctx context.Context, pool *pgxpool.Pool, orderIDs, productIDs, sellerIDs []string) {
	cumulativeWeights := buildSellerWeights(len(sellerIDs))

	rows := make([][]any, 0, copyBatchSize)

	for i := range numItems {
		id := gofakeit.UUID()
		orderID := orderIDs[rand.Intn(len(orderIDs))]
		productID := productIDs[rand.Intn(len(productIDs))]
		sellerID := pickWeightedSeller(cumulativeWeights, sellerIDs)

		price := gofakeit.Price(5, 2000)
		freight := gofakeit.Price(5, 100)

		rows = append(rows, []any{id, orderID, productID, sellerID, price, freight})

		if len(rows) == copyBatchSize || i == numItems-1 {
			_, err := pool.CopyFrom(ctx,
				pgx.Identifier{"order_items"},
				[]string{"order_item_id", "order_id", "product_id", "seller_id", "price", "freight_value"},
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				log.Fatalf("copying order_items (batch ending at %d): %v", i, err)
			}
			rows = rows[:0]
			fmt.Printf("  order_items: %d / %d\n", i+1, numItems)
		}
	}
}
