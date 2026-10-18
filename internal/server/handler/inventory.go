package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"infineon/internal/auth"
	"infineon/internal/events"
	"infineon/internal/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── Products ──────────────────────────────────────────────

type CreateProductRequest struct {
	SKU            string  `json:"sku"`
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	Subcategory    string  `json:"subcategory"`
	UnitOfMeasure  string  `json:"unit_of_measure"`
	CostPrice      float64 `json:"cost_price"`
	SellingPrice   float64 `json:"selling_price"`
	ReorderLevel   float64 `json:"reorder_level"`
	MaximumStock   float64 `json:"maximum_stock"`
	MinimumStock   float64 `json:"minimum_stock"`
	SupplierID     string  `json:"supplier_id"`
	Barcode        string  `json:"barcode"`
	TaxRate        float64 `json:"tax_rate"`
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	search := r.URL.Query().Get("search")
	category := r.URL.Query().Get("category")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	query := `SELECT id, sku, name, category, subcategory, unit_of_measure, cost_price, 
		selling_price, reorder_level, barcode, is_active
		FROM inventory_products WHERE property_id = $1 AND deleted_at IS NULL`
	args := []interface{}{propertyID}
	argIdx := 2

	if search != "" {
		query += ` AND (name ILIKE $` + strconv.Itoa(argIdx) + ` OR sku ILIKE $` + strconv.Itoa(argIdx) + ` OR barcode ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if category != "" {
		query += ` AND category = $` + strconv.Itoa(argIdx)
		args = append(args, category)
		argIdx++
	}

	var total int64
	h.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM inventory_products WHERE property_id = $1 AND deleted_at IS NULL", propertyID).Scan(&total)

	query += ` ORDER BY name LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, perPage, offset)

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var sku, name, uom string
		var category, subcategory, barcode *string
		var costPrice, sellingPrice, reorderLevel float64
		var isActive bool
		rows.Scan(&id, &sku, &name, &category, &subcategory, &uom, &costPrice, &sellingPrice, &reorderLevel, &barcode, &isActive)
		p := map[string]interface{}{
			"id": id, "sku": sku, "name": name, "unit_of_measure": uom,
			"cost_price": costPrice, "selling_price": sellingPrice,
			"reorder_level": reorderLevel, "is_active": isActive,
		}
		if category != nil {
			p["category"] = *category
		}
		if subcategory != nil {
			p["subcategory"] = *subcategory
		}
		if barcode != nil {
			p["barcode"] = *barcode
		}
		products = append(products, p)
	}
	if products == nil {
		products = []map[string]interface{}{}
	}
	response.RespondPaginated(w, products, total, page, perPage)
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.SKU == "" || req.Name == "" {
		response.RespondError(w, "SKU and name are required", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO inventory_products (id, property_id, sku, name, category, subcategory, unit_of_measure,
			cost_price, selling_price, reorder_level, maximum_stock, minimum_stock, supplier_id, barcode, tax_rate, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, TRUE)
	`, id, propertyID, req.SKU, req.Name, req.Category, req.Subcategory, req.UnitOfMeasure,
		req.CostPrice, req.SellingPrice, req.ReorderLevel, req.MaximumStock, req.MinimumStock,
		uuidOrEmpty(req.SupplierID), req.Barcode, req.TaxRate)
	if err != nil {
		response.RespondError(w, "Failed to create product (SKU may already exist)", http.StatusConflict)
		return
	}
	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Product created successfully", http.StatusCreated)
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sku, name, uom string
	var category, subcategory, barcode *string
	var costPrice, sellingPrice, reorderLevel float64

	err := h.db.QueryRow(context.Background(), `
		SELECT sku, name, category, subcategory, unit_of_measure, cost_price, selling_price, reorder_level, barcode
		FROM inventory_products WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&sku, &name, &category, &subcategory, &uom, &costPrice, &sellingPrice, &reorderLevel, &barcode)
	if err != nil {
		response.RespondError(w, "Product not found", http.StatusNotFound)
		return
	}
	result := map[string]interface{}{
		"id": id, "sku": sku, "name": name, "unit_of_measure": uom,
		"cost_price": costPrice, "selling_price": sellingPrice, "reorder_level": reorderLevel,
	}
	if category != nil {
		result["category"] = *category
	}
	if subcategory != nil {
		result["subcategory"] = *subcategory
	}
	if barcode != nil {
		result["barcode"] = *barcode
	}
	response.RespondSuccess(w, result, "Product retrieved successfully")
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(context.Background(), `
		UPDATE inventory_products SET
			name = COALESCE(NULLIF($2, ''), name), category = NULLIF($3, ''),
			subcategory = NULLIF($4, ''), unit_of_measure = COALESCE(NULLIF($5, ''), unit_of_measure),
			cost_price = $6, selling_price = $7, reorder_level = $8, barcode = NULLIF($9, ''),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, req.Name, req.Category, req.Subcategory, req.UnitOfMeasure,
		req.CostPrice, req.SellingPrice, req.ReorderLevel, req.Barcode)
	if err != nil {
		response.RespondError(w, "Failed to update product", http.StatusInternalServerError)
		return
	}
	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Product updated successfully")
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.db.Exec(context.Background(), "UPDATE inventory_products SET deleted_at = NOW() WHERE id = $1", id)
	response.RespondSuccess(w, nil, "Product deleted successfully")
}

// ── Stock Movements ───────────────────────────────────────

func (h *Handler) GetStock(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	storeID := r.URL.Query().Get("store_id")
	productID := r.URL.Query().Get("product_id")

	query := `SELECT s.id, s.product_id, p.name, p.sku, s.store_id, st.name as store_name, 
		s.quantity, s.batch_number, s.expiry_date, s.cost_price
		FROM inventory_stock s
		JOIN inventory_products p ON s.product_id = p.id
		JOIN inventory_stores st ON s.store_id = st.id
		WHERE st.property_id = $1`
	args := []interface{}{propertyID}
	argIdx := 2

	if storeID != "" {
		query += ` AND s.store_id = $` + strconv.Itoa(argIdx)
		args = append(args, storeID)
		argIdx++
	}
	if productID != "" {
		query += ` AND s.product_id = $` + strconv.Itoa(argIdx)
		args = append(args, productID)
		argIdx++
	}
	query += ` ORDER BY p.name`

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		response.RespondError(w, "Failed to query stock", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var stock []map[string]interface{}
	for rows.Next() {
		var id, prodID, storeIDVal uuid.UUID
		var prodName, sku, storeName string
		var quantity float64
		var batchNumber *string
		var expiryDate, costPrice interface{}
		rows.Scan(&id, &prodID, &prodName, &sku, &storeIDVal, &storeName, &quantity, &batchNumber, &expiryDate, &costPrice)
		s := map[string]interface{}{
			"id": id, "product_id": prodID, "product_name": prodName, "sku": sku,
			"store_id": storeIDVal, "store_name": storeName, "quantity": quantity,
		}
		if batchNumber != nil {
			s["batch_number"] = *batchNumber
		}
		stock = append(stock, s)
	}
	if stock == nil {
		stock = []map[string]interface{}{}
	}
	response.RespondSuccess(w, stock, "Stock retrieved successfully")
}

type StockMovementRequest struct {
	ProductID     string  `json:"product_id"`
	FromStoreID   string  `json:"from_store_id"`
	ToStoreID     string  `json:"to_store_id"`
	Quantity      float64 `json:"quantity"`
	UnitCost      float64 `json:"unit_cost"`
	ReferenceType string  `json:"reference_type"`
	ReferenceID   string  `json:"reference_id"`
	Reason        string  `json:"reason"`
	BatchNumber   string  `json:"batch_number"`
}

func (h *Handler) RecordStockMovement(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req StockMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	movID := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO inventory_movements (id, property_id, product_id, from_store_id, to_store_id, 
			quantity, unit_cost, reference_type, reference_id, reason, batch_number, performed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, movID, propertyID, req.ProductID, uuidOrEmpty(req.FromStoreID), uuidOrEmpty(req.ToStoreID),
		req.Quantity, req.UnitCost, req.ReferenceType, uuidOrEmpty(req.ReferenceID),
		req.Reason, req.BatchNumber, createdBy)
	if err != nil {
		response.RespondError(w, "Failed to record stock movement", http.StatusInternalServerError)
		return
	}

	// Update stock quantity in destination store
	if req.ToStoreID != "" {
		h.db.Exec(context.Background(), `
			INSERT INTO inventory_stock (id, product_id, store_id, quantity, cost_price, batch_number)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (product_id, store_id, batch_number) DO UPDATE SET 
				quantity = inventory_stock.quantity + $4, cost_price = $5, updated_at = NOW()
		`, uuid.New(), req.ProductID, req.ToStoreID, req.Quantity, req.UnitCost, req.BatchNumber)
	}

	// Reduce stock from source store
	if req.FromStoreID != "" {
		h.db.Exec(context.Background(), `
			UPDATE inventory_stock SET quantity = quantity - $3, updated_at = NOW()
			WHERE product_id = $1 AND store_id = $2 AND quantity >= $3
		`, req.ProductID, req.FromStoreID, req.Quantity)
	}

	// ADR-0006: publish the movement — the M-INV subscriber runs the reorder
	// check (drafting a replenishment PO or raising stock.low) without this
	// handler knowing anything about procurement.
	var sku string
	var productReorder float64
	h.db.QueryRow(context.Background(), `
		SELECT COALESCE(sku, ''), COALESCE(reorder_level, 0)
		FROM inventory_products WHERE id = $1
	`, req.ProductID).Scan(&sku, &productReorder)

	var qtyOnHand float64
	h.db.QueryRow(context.Background(), `
		SELECT COALESCE(SUM(quantity), 0) FROM inventory_stock WHERE product_id = $1
	`, req.ProductID).Scan(&qtyOnHand)

	movementType := "adjust"
	if req.ToStoreID != "" {
		movementType = "receive"
	} else if req.FromStoreID != "" {
		movementType = "issue"
	}

	movEv, err := h.emitDomainEvent(r.Context(), h.db, propertyID, eventActor(r),
		events.StockMovement, "inventory", req.ProductID, map[string]interface{}{
			"propertyId":   propertyID,
			"itemId":       req.ProductID,
			"sku":          sku,
			"type":         movementType,
			"delta":        req.Quantity,
			"qtyOnHand":    qtyOnHand,
			"reorderLevel": productReorder,
		})
	if err != nil {
		response.RespondError(w, "Failed to record domain event", http.StatusInternalServerError)
		return
	}
	h.ScheduleEvent(movEv)

	response.RespondSuccess(w, map[string]interface{}{"id": movID}, "Stock movement recorded successfully", http.StatusCreated)
}

// ── Stores ────────────────────────────────────────────────

func (h *Handler) ListStores(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	rows, err := h.db.Query(context.Background(), `
		SELECT id, name, code, type, location, status FROM inventory_stores 
		WHERE property_id = $1 AND deleted_at IS NULL ORDER BY name
	`, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to query stores", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var stores []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, code, storeType, status string
		var location *string
		rows.Scan(&id, &name, &code, &storeType, &location, &status)
		s := map[string]interface{}{
			"id": id, "name": name, "code": code, "type": storeType, "status": status,
		}
		if location != nil {
			s["location"] = *location
		}
		stores = append(stores, s)
	}
	if stores == nil {
		stores = []map[string]interface{}{}
	}
	response.RespondSuccess(w, stores, "Stores retrieved successfully")
}

func (h *Handler) CreateStore(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req struct {
		Name     string `json:"name"`
		Code     string `json:"code"`
		Type     string `json:"type"`
		Location string `json:"location"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO inventory_stores (id, property_id, name, code, type, location, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')
	`, id, propertyID, req.Name, req.Code, req.Type, req.Location)
	if err != nil {
		response.RespondError(w, "Failed to create store", http.StatusInternalServerError)
		return
	}
	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Store created successfully", http.StatusCreated)
}

// ── Suppliers ─────────────────────────────────────────────

func (h *Handler) ListSuppliers(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	rows, err := h.db.Query(context.Background(), `
		SELECT id, name, code, contact_person, email, phone, credit_limit, status
		FROM suppliers WHERE property_id = $1 AND deleted_at IS NULL ORDER BY name
	`, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to query suppliers", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var suppliers []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, code, status string
		var contactPerson, email, phone *string
		var creditLimit float64
		rows.Scan(&id, &name, &code, &contactPerson, &email, &phone, &creditLimit, &status)
		s := map[string]interface{}{
			"id": id, "name": name, "code": code, "credit_limit": creditLimit, "status": status,
		}
		if contactPerson != nil {
			s["contact_person"] = *contactPerson
		}
		if email != nil {
			s["email"] = *email
		}
		if phone != nil {
			s["phone"] = *phone
		}
		suppliers = append(suppliers, s)
	}
	if suppliers == nil {
		suppliers = []map[string]interface{}{}
	}
	response.RespondSuccess(w, suppliers, "Suppliers retrieved successfully")
}

func (h *Handler) CreateSupplier(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req struct {
		Name          string  `json:"name"`
		Code          string  `json:"code"`
		ContactPerson string  `json:"contact_person"`
		Email         string  `json:"email"`
		Phone         string  `json:"phone"`
		Address       string  `json:"address"`
		CreditLimit   float64 `json:"credit_limit"`
		PaymentTerms  int     `json:"payment_terms_days"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := uuid.New()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO suppliers (id, property_id, name, code, contact_person, email, phone, address, 
			credit_limit, payment_terms_days, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active')
	`, id, propertyID, req.Name, req.Code, req.ContactPerson, req.Email, req.Phone, req.Address,
		req.CreditLimit, req.PaymentTerms)
	if err != nil {
		response.RespondError(w, "Failed to create supplier", http.StatusInternalServerError)
		return
	}
	response.RespondSuccess(w, map[string]interface{}{"id": id}, "Supplier created successfully", http.StatusCreated)
}

// ── Purchase Orders ───────────────────────────────────────

func (h *Handler) ListPurchaseOrders(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	rows, err := h.db.Query(context.Background(), `
		SELECT po.id, po.po_number, s.name as supplier_name, po.order_date, po.expected_date,
			po.total, po.status
		FROM purchase_orders po
		JOIN suppliers s ON po.supplier_id = s.id
		WHERE po.property_id = $1 ORDER BY po.created_at DESC LIMIT 100
	`, propertyID)
	if err != nil {
		response.RespondError(w, "Failed to query purchase orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var poNumber, supplierName, status string
		var orderDate, expectedDate interface{}
		var total float64
		rows.Scan(&id, &poNumber, &supplierName, &orderDate, &expectedDate, &total, &status)
		orders = append(orders, map[string]interface{}{
			"id": id, "po_number": poNumber, "supplier_name": supplierName,
			"order_date": orderDate, "expected_date": expectedDate,
			"total": total, "status": status,
		})
	}
	if orders == nil {
		orders = []map[string]interface{}{}
	}
	response.RespondSuccess(w, orders, "Purchase orders retrieved successfully")
}

func (h *Handler) CreatePurchaseOrder(w http.ResponseWriter, r *http.Request) {
	propertyID := r.Header.Get("X-Property-ID")
	var req struct {
		SupplierID  string         `json:"supplier_id"`
		ExpectedDate string       `json:"expected_date"`
		Items       []struct {
			ProductID string  `json:"product_id"`
			Quantity  float64 `json:"quantity"`
			UnitPrice float64 `json:"unit_price"`
		} `json:"items"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.SupplierID == "" {
		response.RespondError(w, "supplier_id is required", http.StatusBadRequest)
		return
	}

	var createdBy *uuid.UUID
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		createdBy = &claims.UserID
	}

	var total float64
	for _, item := range req.Items {
		total += item.Quantity * item.UnitPrice
	}

	poID := uuid.New()
	poNumber := fmt.Sprintf("PO-%s", generateConfirmationNumber()[:8])

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		response.RespondError(w, "Transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(), `
		INSERT INTO purchase_orders (id, property_id, po_number, supplier_id, order_date, expected_date, total, status, created_by)
		VALUES ($1, $2, $3, $4, CURRENT_DATE, $5, $6, 'pending', $7)
	`, poID, propertyID, poNumber, req.SupplierID, req.ExpectedDate, total, createdBy)
	if err != nil {
		response.RespondError(w, "Failed to create PO", http.StatusInternalServerError)
		return
	}

	for _, item := range req.Items {
		lineTotal := item.Quantity * item.UnitPrice
		tx.Exec(r.Context(), `
			INSERT INTO purchase_order_lines (id, po_id, product_id, quantity, unit_price, amount)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, uuid.New(), poID, item.ProductID, item.Quantity, item.UnitPrice, lineTotal)
	}

	tx.Commit(r.Context())

	response.RespondSuccess(w, map[string]interface{}{
		"id": poID, "po_number": poNumber, "total": total,
	}, "Purchase order created successfully", http.StatusCreated)
}
