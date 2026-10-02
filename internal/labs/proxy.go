package labs

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Proxy: por cada laboratorio abre un puerto PÚBLICO que pide el token
// y reenvía a GNS3, que solo escucha en 127.0.0.1 del servidor.
// Funciona para la Web UI y sus websockets (consolas en el navegador).
type Proxy struct {
	mu         sync.Mutex
	servidores map[string]*http.Server // id del workspace -> servidor
}

func NuevoProxy() *Proxy {
	return &Proxy{servidores: make(map[string]*http.Server)}
}

const cookieWS = "rl_ws"

// autorizado: token en la contraseña de Basic Auth (cualquier usuario) o en la cookie
func autorizado(r *http.Request, token string) (bool, bool) {
	if _, pass, ok := r.BasicAuth(); ok && subtle.ConstantTimeCompare([]byte(pass), []byte(token)) == 1 {
		return true, true
	}
	if c, err := r.Cookie(cookieWS); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(token)) == 1 {
		return true, false
	}
	return false, false
}

// Abrir empieza a escuchar en publico y reenvía a 127.0.0.1:interno
func (p *Proxy) Abrir(id string, publico, interno int, token string) error {
	p.Cerrar(id)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", publico))
	if err != nil {
		return fmt.Errorf("no se pudo abrir el puerto %d: %w", publico, err)
	}

	destino, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", interno))
	rp := httputil.NewSingleHostReverseProxy(destino)
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "GNS3 aún está arrancando; espera unos segundos y recarga", http.StatusBadGateway)
	}

	manejador := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, porBasic := autorizado(r, token)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Raccoon Lab: usuario = tu matricula, contrasena = token del laboratorio", charset="UTF-8"`)
			http.Error(w, "Token del laboratorio requerido", http.StatusUnauthorized)
			return
		}
		if porBasic {
			// La cookie ayuda a los navegadores que no reenvían Basic Auth en websockets
			http.SetCookie(w, &http.Cookie{Name: cookieWS, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		r.Header.Del("Authorization")
		rp.ServeHTTP(w, r)
	})

	srv := &http.Server{Handler: manejador, ReadHeaderTimeout: 10 * time.Second}
	p.mu.Lock()
	p.servidores[id] = srv
	p.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[PROXY] %s: %v", id, err)
		}
	}()
	log.Printf("[PROXY] %s: puerto público %d -> 127.0.0.1:%d", id, publico, interno)
	return nil
}

// Cerrar deja de escuchar el puerto del laboratorio
func (p *Proxy) Cerrar(id string) {
	p.mu.Lock()
	srv, ok := p.servidores[id]
	delete(p.servidores, id)
	p.mu.Unlock()
	if ok {
		_ = srv.Close()
		log.Printf("[PROXY] %s: cerrado", id)
	}
}
