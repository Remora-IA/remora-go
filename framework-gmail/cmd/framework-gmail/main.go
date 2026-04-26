package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	// ========== EMAIL COMUNICATION ==========
	case "send-email":
		cmdSendEmail(os.Args[2:])
	case "create-draft":
		cmdCreateDraft(os.Args[2:])
	case "list-drafts":
		cmdListDrafts(os.Args[2:])
	case "get-unread-emails":
		cmdGetUnreadEmails(os.Args[2:])
	case "read-email":
		cmdReadEmail(os.Args[2:])
	case "open-email":
		cmdOpenEmail(os.Args[2:])
	case "mark-email-as-read":
		cmdMarkEmailAsRead(os.Args[2:])
	case "trash-email":
		cmdTrashEmail(os.Args[2:])

	// ========== SEARCH / DISCOVERY ==========
	case "search-emails":
		cmdSearchEmails(os.Args[2:])
	case "search-by-label":
		cmdSearchByLabel(os.Args[2:])

	// ========== LABELS ==========
	case "list-labels":
		cmdListLabels(os.Args[2:])
	case "create-label":
		cmdCreateLabel(os.Args[2:])
	case "apply-label":
		cmdApplyLabel(os.Args[2:])
	case "remove-label":
		cmdRemoveLabel(os.Args[2:])
	case "rename-label":
		cmdRenameLabel(os.Args[2:])
	case "delete-label":
		cmdDeleteLabel(os.Args[2:])

	// ========== FOLDERS ==========
	case "list-folders":
		cmdListFolders(os.Args[2:])
	case "create-folder":
		cmdCreateFolder(os.Args[2:])
	case "move-to-folder":
		cmdMoveToFolder(os.Args[2:])

	// ========== FILTERS ==========
	case "list-filters":
		cmdListFilters(os.Args[2:])
	case "get-filter":
		cmdGetFilter(os.Args[2:])
	case "create-filter":
		cmdCreateFilter(os.Args[2:])
	case "delete-filter":
		cmdDeleteFilter(os.Args[2:])

	// ========== ARCHIVE ==========
	case "archive-email":
		cmdArchiveEmail(os.Args[2:])
	case "batch-archive":
		cmdBatchArchive(os.Args[2:])
	case "list-archived":
		cmdListArchived(os.Args[2:])
	case "restore-to-inbox":
		cmdRestoreToInbox(os.Args[2:])

	// ========== HELP ==========
	case "help", "-h", "--help", "commands":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`Framework framework-gmail - Gmail MCP CLI

COMUNICACIÓN:
  send-email             Enviar un email
  create-draft           Crear borrador sin enviar
  list-drafts            Listar borradores
  get-unread-emails      Obtener emails no leídos
  read-email             Leer contenido de un email
  open-email             Abrir email en navegador
  mark-email-as-read     Marcar como leído
  trash-email            Mover a papelera

BÚSQUEDA:
  search-emails          Buscar con sintaxis Gmail
  search-by-label        Buscar por etiqueta

ETIQUETAS:
  list-labels            Listar etiquetas
  create-label           Crear etiqueta
  apply-label            Aplicar etiqueta a email
  remove-label           Quitar etiqueta
  rename-label           Renombrar etiqueta
  delete-label           Eliminar etiqueta

CARPETAS:
  list-folders           Listar carpetas
  create-folder          Crear carpeta
  move-to-folder         Mover email a carpeta

FILTROS:
  list-filters           Listar filtros
  get-filter             Ver filtro específico
  create-filter          Crear filtro
  delete-filter          Eliminar filtro

ARCHIVO:
  archive-email          Archivar email
  batch-archive          Archivar múltiples emails
  list-archived          Listar emails archivados
  restore-to-inbox       Restaurar a bandeja

USO:
  gmail send-email --recipient_id EMAIL --subject 'ASUNTO' --message 'MENSAJE'
  gmail search-emails --query 'from:remitente@gmail.com'
`)
}

func getParam(params map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := params[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

func parseArgs(args []string) map[string]string {
	result := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				result[key] = args[i+1]
				i++
			} else {
				result[key] = "true"
			}
		}
	}
	return result
}

// ========== EMAIL COMMANDS ==========

func cmdSendEmail(args []string) {
	params := parseArgs(args)
	recipient := getParam(params, "recipient", "recipient_id", "to")
	subject := params["subject"]
	message := params["message"]
	if recipient == "" || subject == "" || message == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail send-email --recipient_id EMAIL --subject 'ASUNTO' --message 'MENSAJE'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: send-email to=%s subject=%s\n", recipient, subject)
}

func cmdCreateDraft(args []string) {
	params := parseArgs(args)
	recipient := getParam(params, "recipient", "recipient_id", "to")
	subject := params["subject"]
	message := params["message"]
	if recipient == "" || subject == "" || message == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail create-draft --recipient_id EMAIL --subject 'ASUNTO' --message 'MENSAJE'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: create-draft to=%s subject=%s\n", recipient, subject)
}

func cmdListDrafts(args []string) {
	fmt.Println("Ejecutando: list-drafts")
}

func cmdGetUnreadEmails(args []string) {
	fmt.Println("Ejecutando: get-unread-emails")
}

func cmdReadEmail(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id", "email")
	if emailID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail read-email --email_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: read-email id=%s\n", emailID)
}

func cmdOpenEmail(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	if emailID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail open-email --email_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: open-email id=%s\n", emailID)
}

func cmdMarkEmailAsRead(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	if emailID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail mark-email-as-read --email_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: mark-email-as-read id=%s\n", emailID)
}

func cmdTrashEmail(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	if emailID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail trash-email --email_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: trash-email id=%s\n", emailID)
}

// ========== SEARCH COMMANDS ==========

func cmdSearchEmails(args []string) {
	params := parseArgs(args)
	query := getParam(params, "query", "q")
	maxResults := getParam(params, "max_results", "max")
	if maxResults == "" {
		maxResults = "50"
	}
	if query == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail search-emails --query 'from:remitente@email.com'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: search-emails query='%s' max_results=%s\n", query, maxResults)
}

func cmdSearchByLabel(args []string) {
	params := parseArgs(args)
	labelID := getParam(params, "label_id", "label")
	if labelID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail search-by-label --label_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: search-by-label label_id=%s\n", labelID)
}

// ========== LABEL COMMANDS ==========

func cmdListLabels(args []string) {
	fmt.Println("Ejecutando: list-labels")
}

func cmdCreateLabel(args []string) {
	params := parseArgs(args)
	name := params["name"]
	if name == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail create-label --name 'NOMBRE'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: create-label name='%s'\n", name)
}

func cmdApplyLabel(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	labelID := getParam(params, "label_id", "label")
	if emailID == "" || labelID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail apply-label --email_id ID --label_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: apply-label email_id=%s label_id=%s\n", emailID, labelID)
}

func cmdRemoveLabel(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	labelID := getParam(params, "label_id", "label")
	if emailID == "" || labelID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail remove-label --email_id ID --label_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: remove-label email_id=%s label_id=%s\n", emailID, labelID)
}

func cmdRenameLabel(args []string) {
	params := parseArgs(args)
	labelID := getParam(params, "label_id", "id")
	newName := getParam(params, "new_name", "name")
	if labelID == "" || newName == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail rename-label --label_id ID --new_name 'NOMBRE'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: rename-label label_id=%s new_name='%s'\n", labelID, newName)
}

func cmdDeleteLabel(args []string) {
	params := parseArgs(args)
	labelID := getParam(params, "label_id", "id")
	if labelID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail delete-label --label_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: delete-label label_id=%s\n", labelID)
}

// ========== FOLDER COMMANDS ==========

func cmdListFolders(args []string) {
	fmt.Println("Ejecutando: list-folders")
}

func cmdCreateFolder(args []string) {
	params := parseArgs(args)
	name := params["name"]
	if name == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail create-folder --name 'NOMBRE'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: create-folder name='%s'\n", name)
}

func cmdMoveToFolder(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	folderID := getParam(params, "folder_id", "folder")
	if emailID == "" || folderID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail move-to-folder --email_id ID --folder_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: move-to-folder email_id=%s folder_id=%s\n", emailID, folderID)
}

// ========== FILTER COMMANDS ==========

func cmdListFilters(args []string) {
	fmt.Println("Ejecutando: list-filters")
}

func cmdGetFilter(args []string) {
	params := parseArgs(args)
	filterID := getParam(params, "filter_id", "id")
	if filterID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail get-filter --filter_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: get-filter filter_id=%s\n", filterID)
}

func cmdCreateFilter(args []string) {
	params := parseArgs(args)
	fmt.Println("Ejecutando: create-filter")
	data, _ := json.MarshalIndent(params, "", "  ")
	fmt.Println(string(data))
}

func cmdDeleteFilter(args []string) {
	params := parseArgs(args)
	filterID := getParam(params, "filter_id", "id")
	if filterID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail delete-filter --filter_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: delete-filter filter_id=%s\n", filterID)
}

// ========== ARCHIVE COMMANDS ==========

func cmdArchiveEmail(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	if emailID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail archive-email --email_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: archive-email email_id=%s\n", emailID)
}

func cmdBatchArchive(args []string) {
	params := parseArgs(args)
	query := getParam(params, "query", "q")
	maxEmails := getParam(params, "max_emails", "max")
	if maxEmails == "" {
		maxEmails = "100"
	}
	if query == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail batch-archive --query 'from:remitente@email.com'\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: batch-archive query='%s' max_emails=%s\n", query, maxEmails)
}

func cmdListArchived(args []string) {
	params := parseArgs(args)
	maxResults := getParam(params, "max_results", "max")
	if maxResults == "" {
		maxResults = "50"
	}
	fmt.Printf("Ejecutando: list-archived max_results=%s\n", maxResults)
}

func cmdRestoreToInbox(args []string) {
	params := parseArgs(args)
	emailID := getParam(params, "email_id", "id")
	if emailID == "" {
		fmt.Fprintf(os.Stderr, "Uso: gmail restore-to-inbox --email_id ID\n")
		os.Exit(1)
	}
	fmt.Printf("Ejecutando: restore-to-inbox email_id=%s\n", emailID)
}
