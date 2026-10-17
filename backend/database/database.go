package database

import (
	"log"
	"time"

	firebird "github.com/flylink888/gorm-firebird"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectDB regresa un puntero: toda la app comparte el MISMO pool de conexiones
func ConnectDB(dns string) *gorm.DB {
	var db *gorm.DB
	var err error

	// Firebird tarda unos segundos en arrancar dentro del contenedor: reintenta 60 s
	for i := 1; i <= 30; i++ {
		db, err = gorm.Open(firebird.Open(dns), &gorm.Config{
			Logger:         logger.Default.LogMode(logger.Error),
			NamingStrategy: firebird.NamingStrategy{},
		})
		if err == nil {
			break
		}
		log.Printf("Esperando a Firebird (%d/30): %v", i, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("Error de conexion: ", err.Error())
	}

	// Pool conn config
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Error al obtener la conexión SQL: ", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("¡Conectado a Firebird 5 con GORM!")
	return db
}
