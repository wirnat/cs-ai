package intents

type ProductCatalog struct{}

func (p ProductCatalog) Param() interface{} {
	return nil
}

func (p ProductCatalog) Handle(i map[string]interface{}) (interface{}, error) {
	return []map[string]interface{}{
		{
			"service":     "Hairnerd Pomade",
			"description": "Pomade untuk rambut kaku dan berkilau",
			"price":       70000,
		},
		{
			"service":     "Hairnerd Shampoo",
			"description": "Shampoo untuk menghilangkan ketombe dan melemaskan rambut",
			"price":       40000,
		},
		{
			"service":     "Selsun Shampoo",
			"description": "Shampoo untuk menghilangkan ketombe",
			"price":       10000,
		},
	}, nil
}

func (p ProductCatalog) Description() []string {
	return []string{
		"Ingin tau produk apa",
		"Cari produk",
		"Lihat daftar produk",
		"Beli produk",
		"Apa saja produk yg dijual",
		"Catalog produknya apa saja",
		"List Produk/Item",
	}
}
func (p ProductCatalog) Code() string {
	return "product-catalog"
}
