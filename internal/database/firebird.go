// Package database: conexión a Firebird, esquema y datos iniciales.
package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"strings"
	"time"

	"raccoon_lab/internal/config"

	_ "github.com/nakagami/firebirdsql"
	"golang.org/x/crypto/bcrypt"
)

//go:embed esquema.sql
var esquemaSQL string

// Conectar abre UNA conexión compartida (*sql.DB) y espera a que Firebird esté listo.
func Conectar(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@%s:%d/%s",
		cfg.DB.Usuario, cfg.DB.Password, cfg.DB.Host, cfg.DB.Puerto, cfg.DB.Ruta)

	db, err := sql.Open("firebirdsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("configurando driver Firebird: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Firebird puede tardar en arrancar: reintenta hasta 60 s
	var ultimoErr error
	for i := 1; i <= 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		ultimoErr = db.PingContext(ctx)
		cancel()
		if ultimoErr == nil {
			log.Printf("[BD] Conectado a Firebird %s:%d", cfg.DB.Host, cfg.DB.Puerto)
			return db, nil
		}
		log.Printf("[BD] Esperando a Firebird (%d/30): %v", i, ultimoErr)
		time.Sleep(2 * time.Second)
	}
	db.Close()
	return nil, fmt.Errorf("no se pudo conectar a Firebird: %w", ultimoErr)
}

// AplicarEsquema crea las tablas solo si la base está vacía.
func AplicarEsquema(db *sql.DB) error {
	var existe int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM RDB$RELATIONS WHERE RDB$RELATION_NAME = 'USUARIOS'`,
	).Scan(&existe)
	if err != nil {
		return fmt.Errorf("revisando esquema: %w", err)
	}
	if existe > 0 {
		log.Println("[BD] Esquema ya existe")
		return nil
	}

	log.Println("[BD] Creando esquema...")
	for i, sentencia := range dividirSentencias(esquemaSQL) {
		if _, err := db.Exec(sentencia); err != nil {
			return fmt.Errorf("sentencia %d del esquema: %w\n%s", i+1, err, sentencia)
		}
	}
	log.Println("[BD] Esquema creado")
	return nil
}

// dividirSentencias quita comentarios "--" y separa por ";"
func dividirSentencias(script string) []string {
	var limpio strings.Builder
	for _, linea := range strings.Split(script, "\n") {
		if strings.HasPrefix(strings.TrimSpace(linea), "--") {
			continue
		}
		limpio.WriteString(linea)
		limpio.WriteByte('\n')
	}

	var sentencias []string
	for _, s := range strings.Split(limpio.String(), ";") {
		if s = strings.TrimSpace(s); s != "" {
			sentencias = append(sentencias, s)
		}
	}
	return sentencias
}

// CrearAdminInicial crea el admin del .env si todavía no existe ningún ADMIN.
func CrearAdminInicial(db *sql.DB, cfg *config.Config) error {
	var admins int
	if err := db.QueryRow(`SELECT COUNT(*) FROM USUARIO_ROLES WHERE ROL = 'ADMIN'`).Scan(&admins); err != nil {
		return fmt.Errorf("contando admins: %w", err)
	}
	if admins > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("cifrando contraseña del admin: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO USUARIOS (MATRICULA, NOMBRE, PASSWORD_HASH, TIPO) VALUES (?, ?, ?, 'admins')`,
		cfg.AdminClave, "Administrador General", string(hash),
	)
	if err != nil {
		return fmt.Errorf("insertando admin: %w", err)
	}
	for _, rol := range []string{"ADMIN", "PROFESOR", "ALUMNO"} {
		if _, err := tx.Exec(`INSERT INTO USUARIO_ROLES (MATRICULA, ROL) VALUES (?, ?)`, cfg.AdminClave, rol); err != nil {
			return fmt.Errorf("asignando rol %s al admin: %w", rol, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("[BD] Admin inicial creado: %s (contraseña en el .env)", cfg.AdminClave)
	return nil
}
