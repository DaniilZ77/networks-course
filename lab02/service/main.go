package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type Product struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"`
}

type ProductStore struct {
	mu       *sync.Mutex
	products map[uint64]Product
}

func (ps *ProductStore) Get(id uint64) (Product, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	product, ok := ps.products[id]
	return product, ok
}

func (ps *ProductStore) GetAll() []Product {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	res := make([]Product, 0)
	for _, p := range ps.products {
		res = append(res, p)
	}
	return res
}

func (ps *ProductStore) Create(product Product) (Product, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if product, ok := ps.products[product.ID]; ok {
		return product, false
	}

	ps.products[product.ID] = product
	return product, true
}

func (ps *ProductStore) Update(id uint64, upd func(Product) Product) (Product, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	product, ok := ps.products[id]
	if !ok {
		return product, false
	}

	product = upd(product)
	ps.products[id] = product
	return product, true
}

func (ps *ProductStore) Delete(id uint64) (Product, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	product, ok := ps.products[id]
	if !ok {
		return product, false
	}

	delete(ps.products, id)
	return product, true
}

func saveImage(image io.Reader, dstPath string) error {
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	dstIcon, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dstIcon.Close()

	_, err = io.Copy(dstIcon, image)
	if err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("failed to save image: %w", err)
	}
	return nil
}

func main() {
	responseJSON := func(w http.ResponseWriter, code int, data any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if err, ok := data.(error); ok && err != nil {
			log.Println(err.Error())
			data = map[string]any{"message": err.Error()}
		}
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Println(err)
		}
	}

	store := ProductStore{
		mu:       &sync.Mutex{},
		products: map[uint64]Product{},
	}

	http.HandleFunc("POST /product", func(w http.ResponseWriter, r *http.Request) {
		var product Product
		if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		id := rand.Uint32()
		product.ID = uint64(id)

		res, ok := store.Create(product)
		if !ok {
			responseJSON(w, http.StatusInternalServerError, fmt.Errorf("product with same id already exists"))
			return
		}

		responseJSON(w, http.StatusCreated, res)
	})
	http.HandleFunc("GET /product/{product_id}", func(w http.ResponseWriter, r *http.Request) {
		strID := r.PathValue("product_id")
		id, err := strconv.ParseUint(strID, 10, 64)
		if err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		product, ok := store.Get(id)
		if !ok {
			responseJSON(w, http.StatusNotFound, fmt.Errorf("product not found"))
			return
		}

		responseJSON(w, http.StatusOK, product)
	})
	http.HandleFunc("PUT /product/{product_id}", func(w http.ResponseWriter, r *http.Request) {
		type update struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
		}
		strID := r.PathValue("product_id")
		id, err := strconv.ParseUint(strID, 10, 64)
		if err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		var upd update
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		product, ok := store.Update(id, func(p Product) Product {
			if upd.Name != nil {
				p.Name = *upd.Name
			}
			if upd.Description != nil {
				p.Description = *upd.Description
			}
			return p
		})
		if !ok {
			responseJSON(w, http.StatusNotFound, fmt.Errorf("product not found"))
			return
		}

		responseJSON(w, http.StatusOK, product)
	})
	http.HandleFunc("DELETE /product/{product_id}", func(w http.ResponseWriter, r *http.Request) {
		strID := r.PathValue("product_id")
		id, err := strconv.ParseUint(strID, 10, 64)
		if err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		product, ok := store.Delete(id)
		if !ok {
			responseJSON(w, http.StatusNotFound, fmt.Errorf("product not found"))
			return
		}

		responseJSON(w, http.StatusOK, product)
	})
	http.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		responseJSON(w, http.StatusOK, store.GetAll())
	})
	http.HandleFunc("POST /product/{product_id}/image", func(w http.ResponseWriter, r *http.Request) {
		strID := r.PathValue("product_id")
		id, err := strconv.ParseUint(strID, 10, 64)
		if err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		icon, header, err := r.FormFile("icon")
		if err != nil {
			responseJSON(w, http.StatusBadRequest, fmt.Errorf("failed to get form file: %w", err))
			return
		}
		defer icon.Close()

		if header.Header.Get("Content-Type") != "image/png" {
			responseJSON(w, http.StatusBadRequest, fmt.Errorf("only png images are allowed"))
			return
		}

		if header.Size > 1_000_000 {
			responseJSON(w, http.StatusBadRequest, fmt.Errorf("icon size must be less than 1MB"))
			return
		}

		iconPath := filepath.Join("icons", "icon_"+strconv.FormatInt(time.Now().UnixNano(), 10))
		if err := saveImage(icon, iconPath); err != nil {
			responseJSON(w, http.StatusInternalServerError, err)
			return
		}

		product, ok := store.Update(id, func(p Product) Product {
			p.Icon = iconPath
			return p
		})
		if !ok {
			_ = os.Remove(iconPath)
			responseJSON(w, http.StatusNotFound, fmt.Errorf("product not found"))
			return
		}

		responseJSON(w, http.StatusOK, product)
	})
	http.HandleFunc("GET /product/{product_id}/image", func(w http.ResponseWriter, r *http.Request) {
		strID := r.PathValue("product_id")
		id, err := strconv.ParseUint(strID, 10, 64)
		if err != nil {
			responseJSON(w, http.StatusBadRequest, err)
			return
		}

		product, ok := store.Get(id)
		if !ok {
			responseJSON(w, http.StatusNotFound, fmt.Errorf("product not found"))
			return
		}
		if product.Icon == "" {
			responseJSON(w, http.StatusNotFound, fmt.Errorf("product icon not found"))
			return
		}

		icon, err := os.Open(product.Icon)
		if err != nil {
			responseJSON(w, http.StatusInternalServerError, err)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, icon); err != nil {
			log.Println(err.Error())
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
