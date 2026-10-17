package handler

import (
	"slices"
	"strconv"
	"strings"

	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/middleware"
	"raccoon-lab/backend/repository"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	session    *middleware.Session
	repository *repository.AuthRepository
}

func NewAuthHandler(s *middleware.Session, r *repository.AuthRepository) *AuthHandler {
	return &AuthHandler{session: s, repository: r}
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	type credentials struct {
		Matricula string `json:"matricula"`
		Password  string `json:"password"`
		Rol       string `json:"rol"`
	}
	var req credentials

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}
	req.Matricula = strings.ToLower(strings.TrimSpace(req.Matricula))

	if req.Matricula == "" || req.Password == "" || req.Rol == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Escribe rol, matrícula y contraseña")
	}

	// ¿Está bloqueada por intentos fallidos?
	if seg := h.repository.LockedSeconds(req.Matricula); seg > 0 {
		c.Set("Retry-After", strconv.Itoa(seg))
		return c.Status(fiber.StatusTooManyRequests).SendString("Cuenta bloqueada por intentos fallidos")
	}

	acc := h.repository.UserByMatricula(req.Matricula)

	// Mismo mensaje si no existe o si la contraseña está mal (no damos pistas)
	if acc == nil || bcrypt.CompareHashAndPassword([]byte(acc.Password), []byte(req.Password)) != nil {
		restantes, bloqueo := h.repository.LoginFailed(req.Matricula)
		if bloqueo > 0 {
			c.Set("Retry-After", strconv.Itoa(bloqueo))
			return c.Status(fiber.StatusTooManyRequests).SendString("Cuenta bloqueada por intentos fallidos")
		}
		return c.Status(fiber.StatusUnauthorized).
			SendString("Matrícula o contraseña incorrecta. Intentos restantes: " + strconv.Itoa(restantes))
	}

	if !acc.Activo {
		return c.Status(fiber.StatusForbidden).SendString("Tu cuenta está desactivada")
	}

	// Un admin puede entrar como admin, maestro o alumno; un alumno solo como alumno
	if !slices.Contains(acc.Roles, req.Rol) {
		return c.Status(fiber.StatusForbidden).SendString("No tienes ese rol")
	}

	h.repository.LoginOK(req.Matricula)
	_ = h.repository.RegisterAccess(acc.ID)

	if err := h.session.Create(c, acc.ID, acc.Matricula, acc.Nombre, req.Rol); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error de sesión")
	}

	return c.SendString("success")
}

func (h *AuthHandler) Logout(c fiber.Ctx) error {
	if err := h.session.Destroy(c); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo cerrar la sesión")
	}
	return c.SendString("success")
}

func (h *AuthHandler) UpdatePasswd(c fiber.Ctx) error {
	var req map[string]string
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Datos inválidos")
	}

	userID, ok := c.Locals("userID").(string) // get userID from session
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Datos de sesión no válidos")
	}

	acc := h.repository.UserByID(userID)
	if acc == nil {
		return c.Status(fiber.StatusNotFound).SendString("El usuario ya no existe")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(acc.Password), []byte(req["currentPasswd"])); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("La contraseña actual es incorrecta")
	}

	if len(req["newPasswd"]) < 8 {
		return c.Status(fiber.StatusBadRequest).SendString("La nueva contraseña debe tener al menos 8 caracteres")
	}

	if req["newPasswd"] != req["confirmPasswd"] {
		return c.Status(fiber.StatusBadRequest).SendString("La nueva y la contraseña de confirmación no coinciden")
	}

	hashPasswd, _ := bcrypt.GenerateFromPassword([]byte(req["newPasswd"]), bcrypt.DefaultCost)
	if err := h.repository.UpdatePasswd(userID, string(hashPasswd)); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("No se pudo cambiar la contraseña")
	}

	// Cierra la sesión en los demás equipos, deja abierta la actual
	sessionID, _ := c.Locals("sessionID").(string)
	_ = h.session.CloseAll(c.Context(), userID, sessionID)

	return c.SendString("success")
}

func (h *AuthHandler) CurrentUserAcc(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(string) // get userID from session
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Datos de sesión no válidos")
	}

	acc := h.repository.UserByID(userID)
	if acc == nil {
		return c.Status(fiber.StatusUnauthorized).SendString("El usuario ya no existe")
	}

	rol, _ := c.Locals("rol").(string)

	return c.JSON(domain.UserAccount{
		ID:        acc.ID,
		Matricula: acc.Matricula,
		Nombre:    acc.Nombre,
		Rol:       rol,
		Roles:     acc.Roles,
		Grupo:     acc.Grupo,
	})
}
