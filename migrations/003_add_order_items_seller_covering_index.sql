CREATE INDEX idx_order_items_seller_covering
  ON order_items(seller_id) INCLUDE (order_id, product_id, price, freight_value);