CREATE TABLE sellers (
    seller_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_name TEXT NOT NULL
);

CREATE TABLE customers (
    customer_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_city TEXT NOT NULL,
    customer_state TEXT NOT NULL
);

CREATE TABLE products (
    product_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_category TEXT NOT NULL
);

CREATE TABLE orders (
    order_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(customer_id),
    order_status TEXT NOT NULL, -- 'delivered' | 'shipped' | 'canceled' etc.
    order_purchase_timestamp TIMESTAMPTZ NOT NULL
);

CREATE TABLE order_items (
    order_item_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(order_id),
    product_id UUID NOT NULL REFERENCES products(product_id),
    seller_id UUID NOT NULL REFERENCES sellers(seller_id),
    price NUMERIC(10,2) NOT NULL,
    freight_value NUMERIC(10,2) NOT NULL
);