package main

// Debtor es el perfil del deudor que Carolina contacta.
// En producción esto vendría de la BD del cliente (FinCrowd).
type Debtor struct {
	ID             string
	Nombre         string
	DeudaCLP       int
	DiasAtraso     int
	HistorialPagos string // "puntual_hasta_2024", "atrasos_recurrentes", "primera_mora", etc.
	TonoPreferido  string // "formal", "cercano" — derivado de interacciones previas o inferido
}

// Caso de prueba seed para correr el MVP.
var DebtorSeed = Debtor{
	ID:             "SR-2024-0142",
	Nombre:         "Patricia Morales",
	DeudaCLP:       847000,
	DiasAtraso:     38,
	HistorialPagos: "puntual_hasta_2024",
	TonoPreferido:  "cercano",
}

// DebtorScenarios son los perfiles de prueba para validar Carolina contra
// distintos comportamientos de deudor antes de integrar WhatsApp real.
// Usar con la variable de entorno DEBTOR_PROFILE=patricia|roberto|marta
var DebtorScenarios = map[string]Debtor{
	"patricia": DebtorSeed,
	"roberto": {
		ID:             "SR-2024-0287",
		Nombre:         "Roberto Espinoza",
		DeudaCLP:       2540000,
		DiasAtraso:     91,
		HistorialPagos: "atrasos_recurrentes",
		TonoPreferido:  "formal",
	},
	"marta": {
		ID:             "SR-2024-0413",
		Nombre:         "Marta Jiménez",
		DeudaCLP:       1180000,
		DiasAtraso:     54,
		HistorialPagos: "primera_mora",
		TonoPreferido:  "cercano",
	},
}

// PlanPago es una oferta concreta que Carolina puede proponer.
type PlanPago struct {
	Cuotas        int
	MontoCuotaCLP int
	DescuentoPct  int  // % de descuento sobre total si paga pronto
	Recargo       bool // true si tiene interés por mora
}

// CatalogoPlanes son los planes válidos que Carolina puede ofrecer.
// En producción esto vendría de las reglas de negocio de FinCrowd.
func CatalogoPlanes(deuda int) []PlanPago {
	return []PlanPago{
		{Cuotas: 1, MontoCuotaCLP: int(float64(deuda) * 0.92), DescuentoPct: 8, Recargo: false},
		{Cuotas: 3, MontoCuotaCLP: deuda / 3, DescuentoPct: 0, Recargo: false},
		{Cuotas: 6, MontoCuotaCLP: int(float64(deuda) * 1.06 / 6), DescuentoPct: 0, Recargo: true},
	}
}
