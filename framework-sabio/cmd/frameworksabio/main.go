// frameworksabio: experto en data indexada por framework-indexa.
// Responde preguntas del usuario haciendo retrieval sobre el vector store
// y formulando la respuesta con Gemini.
//
// Comandos (contrato del orquestador flujo_api):
//
//	./frameworksabio next-question
//	    Devuelve {"id":"...","text":"..."} con la próxima cosa que mostrar
//	    al usuario, o {} si no hay nada pendiente.
//
//	./frameworksabio ingest-answer --question-id <id> --answer <text>
//	    Toma `answer` como pregunta del usuario, hace retrieval+LLM, y
//	    guarda la respuesta para que la próxima next-question la entregue.
//
//	./frameworksabio query --question <text>
//	    Modo CLI directo (sin estado, para testing). Imprime la respuesta.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/profile"
	"framework-sabio/internal/llm"
	"framework-sabio/internal/sqlqa"
)

// HistoryTurn es un turno de la conversación pasado por el orquestador.
// Llega serializado como JSON dentro de --history (base64-url-safe para
// pasar el axioma de seguridad del Channel que rechaza newlines y
// metacaracteres de shell).
type HistoryTurn struct {
	Role    string `json:"role"`    // "user" | "framework"
	Content string `json:"content"` // texto del turno
}

const (
	defaultStorePath = "../framework-indexa/data/store.json"
	defaultDBPath    = "../framework-indexa/data/panalbit.db"
	defaultStatePath = "temp/state.json"
	greetingText     = "Soy el experto en tus datos indexados. Decime qué querés saber (ej: \"cuántos clientes tengo\", \"detalle de proyectos activos\", \"pagos pendientes\")."
	defaultTopK      = 6
)

// profileLoader carga el perfil activo (Forma Genérica o Forma <nombre>)
// Usa env REMORA_PROFILE_PATH para sobreescribir la ubicación (útil en Docker)
var profileLoader = profile.NewLoader(envOr("REMORA_PROFILE_PATH", "../profiles"))
var activeProfile *profile.Profile

// getProfile carga y cachea el perfil activo
func getProfile() *profile.Profile {
	if activeProfile == nil {
		var err error
		activeProfile, err = profileLoader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sabio] profile load error: %v\n", err)
			activeProfile = &profile.Profile{Name: "generic", Overlays: map[string]string{}}
		}
	}
	return activeProfile
}

// systemPromptWithOverlay aplica el overlay del perfil al system prompt base
func systemPromptWithOverlay(baseSystem, framework string) string {
	return getProfile().SystemPromptWithOverlay(baseSystem, framework)
}

// state mantiene la conversación pendiente de Sabio entre invocaciones CLI.
// Como el orquestador llama al binario una vez por paso, el estado vive en
// disco. Para MVP usamos un único state global (un solo "expert chat").
type state struct {
	GreetingAsked bool      `json:"greeting_asked"`
	PendingAnswer string    `json:"pending_answer"` // respuesta lista para entregar al user
	PendingID     string    `json:"pending_id"`     // id sintético del próximo "next-question"
	LastQuestion  string    `json:"last_question"`  // última pregunta del user (debug)
	LastAnswerAt  time.Time `json:"last_answer_at"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "next-question":
		cmdNextQuestion(os.Args[2:])
	case "ingest-answer":
		cmdIngestAnswer(os.Args[2:])
	case "query":
		cmdQuery(os.Args[2:])
	case "reset":
		cmdReset(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`frameworksabio: Q&A sobre datos indexados por framework-indexa

Uso:
  frameworksabio next-question
  frameworksabio ingest-answer --question-id <id> --answer <text>
  frameworksabio query --question <text>
  frameworksabio reset

Variables de entorno:
  GEMINI_API_KEY      requerida
  GEMINI_CHAT_MODEL   opcional (default gemini-2.5-flash)
  SABIO_STATE         override del path del state
  SABIO_STORE         override del path del vector store
`)
}

func resolvePath(flagVal, envKey, def string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return def
}

func loadState(path string) *state {
	s := &state{}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	return s
}

func saveState(path string, s *state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// cmdNextQuestion: devuelve la próxima cosa que mostrar al user.
//   - Si hay PendingAnswer (acabamos de generar respuesta) → la mostramos
//     como si fuera "pregunta" del bot y la limpiamos.
//   - Si no, y aún no saludamos → saludo inicial.
//   - Si no, {} (nada pendiente).
func cmdNextQuestion(args []string) {
	fs := flag.NewFlagSet("next-question", flag.ExitOnError)
	statePath := fs.String("state", "", "path del state")
	fs.Parse(args)
	sp := resolvePath(*statePath, "SABIO_STATE", defaultStatePath)

	s := loadState(sp)

	if s.PendingAnswer != "" {
		out := map[string]string{
			"id":   s.PendingID,
			"text": s.PendingAnswer,
		}
		// Limpiamos: ya entregamos esta respuesta.
		s.PendingAnswer = ""
		s.PendingID = ""
		_ = saveState(sp, s)
		emitJSON(out)
		return
	}

	if !s.GreetingAsked {
		id := fmt.Sprintf("sabio_greeting_%d", time.Now().Unix())
		s.GreetingAsked = true
		_ = saveState(sp, s)
		emitJSON(map[string]string{"id": id, "text": greetingText})
		return
	}

	// Nada pendiente.
	fmt.Println("{}")
}

// cmdIngestAnswer: toma el answer como query y deja la respuesta en el state
// para que la próxima next-question la entregue.
func cmdIngestAnswer(args []string) {
	fs := flag.NewFlagSet("ingest-answer", flag.ExitOnError)
	questionID := fs.String("question-id", "", "id de la pregunta")
	answer := fs.String("answer", "", "respuesta del usuario (=su pregunta para Sabio)")
	statePath := fs.String("state", "", "path del state")
	storePath := fs.String("store", "", "path del store")
	historyB64 := fs.String("history", "", "historial conversacional (base64-url-safe de JSON [{role,content},...])")
	fs.Parse(args)

	if *answer == "" {
		fail("ingest-answer: --answer requerido")
	}
	sp := resolvePath(*statePath, "SABIO_STATE", defaultStatePath)
	stp := resolvePath(*storePath, "SABIO_STORE", defaultStorePath)

	history := decodeHistory(*historyB64)
	respText, err := answerQuestion(*answer, stp, history)
	if err != nil {
		// No queremos romper el chain por un error de Sabio. Dejamos un
		// mensaje legible como "respuesta".
		respText = fmt.Sprintf("Sabio no pudo responder esta vez: %v", err)
	}

	s := loadState(sp)
	s.LastQuestion = *answer
	s.PendingAnswer = respText
	s.PendingID = fmt.Sprintf("sabio_answer_%d", time.Now().Unix())
	s.LastAnswerAt = time.Now()
	if err := saveState(sp, s); err != nil {
		fail("save state: %v", err)
	}
	_ = *questionID
}

// cmdQuery: modo directo CLI (sin state). Útil para tests.
func cmdQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	question := fs.String("question", "", "pregunta")
	storePath := fs.String("store", "", "path del store")
	fs.Parse(args)
	if *question == "" {
		fail("query: --question requerido")
	}
	stp := resolvePath(*storePath, "SABIO_STORE", defaultStorePath)
	resp, err := answerQuestion(*question, stp, nil)
	if err != nil {
		fail("query: %v", err)
	}
	fmt.Println(resp)
}

func cmdReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	statePath := fs.String("state", "", "path del state")
	fs.Parse(args)
	sp := resolvePath(*statePath, "SABIO_STATE", defaultStatePath)
	_ = os.Remove(sp)
	fmt.Println("state limpiado")
}

// answerQuestion intenta responder la pregunta usando SQL (Text-to-SQL
// sobre la DB SQLite generada por framework-indexa). Si SQL falla o no
// tiene resultados útiles, cae a BM25 como fallback.
//
// El path SQL maneja agregaciones, joins y filtros precisos; BM25 maneja
// recall semántico cuando no hay query SQL clara (descripciones, etc).
func answerQuestion(question, storePath string, history []HistoryTurn) (string, error) {
	llmCli, err := llm.NewClient()
	if err != nil {
		return "", fmt.Errorf("llm client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	dbPath := envOr("SABIO_DB", defaultDBPath)
	if _, statErr := os.Stat(dbPath); statErr == nil {
		// SQL path
		ans, sqlErr := answerWithSQL(ctx, llmCli, dbPath, question, history)
		if sqlErr == nil && ans != "" {
			return ans, nil
		}
		// Caemos a BM25 si SQL falló. Lo logueamos para debug en stderr.
		if sqlErr != nil {
			fmt.Fprintf(os.Stderr, "sql path failed, falling back to BM25: %v\n", sqlErr)
		}
	}

	st, err := newLocalFileStore(storePath)
	if err != nil {
		return "", fmt.Errorf("store open (%s): %w", storePath, err)
	}
	defer st.Close()

	stats, _ := st.Stats()
	totalDocs := 0
	for _, n := range stats {
		totalDocs += n
	}
	if totalDocs == 0 {
		return "El store está vacío. Corré framework-indexa para indexar datos antes de preguntar.", nil
	}

	// Query expansion: pedimos al LLM keywords útiles para BM25 incluyendo
	// términos en inglés (los endpoints y campos del API origen están en
	// inglés: clients, projects, billing_documents, etc.). Esto compensa
	// la limitación literal de BM25 cuando el usuario pregunta en español.
	expanded, err := expandQuery(ctx, llmCli, question)
	if err != nil || strings.TrimSpace(expanded) == "" {
		// Fallback silencioso: si la expansión falla, buscamos con la query original.
		expanded = question
	}
	searchQuery := question + " " + expanded

	hits, err := st.Search(searchQuery, defaultTopK, nil)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	if len(hits) == 0 {
		return "No encontré nada relevante en los datos indexados para esa pregunta.", nil
	}

	var ctxBuilder strings.Builder
	ctxBuilder.WriteString("Datos relevantes recuperados del store (más relevantes primero):\n\n")
	for i, h := range hits {
		fmt.Fprintf(&ctxBuilder, "[%d] endpoint=%s record_id=%s score=%.3f\n", i+1, h.Document.Endpoint, h.Document.RecordID, h.Score)
		ctxBuilder.WriteString(h.Document.Text)
		ctxBuilder.WriteString("\n---\n")
	}

	baseSystem := `Sos Sabio, un asistente que responde sobre los datos de negocio del usuario en español rioplatense natural.
El usuario NO es técnico (gerente, abogado, contador). No conoce JSON, IDs, ni nombres de tablas.

PROHIBIDO ABSOLUTAMENTE (jamás aparece en tu respuesta):
- Nombres de campos crudos en inglés: "residue", "client_id", "project_id", "active", "amount", "currency_id", "status", "code", "id", "type", "created_at", "updated_at", "billing_documents", "time_entries", "advances", "payments", "expenses", "agreements", "rates", "endpoint", "record_id", etc.
- Citas técnicas tipo [payments:2], [clients:1], [advances_detail:3522]. Cero corchetes con IDs.
- IDs numéricos internos sueltos (243, 422, 3522). Solo se permiten códigos formateados de negocio (ej "000239-0001", "000001").
- Frases del estilo "tiene X: 0", "su Y es Z" donde X o Y es un nombre de campo en inglés.

OBLIGATORIO:
- Hablá como una persona, no como un reporte de base de datos.
- Traducí siempre los conceptos: facturas, pagos, anticipos, gastos, horas, tarifas, contratos, clientes, proyectos, áreas.
- Estados: "residue: 0" decílo "ya saldado"; "active: 0" decílo "dado de baja"; "status: ABIERTA" decílo "abierta" o "pendiente".
- Montos: siempre con símbolo de moneda si está disponible (USD, ARS, COP). Si no hay moneda, decí solo "100" sin moneda inventada.
- Fechas en DD/MM/AAAA.
- Si falta un dato pedido, decí "no encontré ese dato" UNA vez al final, no por cada campo faltante.

ESTILO:
- Frases cortas, directas. Sin preámbulos ("Claro", "Por supuesto").
- Listas con viñetas solo si hay 3+ items comparables.
- Sin asteriscos a modo de etiquetas tipo "*Código*: ...". Escribilo natural: "El código es 000239".
- Si la pregunta es ambigua, hacé UNA pregunta corta para clarificar.

EJEMPLOS DE CONVERSIÓN:
- "Todos los pagos tienen residue: 0" → "Todos los pagos ya están saldados."
- "El proyecto tiene project_id: 422 y client_id: 243" → "Es el proyecto 000239-0001 del cliente Gislason Ltd."
- "billing_documents_detail muestra status: ABIERTA" → "La factura está abierta."
- "[advances:3522]" → (omitir, no se cita).`

	system := systemPromptWithOverlay(baseSystem, "sabio")

	userPrompt := fmt.Sprintf("Pregunta del usuario: %s\n\nDatos del negocio relevantes (uso interno, NO copiar literal):\n%s\n\nRespondé al usuario en lenguaje natural de negocio. Aplicá las reglas estrictas: nada de nombres de campos, nada de IDs internos, nada de citas con corchetes.", question, ctxBuilder.String())

	resp, err := llmCli.Generate(ctx, system, userPrompt)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}
	return strings.TrimSpace(resp), nil
}

// expandQuery pide al LLM keywords útiles para BM25, incluyendo traducciones
// al inglés (los datos están en inglés). Devuelve una cadena de keywords
// separadas por espacio. Si falla, se devuelve string vacío y el caller cae
// al fallback de usar la query original.
func expandQuery(ctx context.Context, c *llm.Client, question string) (string, error) {
	system := `Sos un asistente que ayuda a buscar registros en una base de datos de un sistema de timebilling para estudios jurídicos.
Los datos están en inglés con campos como: clients, projects, billing_documents, payments, expenses, time_entries, charges, advances, currencies, areas, users, rates, etc.

Tu tarea: dada una pregunta del usuario, generá una lista compacta de keywords útiles para búsqueda full-text (BM25), incluyendo:
- Traducciones al inglés de los términos clave de la pregunta
- Nombres de los endpoints/campos relevantes
- Sinónimos relevantes

Reglas:
- Devolvé SOLO las keywords, separadas por espacio.
- No expliques, no agregues comillas, no uses markdown.
- Máximo ~15 keywords.
- Si la pregunta es ambigua, incluí keywords de las interpretaciones más probables.`

	resp, err := c.Generate(ctx, system, question)
	if err != nil {
		return "", err
	}
	// Limpieza defensiva: a veces el modelo agrega un prefacio.
	resp = strings.TrimSpace(resp)
	// Si la respuesta tiene varias líneas, tomamos solo la primera.
	if idx := strings.IndexByte(resp, '\n'); idx >= 0 {
		resp = resp[:idx]
	}
	return resp, nil
}

// answerWithSQL implementa el path Text-to-SQL:
//
//  1. abre la DB read-only
//  2. pide al LLM una SQL SELECT a partir de schema + pregunta
//  3. ejecuta y trae filas
//  4. pide al LLM phrasing natural en español rioplatense, sin tecnicismos
//
// Si la DB no responde, la SQL es inválida o sin filas relevantes, retorna
// "" como segundo arg para que el caller decida fallback.
func answerWithSQL(ctx context.Context, c *llm.Client, dbPath, question string, history []HistoryTurn) (string, error) {
	eng, err := sqlqa.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("open db: %w", err)
	}
	defer eng.Close()

	// Reintento corto: si el LLM se equivoca con SQL inválido, le devolvemos
	// el error para que corrija. 2 intentos máx.
	var (
		lastSQL  string
		lastErr  error
		queryRes *sqlqa.QueryResult
	)
	for attempt := 0; attempt < 3; attempt++ {
		sqlText, err := generateSQL(ctx, c, eng.Schema(), question, history, lastSQL, lastErr)
		if err != nil {
			return "", fmt.Errorf("llm sql gen: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[sabio sql attempt %d] %s\n", attempt+1, sqlText)
		lastSQL = sqlText
		res, runErr := eng.Run(ctx, sqlText)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "[sabio sql attempt %d FAILED] %v\n", attempt+1, runErr)
			lastErr = runErr
			continue
		}
		queryRes = res
		break
	}
	if queryRes == nil {
		return "", fmt.Errorf("sql falló tras reintentos: %v", lastErr)
	}

	// Phrasing natural con las filas.
	answer, err := phraseSQLAnswer(ctx, c, question, queryRes, history)
	if err != nil {
		return "", fmt.Errorf("phrasing: %w", err)
	}
	return strings.TrimSpace(answer), nil
}

// generateSQL le pide al LLM la SQL SELECT.
// Si previousSQL/previousErr no son vacíos, los incluye como feedback.
// history aporta contexto conversacional para resolver referencias como
// "los 2 primeros", "y los inactivos?", etc.
func generateSQL(ctx context.Context, c *llm.Client, schema, question string, history []HistoryTurn, previousSQL string, previousErr error) (string, error) {
	baseSystem := `Sos un experto en SQLite. Te dan el schema de una base con datos de un sistema de timebilling para estudios jurídicos, y una pregunta del usuario en español.

Tu tarea: generar UNA sola sentencia SELECT (o WITH ... SELECT) en sintaxis SQLite que responda la pregunta usando los datos reales de la base.

REGLAS ESTRICTAS:
- Devolvé SOLO la SQL, sin markdown, sin backticks, sin explicaciones.
- UNA sola sentencia. Sin punto y coma final. Sin múltiples queries.
- Solo SELECT. Nunca INSERT/UPDATE/DELETE/DROP/ALTER/PRAGMA/ATTACH.
- Quoteá tablas y columnas con dobles comillas: "clients", "billing_documents".
- Todos los valores son TEXT en SQLite. Para comparar números: CAST("col" AS REAL) o CAST("col" AS INTEGER).
- Para fechas: las columnas son strings tipo "2024-01-15" o "2024-01-15 14:30:00". Usá comparación de strings o date()/datetime().
- Para preguntas de cantidad usá COUNT(*). Para sumas SUM(CAST(...)). Para top-N usá ORDER BY ... LIMIT.
- Si la pregunta requiere joins, usá JOIN explícito con ON. Convenciones del schema:
    * "clients"."id" = "projects"."client_id"
    * "projects"."id" = "billing_documents"."project_id"  (verificalo con el schema)
    * "billing_documents"."id" = "payments"."billing_document_id" (idem)
    * "users"."id" = "time_entries"."user_id"
- Para "activos" usá ' "active" = "1" '. Para "dados de baja"/"inactivos" usá ' "active" = "0" '.
- Si la pregunta es ambigua respecto a qué entidad, elegí la interpretación más útil y completa.
- LIMIT defensivo: si la query devuelve filas (no agregados), poné LIMIT 100.
- Si la pregunta tiene VARIAS sub-preguntas independientes ("cuántos X y cuántos Y"), NO uses JOIN. Usá subqueries escalares en una sola sentencia:
    SELECT
      (SELECT COUNT(*) FROM "clients" WHERE "active" = '1') AS clientes_activos,
      (SELECT COUNT(*) FROM "payments") AS total_pagos
- En JOINs, SIEMPRE prefijá las columnas con el alias o nombre de tabla (ej "c"."id", "p"."id") para evitar errores de "ambiguous column name".
- Antes de hacer un JOIN entre dos tablas, mirá el schema: solo usá la columna FK que existe (ej si projects tiene "client_id", entonces JOIN ON "projects"."client_id" = "clients"."id").
- BÚSQUEDA POR NOMBRE: cuando el usuario menciona un nombre (cliente, proyecto, persona, etc.), NUNCA uses '=' exacto. Usá SIEMPRE LIKE con wildcards y comparación case-insensitive: lower("name") LIKE lower('%texto%'). Los usuarios suelen escribir solo parte del nombre o con acentos distintos.
- Si la pregunta menciona un valor (ej "cliente Tillman"), no asumas que el match es exacto — el nombre real puede ser "Tillman-Crona" o similar.
- HISTORIAL: si el usuario hace una pregunta de seguimiento ("y los inactivos?", "cuáles son?", "los 2 primeros", "y en 2023?"), MIRÁ el historial conversacional para entender a qué se refiere y reformulá la query SOBRE ESA MISMA ENTIDAD/FILTRO. Ejemplo: si antes preguntaste "cuántos clientes activos" y ahora dice "cuáles son los 2 primeros", devolvé SELECT "name" FROM "clients" WHERE "active"='1' ORDER BY "name" LIMIT 2. NUNCA repitas el COUNT anterior.
- ALIAS SEMÁNTICOS OBLIGATORIOS: nombrá las columnas devueltas con sufijos que indiquen su unidad para que el lenguaje natural no se confunda:
    * Conteos: usá sufijo _count o _total_n. Ej: COUNT(*) AS clientes_count, COUNT(*) AS pagos_total_n.
    * Sumas de dinero: sufijo _amount con la moneda si se conoce. Ej: SUM(CAST("amount" AS REAL)) AS pagos_amount.
    * Días de mora: sufijo _dias. Ej: CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias.
    * Sumas de horas: sufijo _hours. Ej: SUM(CAST("duration" AS REAL)) AS trabajo_hours.
    * Listados de nombres: alias claro como nombre, cliente, proyecto.
  Esto es CRÍTICO: el phraser lee estos nombres y decide si poner $ o no.

CÁLCULO DE DÍAS DE MORA — REGLA CRÍTICA:
- Para calcular días de mora SIEMPRE usá aritmética de fechas de SQLite:
    CAST((julianday('now') - julianday("date")) AS INTEGER)
- "date" debe ser una columna de tipo fecha ('YYYY-MM-DD'), NUNCA "amount" ni ningún campo numérico monetario.
- NUNCA uses strftime('%s',...) para calcular diferencia de días (produce segundos, no días).
- NUNCA uses CAST("date" AS INTEGER) (produce solo el año, ej: 2015).
- Ejemplo correcto: CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias

CÁLCULO DE SALDO PENDIENTE DE COBRANZA — REGLA CRÍTICA:
- El saldo pendiente real de un cliente se calcula sumando milestones de sus charges en estado impago:
    SELECT c."name",
           COALESCE(SUM(CAST(m."amount" AS REAL)), 0) AS saldo_amount,
           CAST((julianday('now') - julianday(MIN(m."date"))) AS INTEGER) AS mora_dias,
           COUNT(DISTINCT ch."id") AS facturas_count
    FROM "clients" c
    JOIN "charges" ch ON ch."client_id" = c."id"
    LEFT JOIN "milestones" m ON m."charge_id" = ch."id"
    WHERE ch."state" IN ('FACTURADO','EMITIDO','PAGO PARCIAL','ENVIADO AL CLIENTE','EN REVISION')
      AND m."amount" IS NOT NULL AND m."amount" != ''
      AND lower(c."name") LIKE lower('%nombre%')
    GROUP BY c."id"
- NUNCA confundas saldo_amount con mora_dias. Son columnas distintas con significado distinto.
- mora_dias es un número de días (ej: 400), saldo_amount es un monto monetario (ej: 22611.60).

EJEMPLOS:
- "cuántos clientes" → SELECT COUNT(*) AS total FROM "clients"
- "cuántos clientes activos y cuántos proyectos" → SELECT (SELECT COUNT(*) FROM "clients" WHERE "active"='1') AS activos, (SELECT COUNT(*) FROM "projects") AS proyectos
- "top 5 clientes por proyectos" → SELECT c."name", COUNT(p."id") AS n FROM "clients" c JOIN "projects" p ON p."client_id" = c."id" GROUP BY c."id" ORDER BY n DESC LIMIT 5
- "cuántas facturas tiene el cliente Tillman" → SELECT COUNT(*) FROM "billing_documents" bd JOIN "clients" c ON bd."client_id" = c."id" WHERE lower(c."name") LIKE lower('%Tillman%')
- "pagos del año 2024" → SELECT COUNT(*) FROM "payments" WHERE "date" LIKE '2024-%'
- "monto total de pagos en 2022" → SELECT SUM(CAST("amount" AS REAL)) FROM "payments" WHERE "date" LIKE '2022-%'
- "análisis 360° de Thiel-Effertz" → usar el patrón de CÁLCULO DE SALDO PENDIENTE con nombre '%Thiel%'
- "generar email de cobranza" o "email para [cliente]" → (1) extraé el nombre del cliente del historial conversacional, (2) aplicá el patrón de CÁLCULO DE SALDO PENDIENTE exacto para ese cliente (charges+milestones, NO billing_documents.amount que es distinto), (3) devolvé: nombre, saldo_amount, mora_dias, facturas_count.
- NUNCA uses "billing_documents"."amount" como saldo de cobranza — ese campo es el monto facturado, no el saldo pendiente de milestones. El saldo real siempre viene de SUM(milestones.amount) filtrado por charges.state impago.

Schema:
` + schema

	system := systemPromptWithOverlay(baseSystem, "sabio")

	historyBlock := formatHistoryForPrompt(history)
	userPrompt := question
	if historyBlock != "" {
		userPrompt = fmt.Sprintf("Historial conversacional reciente (último primero abajo):\n%s\n\nNueva pregunta del usuario: %s", historyBlock, question)
	}
	if previousSQL != "" && previousErr != nil {
		userPrompt = fmt.Sprintf(`%s

Tu intento anterior:
%s

Falló con este error:
%v

Corregí la SQL y devolvé solo la nueva sentencia.`, userPrompt, previousSQL, previousErr)
	}

	resp, err := c.Generate(ctx, system, userPrompt)
	if err != nil {
		return "", err
	}
	return cleanSQLResponse(resp), nil
}

// cleanSQLResponse remueve markdown fences, prefijos y sufijos comunes
// que algunos modelos agregan a pesar de las reglas.
func cleanSQLResponse(s string) string {
	s = strings.TrimSpace(s)
	// Quitar fences ```sql ... ``` o ```...```
	if strings.HasPrefix(s, "```") {
		// Quitar primera línea (con o sin lenguaje)
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	// Algunos modelos prefijan con "SQL:" o similar.
	for _, p := range []string{"sql:", "SQL:", "Query:", "query:"} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

// phraseSQLAnswer pide al LLM que arme la respuesta natural a partir
// de las filas devueltas por la SQL. history aporta el contexto previo
// para que el phraser pueda continuar el hilo (ej: el usuario dijo "los 2
// primeros" → la respuesta debe nombrarlos, no decir "hay 2").
func phraseSQLAnswer(ctx context.Context, c *llm.Client, question string, qr *sqlqa.QueryResult, history []HistoryTurn) (string, error) {
	baseSystem := `Sos Sabio, un asistente que responde en español rioplatense natural sobre los datos de negocio del usuario (estudio jurídico, sistema de timebilling).

El usuario NO es técnico (gerente, abogado, contador). NO conoce JSON, IDs, ni nombres de tablas.

PROHIBIDO:
- Nombres de tablas o columnas en inglés ("billing_documents", "client_id", "active", "id", "code").
- Citas técnicas o IDs internos sueltos. Solo se permiten códigos formateados de negocio (ej "000239-0001").
- Mostrar la SQL al usuario. Solo el resultado en lenguaje humano.
- INVENTAR UNIDADES O MONEDAS. Si los datos no traen una moneda explícita, NO pongas "$", "USD", "€", "ARS", etc.

INTERPRETACIÓN DE COLUMNAS (CRÍTICO PARA NO ALUCINAR UNIDADES):
- Columna terminada en _count, _total_n, _qty, _n, o que es un COUNT(*) → CANTIDAD ENTERA. NUNCA agregues símbolo de moneda. Ej: "pagos_count": 1451 → "hay 1451 pagos" (NO "$1451").
- Columna terminada en _amount, _monto, _total_amount, _sum → MONTO DE DINERO. Solo agregá moneda si en los datos hay una columna como currency, currency_code o si la pregunta lo deja claro. Si no, decí el número sin símbolo.
- Columna terminada en _dias, _days → DÍAS ENTEROS. Decí "X días". NUNCA lo interpretes como monto de dinero.
- Columna terminada en _hours, _horas → HORAS. Decí "X horas".
- Columna terminada en _date, _at o que se ve como fecha → FECHA en DD/MM/AAAA.
- Si la columna se llama solo "name", "id", "code", o un alias semántico claro ("cliente", "proyecto") → es un identificador o nombre, decílo como tal.
- Si la SQL tiene COUNT(*) sin alias, igual es CANTIDAD, nunca dinero.

OBLIGATORIO:
- Traducí los conceptos: facturas, pagos, anticipos, gastos, horas, tarifas, contratos, clientes, proyectos, áreas.
- Estados: "active=0" → "dado de baja". "active=1" → "activo". "residue=0" → "saldado".
- Fechas en DD/MM/AAAA.
- Frases cortas, directas. Sin preámbulos. Si hay 3+ items comparables usá viñetas.
- Si la respuesta es un único número (COUNT, SUM), decílo en una frase clara con la unidad correcta según el alias.
- Si las filas están vacías, decí honestamente "no encontré ningún registro que coincida".
- USÁ EL HISTORIAL: si el usuario hizo una pregunta de seguimiento ("cuáles son?", "y los 2 primeros?"), respondé concretamente con los datos pedidos, NO repitas el conteo o respuesta del turno anterior.

Vas a recibir:
1. El historial de la conversación reciente (puede estar vacío).
2. La pregunta actual del usuario.
3. La SQL ejecutada (solo para que entiendas qué representa cada columna; NO la muestres al usuario).
4. Las filas devueltas.

Tu salida: solo la respuesta natural al usuario. Nada más.`

	system := systemPromptWithOverlay(baseSystem, "sabio")

	historyBlock := formatHistoryForPrompt(history)
	if historyBlock == "" {
		historyBlock = "(sin historial previo)"
	}

	userPrompt := fmt.Sprintf(`Historial conversacional reciente:
%s

Pregunta actual del usuario:
%s

SQL ejecutada (uso interno, NO mostrar al usuario, sirve para entender alias/unidades):
%s

Resultado (%d filas%s):
%s

Respondé en lenguaje natural al usuario aplicando las reglas de unidades.`,
		historyBlock,
		question,
		qr.SQL,
		qr.RowCount,
		func() string {
			if qr.Truncated {
				return ", se muestran los primeros"
			}
			return ""
		}(),
		sqlqa.FormatRowsForPrompt(qr),
	)

	return c.Generate(ctx, system, userPrompt)
}

// formatHistoryForPrompt arma el bloque de historial conversacional
// para inyectar en los prompts. Tomamos hasta 6 turnos recientes y los
// presentamos en orden cronológico.
func formatHistoryForPrompt(history []HistoryTurn) string {
	if len(history) == 0 {
		return ""
	}
	const maxTurns = 6
	start := 0
	if len(history) > maxTurns {
		start = len(history) - maxTurns
	}
	var sb strings.Builder
	for _, t := range history[start:] {
		role := t.Role
		switch role {
		case "user":
			role = "Usuario"
		case "framework", "sabio", "assistant":
			role = "Sabio"
		}
		content := strings.TrimSpace(t.Content)
		if len(content) > 800 {
			content = content[:800] + "…"
		}
		fmt.Fprintf(&sb, "%s: %s\n", role, content)
	}
	return strings.TrimSpace(sb.String())
}

// decodeHistory toma el flag --history (base64-url-safe de un JSON
// [{role,content}, ...]) y lo decodifica de forma defensiva: si está
// vacío o falla el parse, devuelve nil sin romper el flujo.
func decodeHistory(s string) []HistoryTurn {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		// fallback: probar base64 estándar por si el orquestador no usó url-safe
		if raw2, err2 := base64.StdEncoding.DecodeString(s); err2 == nil {
			raw = raw2
		} else {
			fmt.Fprintf(os.Stderr, "[sabio] decodeHistory: base64 decode falló: %v\n", err)
			return nil
		}
	}
	var out []HistoryTurn
	if err := json.Unmarshal(raw, &out); err != nil {
		fmt.Fprintf(os.Stderr, "[sabio] decodeHistory: json unmarshal falló: %v\n", err)
		return nil
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func emitJSON(v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Println(string(data))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
