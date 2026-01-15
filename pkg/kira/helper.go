package kira

func createEmptyKiraHelperForm() KiraHelperForm {
	return KiraHelperForm{
		User: Character{
			Interessen:               []string{},
			TraeumeUndWuensche:       []string{},
			GespeicherteErinnerungen: []string{},
			AktuelleThemen:           []string{},
			TabuThemen:               []string{},
			PersonenImLeben:          []PersonImLeben{},
		},
		Kira: Character{
			Interessen:               []string{},
			TraeumeUndWuensche:       []string{},
			GespeicherteErinnerungen: []string{},
			AktuelleThemen:           []string{},
			TabuThemen:               []string{},
			PersonenImLeben:          []PersonImLeben{},
		},
	}
}
