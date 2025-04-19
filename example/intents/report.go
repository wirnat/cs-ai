package intents

type Report struct {
}

func (r Report) Code() string {
	return "report"
}

func (r Report) Handle(m map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"date":  "2023-10-01",
		"net":   2000,
		"gross": 2500,
		"tax":   500,
	}, nil
}

func (r Report) Description() []string {
	return []string{
		"laporan penjualan",
		"ketika user ingin tau penjualan",
	}
}

func (r Report) Param() interface{} {
	return ReportParam{}
}

type ReportParam struct {
	Date string `json:"date" validate:"required" description:"tanggal laporan yg ingin diambil"`
}
