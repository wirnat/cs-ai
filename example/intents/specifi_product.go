package intents

import "context"

type StockProduct struct{}

func (s StockProduct) Description() []string {
	return []string{
		"mengecek stok suatu produk",
	}
}

func (s StockProduct) Param() interface{} {
	return ItemStockParam{}
}

func (s StockProduct) Code() string {
	return "get-stock"
}

func (s StockProduct) Handle(ctx context.Context, i map[string]interface{}) (interface{}, error) {
	return ItemR{
		Name:  i["name"].(string),
		Stock: 10,
	}, nil
}

type ItemR struct {
	Name  string `json:"name"`
	Stock int    `json:"stock"`
}

type ItemStockParam struct {
	Name string `json:"name" validate:"required" description:"nama dari produk yg akan di cek stoknya"`
}
