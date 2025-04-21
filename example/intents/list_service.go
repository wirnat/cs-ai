package intents

import "context"

type ListService struct{}

func (l ListService) Code() string {
	return "list-service"
}

func (l ListService) Handle(ctx context.Context, m map[string]interface{}) (interface{}, error) {
	return []map[string]interface{}{
		{
			"service":     "Haircut Adult",
			"description": "Potong rambut untuk orang dewasa",
			"price":       50000,
		},
		{
			"service":     "Haircut Kids",
			"description": "Potong rambut untuk anak dibawah 10 tahun",
			"price":       40000,
		},
		{
			"service":     "Hair Wash",
			"description": "Keramas rambut",
			"price":       10000,
		},
	}, nil
}

func (l ListService) Description() []string {
	return []string{
		"list servis yg tersedia",
		"layanan yg tersedia",
	}
}

func (l ListService) Param() interface{} {
	return nil
}
