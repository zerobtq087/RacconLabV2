package handler

import (
	"crypto/subtle"
	"strings"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/service"

	"github.com/gofiber/fiber/v3"
)

// InternoHandler: lo usa el agente del servidor (raccoon-accesos), no el navegador
type InternoHandler struct {
	accesos *service.Accesos
	env     *config.Env
}

func NewInternoHandler(a *service.Accesos, env *config.Env) *InternoHandler {
	return &InternoHandler{accesos: a, env: env}
}

// Accesos: una línea por IP y puerto, ya en el formato del set de nftables:
//
//	192.168.1.50 . 30000     ← GNS3 del lab
//	192.168.1.50 . 30001     ← consola de un nodo que existe
//
// El agente la pide cada segundo con la clave ACCESOS_TOKEN y actualiza el firewall.
func (h *InternoHandler) Accesos(c fiber.Ctx) error {
	clave := c.Get("X-Raccoon-Token")
	if h.env.AccesosToken == "" || subtle.ConstantTimeCompare([]byte(clave), []byte(h.env.AccesosToken)) != 1 {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}
	return c.SendString(strings.Join(h.accesos.Lista(), "\n"))
}
