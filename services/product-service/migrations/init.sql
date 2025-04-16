-- Схема для мультитенантной архитектуры
CREATE SCHEMA IF NOT EXISTS product;

-- Таблица продуктов
CREATE TABLE IF NOT EXISTS product.products (
                                                id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    supplier_id VARCHAR(36) NOT NULL,
    base_data JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
                             PRIMARY KEY (id, tenant_id)
    );

-- Индексы для таблицы продуктов
CREATE INDEX IF NOT EXISTS idx_products_tenant_supplier ON product.products(tenant_id, supplier_id);
CREATE INDEX IF NOT EXISTS idx_products_updated_at ON product.products(updated_at);
CREATE INDEX IF NOT EXISTS idx_products_base_data_gin ON product.products USING gin (base_data);

-- Таблица инвентаря продуктов
CREATE TABLE IF NOT EXISTS product.inventory (
                                                 product_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    supplier_id VARCHAR(36) NOT NULL,
    quantity INTEGER NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
                             PRIMARY KEY (product_id, tenant_id),
    FOREIGN KEY (product_id, tenant_id) REFERENCES product.products(id, tenant_id) ON DELETE CASCADE
    );

-- Таблица цен продуктов
CREATE TABLE IF NOT EXISTS product.prices (
                                              product_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    supplier_id VARCHAR(36) NOT NULL,
    base_price DECIMAL(15, 2) NOT NULL,
    special_price DECIMAL(15, 2),
    currency VARCHAR(3) NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE,
    end_date TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
                             PRIMARY KEY (product_id, tenant_id),
    FOREIGN KEY (product_id, tenant_id) REFERENCES product.products(id, tenant_id) ON DELETE CASCADE
    );

-- Таблица медиафайлов продуктов
CREATE TABLE IF NOT EXISTS product.media (
                                             id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    type VARCHAR(10) NOT NULL, -- 'image', 'video', etc.
    url TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
                             PRIMARY KEY (id, tenant_id),
    FOREIGN KEY (product_id, tenant_id) REFERENCES product.products(id, tenant_id) ON DELETE CASCADE
    );

CREATE INDEX IF NOT EXISTS idx_media_product ON product.media(product_id, tenant_id);

-- Таблица категорий продуктов
CREATE TABLE IF NOT EXISTS product.categories (
    id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    parent_id VARCHAR(36),
    level INTEGER NOT NULL,
    path TEXT NOT NULL,
    image_url TEXT,
    PRIMARY KEY (id, tenant_id),
    FOREIGN KEY (parent_id, tenant_id) REFERENCES product.categories(id, tenant_id) ON DELETE SET NULL
    );

CREATE INDEX IF NOT EXISTS idx_categories_parent ON product.categories(parent_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_categories_path ON product.categories(path);

-- Таблица связей продуктов и категорий
CREATE TABLE IF NOT EXISTS product.product_categories (
                                                          product_id VARCHAR(36) NOT NULL,
    category_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    PRIMARY KEY (product_id, category_id, tenant_id),
    FOREIGN KEY (product_id, tenant_id) REFERENCES product.products(id, tenant_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id, tenant_id) REFERENCES product.categories(id, tenant_id) ON DELETE CASCADE
    );

-- Таблица истории изменений продуктов
CREATE TABLE IF NOT EXISTS product.history (
                                               id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    change_type VARCHAR(10) NOT NULL, -- 'create', 'update', 'delete'
    before JSONB,
    after JSONB,
    changed_by VARCHAR(255),
    changed_at BIGINT NOT NULL, -- Unix timestamp
    change_comment TEXT,
    PRIMARY KEY (id, tenant_id)
    );

CREATE INDEX IF NOT EXISTS idx_history_product ON product.history(product_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_history_changed_at ON product.history(changed_at);