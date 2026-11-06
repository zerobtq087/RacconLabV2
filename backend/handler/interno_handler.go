package handler

import (
	"crypto/subtle"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/repository"

	"github.com/gofiber/fiber/v3"
)

// InternoHandler: lo usa el agente del servidor (raccoon-accesos), no el navegador
type InternoHandler struct {
	accesos *repository.AccesoRepository
	env     *config.Env
}

func NewInternoHandler(a *repository.AccesoRepository, env *config.Env) *InternoHandler {
	return &InternoHandler{accesos: a, env: env}
}

// Accesos: una línea por IP y lab, ya en el formato del set de nftables:
//
//	192.168.1.50 . 30000-30039
//
// El agente la pide cada 2 segundos con la clave ACCESOS_TOKEN y actualiza el firewall.
func (h *InternoHandler) Accesos(c fiber.Ctx) error {
	clave := c.Get("X-Raccoon-Token")
	if h.env.AccesosToken == "" || subtle.ConstantTimeCompare([]byte(clave), []byte(h.env.AccesosToken)) != 1 {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}

	// Mismos tiempos que la sesión en Redis (cmd/main.go): 8 h sin uso o 12 h en total
	var lineas []string
	for _, a := range h.accesos.Accesos(8*time.Hour, 12*time.Hour) {
		ip, err := netip.ParseAddr(a.IP)
		if err != nil || !ip.Is4() {
			continue // el firewall de los labs es solo IPv4
		}
		lineas = append(lineas, fmt.Sprintf("%s . %d-%d", ip, a.Puerto, a.Puerto+h.env.LabPuertos-1))
	}
	return c.SendString(strings.Join(lineas, "\n"))
}
