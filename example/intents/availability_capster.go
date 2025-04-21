package intents

import (
	"context"
	"fmt"
)

type AvailabilityCapster struct {
}

func (a AvailabilityCapster) Code() string {
	return "capster-availability"
}

func (a AvailabilityCapster) Handle(ctx context.Context, m map[string]interface{}) (interface{}, error) {
	date := m["date"].(string)
	return []map[string]interface{}{
		{
			"capster": "Rudi",
			"date":    []string{fmt.Sprintf("%v 13:00", date), fmt.Sprintf("%v 14:00", date), fmt.Sprintf("%v 015:00", date)},
		},
		{
			"capster": "Rama",
			"date":    []string{fmt.Sprintf("%v 14:00", date), fmt.Sprintf("%v 13:00", date), fmt.Sprintf("%v 12:00", date)},
		},
		{
			"capster": "Ade",
			"date":    []string{fmt.Sprintf("%v 09:00", date), fmt.Sprintf("%v 10:00", date), fmt.Sprintf("%v 11:00", date)},
		},
	}, nil
}

func (a AvailabilityCapster) Description() []string {
	return []string{
		"menjawab tentang pertanyaan ketersediaan capster",
		"menanyakan spesifik ketersediaan capster di dalam tanggal tertentu",
		"contoh: deny hari ini ada kk?",
	}
}

type AvailabilityCapsterParam struct {
	Date *string `json:"date" validate:"required" description:"tanggal jadwal capster yg dicek"`
}

func (a AvailabilityCapster) Param() interface{} {
	return AvailabilityCapsterParam{}
}
