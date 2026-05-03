// Binary `vault`: CLI para que cualquier framework consulte/escriba
// secretos en el vault compartido sin duplicar el código de cifrado.
//
// Uso:
//
//	vault has    --conv <id> --key <capability>
//	vault get    --conv <id> --key <capability>          # imprime JSON plaintext en stdout
//	vault set    --conv <id> --key <capability> --value <json>
//	vault set    --conv <id> --key <capability> --stdin  # lee value de stdin
//	vault list   --conv <id>                             # JSON array de capabilities
//	vault delete --conv <id> --key <capability>
//	vault genkey                                         # imprime REMORA_VAULT_KEY=<hex>
//
// Exit codes:
//
//	0  ok
//	2  capability no encontrada (vault has = false, vault get sin valor)
//	1  error operacional (clave faltante, disco, decrypt, etc.)
//
// El binario imprime errores legibles en stderr y resultados en stdout para
// que callers (otros frameworks) puedan parsear con seguridad.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"channel/vault"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "has":
		cmdHas(os.Args[2:])
	case "get":
		cmdGet(os.Args[2:])
	case "set":
		cmdSet(os.Args[2:])
	case "list":
		cmdList(os.Args[2:])
	case "delete", "del", "rm":
		cmdDelete(os.Args[2:])
	case "genkey":
		cmdGenKey()
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "vault: comando desconocido: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`vault — almacén de secretos compartido entre frameworks

Comandos:
  has    --conv <id> --key <cap>           true/false (exit 0 si existe, 2 si no)
  get    --conv <id> --key <cap>           imprime el JSON plaintext
  set    --conv <id> --key <cap> --value J o --stdin
  list   --conv <id>                       JSON array de capabilities
  delete --conv <id> --key <cap>
  genkey                                   genera REMORA_VAULT_KEY (hex)

Env:
  REMORA_VAULT_KEY    clave maestra (32 bytes hex/base64). Requerida.
  REMORA_VAULT_DIR    directorio base (default: channel/vault_data)`)
}

func cmdHas(args []string) {
	fs := flag.NewFlagSet("has", flag.ExitOnError)
	conv := fs.String("conv", "", "conversation id")
	key := fs.String("key", "", "capability key")
	_ = fs.Parse(args)
	requireConvKey(*conv, *key)
	if vault.Has("", *conv, *key) {
		fmt.Println("true")
		os.Exit(0)
	}
	fmt.Println("false")
	os.Exit(2)
}

func cmdGet(args []string) {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	conv := fs.String("conv", "", "conversation id")
	key := fs.String("key", "", "capability key")
	_ = fs.Parse(args)
	requireConvKey(*conv, *key)
	val, err := vault.Get("", *conv, *key)
	if err != nil {
		if err == vault.ErrNotFound {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "vault get: %v\n", err)
		os.Exit(1)
	}
	// Stdout sin newline extra: el valor es JSON, el caller decide.
	_, _ = os.Stdout.Write(val)
}

func cmdSet(args []string) {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	conv := fs.String("conv", "", "conversation id")
	key := fs.String("key", "", "capability key")
	value := fs.String("value", "", "valor JSON (o usar --stdin)")
	stdin := fs.Bool("stdin", false, "leer valor de stdin")
	_ = fs.Parse(args)
	requireConvKey(*conv, *key)

	var data []byte
	switch {
	case *stdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vault set: read stdin: %v\n", err)
			os.Exit(1)
		}
		data = b
	case *value != "":
		data = []byte(*value)
	default:
		fmt.Fprintln(os.Stderr, "vault set: --value o --stdin requerido")
		os.Exit(2)
	}
	if err := vault.Set("", *conv, *key, data); err != nil {
		fmt.Fprintf(os.Stderr, "vault set: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("ok")
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	conv := fs.String("conv", "", "conversation id")
	_ = fs.Parse(args)
	if strings.TrimSpace(*conv) == "" {
		fmt.Fprintln(os.Stderr, "vault list: --conv requerido")
		os.Exit(2)
	}
	caps, err := vault.List("", *conv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vault list: %v\n", err)
		os.Exit(1)
	}
	// Formato JSON manual para no agregar deps.
	fmt.Print("[")
	for i, c := range caps {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%q", c)
	}
	fmt.Println("]")
}

func cmdDelete(args []string) {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	conv := fs.String("conv", "", "conversation id")
	key := fs.String("key", "", "capability key")
	_ = fs.Parse(args)
	requireConvKey(*conv, *key)
	if err := vault.Delete("", *conv, *key); err != nil {
		fmt.Fprintf(os.Stderr, "vault delete: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("ok")
}

func cmdGenKey() {
	k, err := vault.GenerateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vault genkey: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("REMORA_VAULT_KEY=%s\n", k)
}

func requireConvKey(conv, key string) {
	if strings.TrimSpace(conv) == "" || strings.TrimSpace(key) == "" {
		fmt.Fprintln(os.Stderr, "vault: --conv y --key son requeridos")
		os.Exit(2)
	}
}
