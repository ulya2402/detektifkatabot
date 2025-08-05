package game

type WordData struct {
	Word  string
	Hints []string
}

var SoloWordList = []WordData{
	{
		Word:  "KEMERDEKAAN",
		Hints: []string{"SEJARAH", "MERAH-PUTIH", "AGUSTUS"},
	},
	{
		Word:  "RESTORAN",
		Hints: []string{"MAKANAN", "MENU", "PELAYAN"},
	},
	{
		Word:  "BIOSKOP",
		Hints: []string{"FILM", "POPCORN", "LAYAR BESAR"},
	},
	{
		Word:  "PULAU",
		Hints: []string{"LAUT", "PANTAI", "TERPENCIL"},
	},
	{
		Word:  "ASTRONOT",
		Hints: []string{"LUAR ANGKASA", "ROCKET", "BULAN"},
	},
}