package intents

type BookingCapster struct{}

func (b BookingCapster) Code() string {
	return "booking-capster"
}

func (b BookingCapster) Handle(m map[string]interface{}) (interface{}, error) {
	capsterName, _ := m["capster_name"].(string)
	date, _ := m["date"].(string)

	return map[string]interface{}{
		"capster_name": capsterName,
		"date":         date,
		"booking_code": "B0001",
	}, nil
}

func (b BookingCapster) Description() []string {
	return []string{
		"customer ingin melakukan booking",
		"selalu pastikan data tool yg required, dan jangan mengisi sendiri date jika customer belum memberikannya",
		"menghandle semua aktivitas booking dari customer",
		"contoh: saya ingin booking hari ini, saya ingin booking, saya ingin booking john",
	}
}

type BookingRequest struct {
	CapsterName string `json:"capster_name" description:"nama capster yg dibook"`
	Date        string `json:"date" validate:"required" description:"tanggal yg dibooking harus diisi, ketika user tidak mencantumkan tanggal atau konteks waktu maka minta itu terlebih dahulu"`
	Service     string `json:"service" description:"nama servis yg akan dibooking / direservasi. contoh: cukur, cukur jenggot, haircut"`
}

func (b BookingCapster) Param() interface{} {
	return BookingRequest{}
}
