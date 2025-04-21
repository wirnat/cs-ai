package intents

import "context"

type BrandInfo struct {
}

func (b BrandInfo) Code() string {
	return "brand-info"
}

type Outlet struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (b BrandInfo) Handle(ctx context.Context, i map[string]interface{}) (interface{}, error) {
	return Outlet{
		Address: "Jl. Kenyeri 2, Gang D, No.4",
		Name:    "Ms Man",
	}, nil
}

func (b BrandInfo) Description() []string {
	return []string{
		"menghandle permintaan atau konteks kalimat menanyakan alamat toko",
	}
}

func (b BrandInfo) Param() interface{} {
	return nil
}
