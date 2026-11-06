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
type Consola struct {
	conn net.Conn
}

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

func (c *Consola) Close() { c.conn.Close() }

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

// PrepararIOS deja al router en modo privilegiado (#) y sin paginación.
// Espera a que arranque (hasta "espera") y atiende el diálogo de configuración
// inicial, "Press RETURN" y el modo configuración que haya dejado el alumno.
func (c *Consola) PrepararIOS(enablePassword string, espera time.Duration) error {
	fin := time.Now().Add(espera)
	for time.Now().Before(fin) {
		_ = c.enviar("\r")
		out, _ := c.leer(func(s string) bool {
			return terminaEnPrompt(s) || strings.Contains(s, "[yes/no]") || strings.Contains(s, "Password:")
		}, 5*time.Second)
		final := ultimaLinea(out)
		switch {
		case strings.Contains(out, "initial configuration dialog") && strings.Contains(final, "[yes/no]"):
			_ = c.enviar("no\r")
			time.Sleep(2 * time.Second)
		case strings.Contains(final, "[yes/no]"):
			_ = c.enviar("yes\r")
		case strings.Contains(final, "Password:"):
			if enablePassword == "" {
				return errors.New("el router pide contraseña de enable y la práctica no la tiene registrada")
			}
			_ = c.enviar(enablePassword + "\r")
			out, _ = c.leer(terminaEnPrompt, 5*time.Second)
			if strings.HasSuffix(strings.TrimSpace(out), ">") || strings.Contains(ultimaLinea(out), "Password:") {
				return errors.New("la contraseña de enable de la práctica no es la del router")
			}
			fallthrough
		case terminaEnPrompt(out):
			m := rePrompt.FindStringSubmatch(strings.TrimRight(out, " \r\n"))
			switch {
			case m == nil: // todavía no hay prompt: otra vuelta
			case m[2] != "": // (config...)#
				_ = c.enviar("end\r")
			case m[3] == ">":
				_ = c.enviar("enable\r")
				out, _ = c.leer(func(s string) bool { return terminaEnPrompt(s) || strings.Contains(s, "Password:") }, 5*time.Second)
				if strings.Contains(ultimaLinea(out), "Password:") {
					if enablePassword == "" {
						return errors.New("el router pide contraseña de enable y la práctica no la tiene registrada")
					}
					_ = c.enviar(enablePassword + "\r")
					out, _ = c.leer(terminaEnPrompt, 5*time.Second)
				}
				if strings.HasSuffix(strings.TrimSpace(out), "#") {
					_, err := c.Comando("terminal length 0", 10*time.Second)
					return err
				}
			default: // ya está en #
				_, err := c.Comando("terminal length 0", 10*time.Second)
				return err
			}
		}
		time.Sleep(time.Second)
	}
	return errors.New("el router no terminó de arrancar a tiempo")
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
