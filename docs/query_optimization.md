# Query optimization notes

The export endpoint joins order_items, orders, customers, and products,
filtered by seller and date range. Tested against a seller with a large
order history, on a table with a few million rows.

## No index

`EXPLAIN ANALYZE` showed a full sequential scan on order_items (1.5M rows
scanned to find ~36K matches) and on orders (141K rows discarded by the
date filter).

**Execution time: ~160ms**

## Plain index on seller_id

```sql
CREATE INDEX idx_order_items_seller_id ON order_items(seller_id);
```

**Execution time: ~228ms (worse)**

The index found matching rows fast, but each one still needed a separate
lookup on the table to get the other columns. Those rows were scattered
across disk, so the random reads cost more than the original sequential
scan did.

## Covering index

```sql
CREATE INDEX idx_order_items_seller_covering
  ON order_items(seller_id) INCLUDE (order_id, product_id, price, freight_value);
```

Query is now answered directly from the index, no table lookup needed.

**Execution time: ~108ms**

## Summary

| Version | Execution time |
|---|---|
| No index | ~160ms |
| Plain index | ~228ms |
| Covering index | ~108ms |

## Write tradeoff

This index adds write overhead: every insert into order_items now updates
a wider index. Fine here since the data is bulk-loaded once and only read
after. On a table with continuous writes, I'd measure insert cost before
committing to an index this wide.