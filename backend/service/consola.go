package service

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// Consola: cliente telnet para la consola de un nodo (router IOS o VPCS).
// Las consolas de GNS3 no piden contraseña; la API llega a ellas por la red
// interna del servidor (el firewall solo filtra lo que viene de la LAN).
// Si el alumno puso contraseña en la consola o en enable, el revisor prueba
// las que el profesor escribió en los criterios de ese router.
type Consola struct {
	conn        net.Conn
	priv        bool // está en modo privilegiado (#)
	privInicial bool // ya estaba en # al abrirla (al cerrar no se hace "disable")
}

var (
	// ErrSinEnable: el router pide contraseña de enable y no es ninguna de la práctica
	ErrSinEnable = errors.New("no se pudo entrar a enable con las contraseñas de la práctica")
	// ErrConsolaConClave: la consola pide contraseña (line con 0) y no es ninguna de la práctica
	ErrConsolaConClave = errors.New("la consola pide contraseña y no es ninguna de las de la práctica")
)

// prompt al final de la salida: "R1>", "R1#", "R1(config-if)#", "Host01>"
var rePrompt = regexp.MustCompile(`([A-Za-z0-9._-]+)(\([^)\r\n]*\))?([>#]) ?$`)

var reANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07]*\x07`)

func AbrirConsola(host string, puerto int) (*Consola, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, puerto), 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la consola: %w", err)
	}
	return &Consola{conn: conn}, nil
}

// Close: si el revisor entró a enable, regresa a modo usuario (para no dejarle
// el # abierto a quien abra la consola después) y cierra la conexión
func (c *Consola) Close() {
	if c.priv && !c.privInicial {
		_ = c.enviar("disable\r")
		time.Sleep(200 * time.Millisecond)
	}
	c.conn.Close()
}

func (c *Consola) enviar(s string) error {
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.conn.Write([]byte(s))
	return err
}

// leer junta lo que llega hasta que "listo" diga que ya, o hasta el tiempo límite.
// Contesta la negociación telnet (IAC) para que el servidor no se quede esperando.
func (c *Consola) leer(listo func(string) bool, limite time.Duration) (string, error) {
	var out strings.Builder
	buf := make([]byte, 4096)
	fin := time.Now().Add(limite)
	for time.Now().Before(fin) {
		_ = c.conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := c.conn.Read(buf)
		for i := 0; i < n; i++ {
			b := buf[i]
			if b == 255 && i+2 < n && buf[i+1] >= 251 && buf[i+1] <= 254 { // IAC WILL/WONT/DO/DONT opción
				resp := byte(254) // DONT
				if buf[i+1] == 253 || buf[i+1] == 254 {
					resp = 252 // WONT
				}
				if buf[i+1] == 251 && (buf[i+2] == 1 || buf[i+2] == 3) {
					resp = 253 // DO echo / suppress-go-ahead
				}
				_, _ = c.conn.Write([]byte{255, resp, buf[i+2]})
				i += 2
				continue
			}
			if b != 0 {
				out.WriteByte(b)
			}
		}
		texto := reANSI.ReplaceAllString(out.String(), "")
		if listo(texto) {
			return texto, nil
		}
		if err != nil {
			var ne net.Error
			if !(errors.As(err, &ne) && ne.Timeout()) {
				return texto, err
			}
		}
	}
	return reANSI.ReplaceAllString(out.String(), ""), errors.New("la consola no respondió a tiempo")
}

func terminaEnPrompt(s string) bool {
	return rePrompt.MatchString(strings.TrimRight(s, " \r\n")) && !strings.HasSuffix(strings.TrimRight(s, "\r\n "), "--More--")
}

// PrepararIOS deja al router en su prompt (">" o "#") y sin paginación.
// Espera a que arranque (hasta "espera") y atiende el diálogo de configuración
// inicial, "Press RETURN", el modo configuración que haya dejado el alumno y la
// contraseña de la consola (prueba "claves" en orden). NO entra a enable: los
// show que usa el revisor funcionan en modo usuario.
func (c *Consola) PrepararIOS(claves []string, espera time.Duration) error {
	fin := time.Now().Add(espera)
	pendientes := claves
	enter := true
	for time.Now().Before(fin) {
		if enter {
			_ = c.enviar("\r")
		}
		enter = true
		out, _ := c.leer(func(s string) bool {
			return terminaEnPrompt(s) || strings.Contains(s, "[yes/no]") || strings.Contains(ultimaLinea(s), "Password:")
		}, 5*time.Second)
		final := ultimaLinea(out)
		switch {
		case strings.Contains(out, "initial configuration dialog") && strings.Contains(final, "[yes/no]"):
			_ = c.enviar("no\r")
			time.Sleep(2 * time.Second)
		case strings.Contains(final, "[yes/no]"):
			_ = c.enviar("yes\r")
		case strings.Contains(final, "Password:"):
			// contraseña de la consola: se prueba la siguiente (IOS da 3 intentos)
			if len(pendientes) == 0 {
				return ErrConsolaConClave
			}
			_ = c.enviar(pendientes[0] + "\r")
			pendientes = pendientes[1:]
			enter = false
			continue
		case terminaEnPrompt(out):
			m := rePrompt.FindStringSubmatch(strings.TrimRight(out, " \r\n"))
			switch {
			case m == nil: // todavía no hay prompt: otra vuelta
			case m[2] != "": // (config...)#
				_ = c.enviar("end\r")
			default: // R1> o R1#
				c.priv = m[3] == "#"
				c.privInicial = c.priv
				_, err := c.Comando("terminal length 0", 10*time.Second) // también en modo usuario
				return err
			}
		}
		time.Sleep(time.Second)
	}
	return errors.New("el router no terminó de arrancar a tiempo")
}

// Enable entra a modo privilegiado probando "claves" en orden.
// "pidio" dice si el router pidió contraseña (tiene enable secret/password).
func (c *Consola) Enable(claves []string) (pidio bool, err error) {
	if c.priv {
		return false, nil
	}
	pendientes := claves
	espera := func(s string) bool { return terminaEnPrompt(s) || strings.Contains(ultimaLinea(s), "Password:") }
	_ = c.enviar("enable\r")
	for range 12 {
		out, _ := c.leer(espera, 5*time.Second)
		switch {
		case strings.Contains(ultimaLinea(out), "Password:"):
			pidio = true
			if len(pendientes) > 0 {
				_ = c.enviar(pendientes[0] + "\r")
				pendientes = pendientes[1:]
			} else {
				_ = c.enviar("\r") // se agotan los intentos y regresa a ">"
			}
		case terminaEnPrompt(out):
			if strings.HasSuffix(strings.TrimRight(out, " \r\n"), "#") {
				c.priv = true
				return pidio, nil
			}
			if len(pendientes) == 0 {
				return pidio, ErrSinEnable
			}
			_ = c.enviar("enable\r") // quedan contraseñas: otra ronda de 3 intentos
		default:
			_ = c.enviar("\r")
		}
	}
	return pidio, ErrSinEnable
}

// Disable regresa a modo usuario (para probar enable desde cero)
func (c *Consola) Disable() {
	if c.priv {
		_, _ = c.Comando("disable", 10*time.Second)
		c.priv = false
	}
}

// Comando manda una línea y regresa lo que respondió (sin el eco ni el prompt final)
func (c *Consola) Comando(cmd string, limite time.Duration) (string, error) {
	if err := c.enviar(cmd + "\r"); err != nil {
		return "", err
	}
	out, err := c.leer(func(s string) bool {
		// el prompt debe llegar DESPUÉS del eco del comando
		i := strings.Index(s, cmd)
		return i >= 0 && terminaEnPrompt(s[i+len(cmd):])
	}, limite)
	if i := strings.Index(out, cmd); i >= 0 {
		out = out[i+len(cmd):]
	}
	lineas := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	if len(lineas) > 0 && rePrompt.MatchString(strings.TrimSpace(lineas[len(lineas)-1])) {
		lineas = lineas[:len(lineas)-1]
	}
	return strings.Trim(strings.Join(lineas, "\n"), "\n"), err
}

// PrepararVPCS deja la consola del VPCS lista para recibir comandos
func (c *Consola) PrepararVPCS() error {
	_ = c.enviar("\r")
	_, err := c.leer(terminaEnPrompt, 10*time.Second)
	return err
}

func ultimaLinea(s string) string {
	s = strings.TrimRight(s, " \r\n")
	if i := strings.LastIndexAny(s, "\r\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}
